package botfleet

import (
	"context"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGenerateOrderStrategies(t *testing.T) {
	strategies := []Strategy{StrategyBBOHeavy, StrategyFlashCrash, StrategyHighCancel, StrategyWideSpread}
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

func TestRunHTTP(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
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
	if requests != 16 {
		t.Fatalf("expected 16 requests, got %d", requests)
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

func randSource(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}
