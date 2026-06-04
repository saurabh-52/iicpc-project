package botfleet

import (
	"context"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/Saurabh-52/trading-platform/internal/telemetry"
)

func TestGenerateOrderStrategies(t *testing.T) {
	strategies := []Strategy{
		StrategyBBOHeavy, StrategyFlashCrash, StrategyHighCancel, StrategyWideSpread,
		StrategyMarketMaker, StrategyIceberg, StrategyMomentumBurst,
	}
	for _, strategy := range strategies {
		order := generateOrder(strategy, 1, 1, randSource(42))
		if order.Strategy != string(strategy) {
			t.Fatalf("expected strategy %s, got %s", strategy, order.Strategy)
		}
		if order.Quantity <= 0 {
			t.Fatalf("expected positive quantity for %s", strategy)
		}
	}
}

func TestIcebergHasTotalQuantity(t *testing.T) {
	order := generateOrder(StrategyIceberg, 1, 1, randSource(42))
	if order.TotalQuantity <= 0 {
		t.Fatal("expected positive total_quantity for iceberg strategy")
	}
	if order.TotalQuantity <= order.Quantity {
		t.Fatalf("iceberg total_quantity (%d) should be much larger than visible quantity (%d)",
			order.TotalQuantity, order.Quantity)
	}
}

func TestMomentumBurstDirectionalBias(t *testing.T) {
	rng := randSource(42)
	buys := 0
	total := 200
	for i := 0; i < total; i++ {
		order := generateOrder(StrategyMomentumBurst, 1, i+1, rng)
		if order.Side == "BUY" {
			buys++
		}
	}
	// 90% should be BUY — allow some tolerance
	ratio := float64(buys) / float64(total)
	if ratio < 0.80 || ratio > 0.98 {
		t.Fatalf("momentum_burst should have ~90%% BUY bias, got %.1f%% (%d/%d)", ratio*100, buys, total)
	}
}

func TestMarketMakerTightSpread(t *testing.T) {
	rng := randSource(42)
	for i := 0; i < 50; i++ {
		order := generateOrder(StrategyMarketMaker, 1, i+1, rng)
		if order.Spread > 0.20 {
			t.Fatalf("market_maker spread %.4f exceeds expected tight range", order.Spread)
		}
	}
}

