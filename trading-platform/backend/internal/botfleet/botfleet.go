package botfleet

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Protocol string

const (
	ProtocolHTTP Protocol = "http"
	ProtocolTCP  Protocol = "tcp"
)

type Strategy string

const (
	StrategyBBOHeavy   Strategy = "bbo_heavy"
	StrategyFlashCrash Strategy = "flash_crash"
	StrategyHighCancel Strategy = "high_cancel"
	StrategyWideSpread Strategy = "wide_spread"
)

type Config struct {
	Target      string
	Protocol    Protocol
	Strategy    Strategy
	Bots        int
	Requests    int
	Duration    time.Duration
	Timeout     time.Duration
	Method      string
	Path        string
	ExpectReply bool
	ContentType string
	Seed        int64
}

type Order struct {
	BotID     int       `json:"bot_id"`
	Sequence  int       `json:"sequence"`
	Strategy  string    `json:"strategy"`
	Action    string    `json:"action"`
	Side      string    `json:"side"`
	Price     float64   `json:"price"`
	Quantity  int       `json:"quantity"`
	Spread    float64   `json:"spread"`
	Cancel    bool      `json:"cancel"`
	CreatedAt time.Time `json:"created_at"`
}

type Summary struct {
	Target            string  `json:"target"`
	Protocol          string  `json:"protocol"`
	Strategy          string  `json:"strategy"`
	Bots              int     `json:"bots"`
	Requests          int     `json:"requests"`
	Successes         int     `json:"successes"`
	Failures          int     `json:"failures"`
	Samples           int     `json:"samples"`
	DurationMillis    int64   `json:"duration_ms"`
	RequestsPerSecond float64 `json:"requests_per_second"`
	AverageLatencyMs  float64 `json:"avg_latency_ms"`
	P50LatencyMs      float64 `json:"p50_latency_ms"`
	P90LatencyMs      float64 `json:"p90_latency_ms"`
	P99LatencyMs      float64 `json:"p99_latency_ms"`
}

func NormalizeProtocol(value string) Protocol {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(ProtocolTCP):
		return ProtocolTCP
	default:
		return ProtocolHTTP
	}
}

func NormalizeStrategy(value string) Strategy {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(StrategyFlashCrash):
		return StrategyFlashCrash
	case string(StrategyHighCancel):
		return StrategyHighCancel
	case string(StrategyWideSpread):
		return StrategyWideSpread
	default:
		return StrategyBBOHeavy
	}
}

func Run(ctx context.Context, cfg Config) (Summary, error) {
	if cfg.Target == "" {
		return Summary{}, fmt.Errorf("target is required")
	}
	if cfg.Bots <= 0 {
		cfg.Bots = 32
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.Duration <= 0 && cfg.Requests <= 0 {
		cfg.Duration = 10 * time.Second
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodPost
	}
	if cfg.ContentType == "" {
		cfg.ContentType = "application/json"
	}
	if cfg.Seed == 0 {
		cfg.Seed = time.Now().UnixNano()
	}

	start := time.Now()
	rec := &recorder{}
	var sent int64
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for botID := 0; botID < cfg.Bots; botID++ {
		wg.Add(1)
		go func(botID int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(cfg.Seed + int64(botID)*7919))
			seq := 0

			switch cfg.Protocol {
			case ProtocolTCP:
				runTCPWorker(ctx, cfg, botID, &seq, rng, rec, &sent)
			default:
				runHTTPWorker(ctx, cfg, botID, &seq, rng, rec, &sent)
			}
		}(botID)
	}

	if cfg.Duration > 0 {
		timer := time.NewTimer(cfg.Duration)
		select {
		case <-timer.C:
			cancel()
		case <-ctx.Done():
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	} else {
		for cfg.Requests > 0 && atomic.LoadInt64(&sent) < int64(cfg.Requests) {
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}

	wg.Wait()
	elapsed := time.Since(start)
	return rec.summary(cfg, elapsed), nil
}

func runHTTPWorker(ctx context.Context, cfg Config, botID int, seq *int, rng *rand.Rand, rec *recorder, sent *int64) {
	client := &http.Client{Timeout: cfg.Timeout}
	for {
		if !claimWork(ctx, cfg, sent) {
			return
		}
		*seq++
		order := generateOrder(cfg.Strategy, botID, *seq, rng)
		payload, _ := json.Marshal(order)
		req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.Target, bytes.NewReader(payload))
		if err != nil {
			rec.add(0, false)
			continue
		}
		req.Header.Set("Content-Type", cfg.ContentType)
		req.Header.Set("X-Bot-Id", fmt.Sprintf("%d", botID))
		req.Header.Set("X-Strategy", string(cfg.Strategy))

		started := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			rec.add(time.Since(started), false)
			continue
		}
		_, _ = ioCopyAndClose(resp.Body)
		rec.add(time.Since(started), resp.StatusCode < 400)
	}
}