func TestRunHTTP(t *testing.T) {
	var requests int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requests, 1)
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metrics, err := Run(context.Background(), Config{
		Target:   server.URL,
		Protocol: ProtocolHTTP,
		Strategy: StrategyHighCancel,
		Bots:     4,
		Requests: 16,
		Timeout:  2 * time.Second,
		Method:   http.MethodPost,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if metrics.Successes != 16 {
		t.Fatalf("expected 16 successes, got %d", metrics.Successes)
	}
}

func TestRunHTTPErrorBreakdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusInternalServerError) // 500
	}))
	defer server.Close()

	metrics, err := Run(context.Background(), Config{
		Target:   server.URL,
		Protocol: ProtocolHTTP,
		Strategy: StrategyBBOHeavy,
		Bots:     2,
		Requests: 8,
		Timeout:  2 * time.Second,
		Method:   http.MethodPost,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if metrics.Successes != 0 {
		t.Fatalf("expected 0 successes, got %d", metrics.Successes)
	}
	if metrics.Failures != 8 {
		t.Fatalf("expected 8 failures, got %d", metrics.Failures)
	}
	count, ok := metrics.ErrorBreakdown["http_500"]
	if !ok || count != 8 {
		t.Fatalf("expected error_breakdown to have http_500=8, got %v", metrics.ErrorBreakdown)
	}
}

func TestRunTCP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				buf := make([]byte, 2048)
				for {
					_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
					_, err := conn.Read(buf)
					if err != nil {
						return
					}
					_, _ = conn.Write([]byte("ack\n"))
				}
			}(conn)
		}
	}()

	metrics, err := Run(context.Background(), Config{
		Target:      listener.Addr().String(),
		Protocol:    ProtocolTCP,
		Strategy:    StrategyBBOHeavy,
		Bots:        2,
		Requests:    8,
		Timeout:     2 * time.Second,
		ExpectReply: true,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if metrics.Successes != 8 {
		t.Fatalf("expected 8 successes, got %d", metrics.Successes)
	}
}

func TestRunFIX(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				buf := make([]byte, 4096)
				for {
					_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
					n, err := conn.Read(buf)
					if err != nil {
						return
					}
					msg := string(buf[:n])
					// Validate it looks like a FIX message
					if !strings.Contains(msg, "35=") || !strings.Contains(msg, "|") {
						return
					}
					_, _ = conn.Write([]byte("8=FIX.4.4|35=8|39=0|\n"))
				}
			}(conn)
		}
	}()

	metrics, err := Run(context.Background(), Config{
		Target:      listener.Addr().String(),
		Protocol:    ProtocolFIX,
		Strategy:    StrategyMarketMaker,
		Bots:        2,
		Requests:    8,
		Timeout:     2 * time.Second,
		ExpectReply: true,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if metrics.Successes != 8 {
		t.Fatalf("expected 8 successes, got %d (failures=%d, errors=%v)",
			metrics.Successes, metrics.Failures, metrics.ErrorBreakdown)
	}
}

func TestRunRampUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	start := time.Now()
	metrics, err := Run(context.Background(), Config{
		Target:         server.URL,
		Protocol:       ProtocolHTTP,
		Strategy:       StrategyBBOHeavy,
		Bots:           8,
		Duration:       2 * time.Second,
		Timeout:        2 * time.Second,
		Method:         http.MethodPost,
		RampUpDuration: 1 * time.Second, // Bots staggered over 1s
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if metrics.Successes == 0 {
		t.Fatal("expected some successes")
	}
	// With ramp-up + duration, it should take at least ~1s
	if elapsed < 900*time.Millisecond {
		t.Fatalf("ramp-up should delay execution, but it completed in %v", elapsed)
	}
}

func TestEnhancedMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metrics, err := Run(context.Background(), Config{
		Target:   server.URL,
		Protocol: ProtocolHTTP,
		Strategy: StrategyBBOHeavy,
		Bots:     2,
		Requests: 16,
		Timeout:  2 * time.Second,
		Method:   http.MethodPost,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if metrics.MinLatencyMs < 0 {
		t.Fatal("expected non-negative min_latency_ms")
	}
	if metrics.MaxLatencyMs < metrics.MinLatencyMs {
		t.Fatalf("max (%.4f) should be >= min (%.4f)", metrics.MaxLatencyMs, metrics.MinLatencyMs)
	}
	// StdDev can be 0 if all latencies are identical (unlikely but valid)
	if metrics.StdDevMs < 0 {
		t.Fatal("stddev should be non-negative")
	}
	if metrics.ErrorBreakdown == nil {
		t.Fatal("error_breakdown should never be nil")
	}
}

func TestOrderToFIX(t *testing.T) {
	order := Order{
		BotID:    0,
		Sequence: 1,
		Side:     "BUY",
		Price:    100.50,
		Quantity: 25,
		Cancel:   false,
	}
	fix := orderToFIX(order)
	if !strings.Contains(fix, "35=D") {
		t.Fatalf("expected 35=D (New Order), got: %s", fix)
	}
	if !strings.Contains(fix, "54=1") {
		t.Fatalf("expected 54=1 (Buy side), got: %s", fix)
	}
	if !strings.Contains(fix, "44=100.50") {
		t.Fatalf("expected 44=100.50 (Price), got: %s", fix)
	}

	cancelOrder := Order{
		BotID:    0,
		Sequence: 2,
		Side:     "SELL",
		Cancel:   true,
	}
	fix2 := orderToFIX(cancelOrder)
	if !strings.Contains(fix2, "35=F") {
		t.Fatalf("expected 35=F (Cancel), got: %s", fix2)
	}
	if !strings.Contains(fix2, "54=2") {
		t.Fatalf("expected 54=2 (Sell side), got: %s", fix2)
	}
}

func randSource(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

func TestHTTPWithTelemetry(t *testing.T) {
	// Use manual lifecycle so miniredis stays alive during async goroutine flush
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()
	// MaxRetries=0 prevents retry storms from orphaned fire-and-forget goroutines
	// that would otherwise exhaust macOS ephemeral ports.
	redisClient := redis.NewClient(&redis.Options{
		Addr:       mr.Addr(),
		MaxRetries: 0,
	})
	defer redisClient.Close()

	// Stub trading engine: always 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	const submissionID = "test-sub-telemetry-001"

	// Use Requests mode with a small fixed count to avoid port exhaustion.
	// The Run function's "else" branch waits for sent>=Requests then cancels,
	// which may interrupt the last in-flight request.  Use 1 bot to serialize.
	metrics, err := Run(context.Background(), Config{
		Target:          server.URL,
		Protocol:        ProtocolHTTP,
		Strategy:        StrategyBBOHeavy,
		Bots:            1,
		Requests:        20,
		Timeout:         2 * time.Second,
		Method:          http.MethodPost,
		TelemetryClient: redisClient,
		SubmissionID:    submissionID,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("Successes=%d Failures=%d", metrics.Successes, metrics.Failures)

	// Give the fire-and-forget goroutines time to flush to miniredis
	time.Sleep(300 * time.Millisecond)

	ctx := context.Background()
	events, err := telemetry.ConsumeAllForSubmission(ctx, redisClient, submissionID)
	if err != nil {
		t.Fatalf("ConsumeAllForSubmission: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected >0 telemetry events, got 0 (successes=%d)", metrics.Successes)
	}
	t.Logf("TelemetryEvents=%d", len(events))
	for _, e := range events {
		if e.SubmissionID != submissionID {
			t.Errorf("wrong SubmissionID: %q", e.SubmissionID)
		}
	}
}