func runTCPWorker(ctx context.Context, cfg Config, botID int, seq *int, rng *rand.Rand, rec *recorder, sent *int64) {
	dialer := net.Dialer{Timeout: cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.Target)
	if err != nil {
		rec.add(0, false)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	for {
		if !claimWork(ctx, cfg, sent) {
			return
		}
		*seq++
		order := generateOrder(cfg.Strategy, botID, *seq, rng)
		payload, _ := json.Marshal(order)
		started := time.Now()

		if err := conn.SetWriteDeadline(time.Now().Add(cfg.Timeout)); err != nil {
			rec.add(time.Since(started), false)
			continue
		}
		if _, err := writer.Write(append(payload, '\n')); err != nil {
			rec.add(time.Since(started), false)
			continue
		}
		if err := writer.Flush(); err != nil {
			rec.add(time.Since(started), false)
			continue
		}

		success := true
		if cfg.ExpectReply {
			if err := conn.SetReadDeadline(time.Now().Add(cfg.Timeout)); err != nil {
				success = false
			} else if _, err := reader.ReadBytes('\n'); err != nil {
				success = false
			}
		}

		rec.add(time.Since(started), success)
	}
}

func claimWork(ctx context.Context, cfg Config, sent *int64) bool {
	if ctx.Err() != nil {
		return false
	}
	if cfg.Requests <= 0 {
		return true
	}
	current := atomic.AddInt64(sent, 1)
	if current > int64(cfg.Requests) {
		return false
	}
	return true
}

func generateOrder(strategy Strategy, botID int, sequence int, rng *rand.Rand) Order {
	baseMid := 100.0
	order := Order{
		BotID:     botID,
		Sequence:  sequence,
		Strategy:  string(strategy),
		Side:      "BUY",
		Action:    "NEW",
		Price:     baseMid,
		Quantity:  10,
		Spread:    0.5,
		Cancel:    false,
		CreatedAt: time.Now().UTC(),
	}

	switch strategy {
	case StrategyFlashCrash:
		order.Side = map[bool]string{true: "SELL", false: "BUY"}[rng.Intn(100) < 75]
		order.Action = "NEW"
		order.Price = baseMid + rng.NormFloat64()*4 - 12
		order.Quantity = 50 + rng.Intn(200)
		order.Spread = 4 + rng.Float64()*6
	case StrategyHighCancel:
		if rng.Intn(100) < 65 {
			order.Action = "CANCEL"
			order.Cancel = true
			order.Side = map[bool]string{true: "SELL", false: "BUY"}[rng.Intn(2) == 0]
			order.Price = baseMid + rng.Float64()*2 - 1
			order.Quantity = 1
		} else {
			order.Side = map[bool]string{true: "SELL", false: "BUY"}[rng.Intn(2) == 0]
			order.Price = baseMid + rng.Float64()*1.5 - 0.75
			order.Quantity = 5 + rng.Intn(15)
			order.Spread = 0.2
		}
	case StrategyWideSpread:
		order.Side = map[bool]string{true: "SELL", false: "BUY"}[rng.Intn(2) == 0]
		order.Price = baseMid + rng.Float64()*30 - 15
		order.Quantity = 1 + rng.Intn(20)
		order.Spread = 6 + rng.Float64()*10
	default:
		order.Side = map[bool]string{true: "SELL", false: "BUY"}[rng.Intn(2) == 0]
		order.Price = baseMid + rng.Float64()*0.8 - 0.4
		order.Quantity = 25 + rng.Intn(150)
		order.Spread = 0.15 + rng.Float64()*0.35
	}

	return order
}

type recorder struct {
	mu        sync.Mutex
	latencies []time.Duration
	successes int64
	failures  int64
}

func (r *recorder) add(latency time.Duration, success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if success {
		r.successes++
		r.latencies = append(r.latencies, latency)
		return
	}
	r.failures++
}

func (r *recorder) summary(cfg Config, elapsed time.Duration) Summary {
	r.mu.Lock()
	latencies := append([]time.Duration(nil), r.latencies...)
	successes := r.successes
	failures := r.failures
	r.mu.Unlock()

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	metrics := Summary{
		Target:         cfg.Target,
		Protocol:       string(cfg.Protocol),
		Strategy:       string(cfg.Strategy),
		Bots:           cfg.Bots,
		Requests:       int(successes + failures),
		Successes:      int(successes),
		Failures:       int(failures),
		Samples:        len(latencies),
		DurationMillis: elapsed.Milliseconds(),
	}
	if elapsed > 0 {
		metrics.RequestsPerSecond = float64(metrics.Requests) / elapsed.Seconds()
	}
	if len(latencies) == 0 {
		return metrics
	}

	metrics.AverageLatencyMs = averageDuration(latencies).Seconds() * 1000
	metrics.P50LatencyMs = percentileDuration(latencies, 0.50).Seconds() * 1000
	metrics.P90LatencyMs = percentileDuration(latencies, 0.90).Seconds() * 1000
	metrics.P99LatencyMs = percentileDuration(latencies, 0.99).Seconds() * 1000
	return metrics
}

func averageDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	var total time.Duration
	for _, value := range values {
		total += value
	}
	return total / time.Duration(len(values))
}

func percentileDuration(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * percentile)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func ioCopyAndClose(body io.ReadCloser) (int64, error) {
	defer body.Close()
	return io.Copy(io.Discard, body)
}

