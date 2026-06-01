package scorer_test

import (
	"testing"
	"time"

	"github.com/Saurabh-52/trading-platform/internal/telemetry"
	"github.com/Saurabh-52/trading-platform/internal/scorer"
)

func TestComputeMetrics(t *testing.T) {
	const subID = "metrics-test"
	base := time.Now().UTC()

	// Build 100 mock events with latencies ranging from 1–10ms.
	var events []telemetry.TelemetryEvent
	for i := 0; i < 100; i++ {
		events = append(events, telemetry.TelemetryEvent{
			SubmissionID: subID,
			BotID:        i % 8,
			Sequence:     i,
			Action:       "NEW",
			Side:         "BUY",
			Price:        100.0,
			Quantity:     10,
			StatusCode:   200,
			LatencyMs:    1.0 + float64(i)*9.0/99.0, // 1.0 → 10.0
			Timestamp:    base.Add(time.Duration(i) * time.Millisecond),
		})
	}

	m := scorer.ComputeMetrics(subID, events)

	if m.TotalRequests != 100 {
		t.Errorf("TotalRequests: got %d, want 100", m.TotalRequests)
	}
	if m.Successes != 100 {
		t.Errorf("Successes: got %d, want 100", m.Successes)
	}
	if m.Failures != 0 {
		t.Errorf("Failures: got %d, want 0", m.Failures)
	}
	if m.MinLatencyMs >= m.P50LatencyMs {
		t.Errorf("Min (%.2f) should be < P50 (%.2f)", m.MinLatencyMs, m.P50LatencyMs)
	}
	if m.P50LatencyMs >= m.P90LatencyMs {
		t.Errorf("P50 (%.2f) should be < P90 (%.2f)", m.P50LatencyMs, m.P90LatencyMs)
	}
	if m.P90LatencyMs >= m.P99LatencyMs {
		t.Errorf("P90 (%.2f) should be < P99 (%.2f)", m.P90LatencyMs, m.P99LatencyMs)
	}
	if m.P99LatencyMs > m.MaxLatencyMs {
		t.Errorf("P99 (%.2f) should be <= Max (%.2f)", m.P99LatencyMs, m.MaxLatencyMs)
	}
	if m.TPS <= 0 {
		t.Errorf("TPS should be positive, got %.2f", m.TPS)
	}
	if m.StdDevMs <= 0 {
		t.Errorf("StdDev should be positive, got %.6f", m.StdDevMs)
	}
	t.Logf("Metrics: Min=%.2f P50=%.2f P90=%.2f P99=%.2f Max=%.2f TPS=%.0f σ=%.2f",
		m.MinLatencyMs, m.P50LatencyMs, m.P90LatencyMs, m.P99LatencyMs, m.MaxLatencyMs, m.TPS, m.StdDevMs)
}

func TestComputeMetricsFiltersLogEvents(t *testing.T) {
	events := []telemetry.TelemetryEvent{
		{SubmissionID: "s1", Action: "NEW", StatusCode: 200, LatencyMs: 1, Timestamp: time.Now().UTC()},
		{SubmissionID: "s1", Action: "LOG", EngineOutput: "log line"},
		{SubmissionID: "s1", Action: "NEW", StatusCode: 200, LatencyMs: 2, Timestamp: time.Now().UTC()},
	}
	m := scorer.ComputeMetrics("s1", events)
	if m.TotalRequests != 2 {
		t.Errorf("expected 2 requests (LOG skipped), got %d", m.TotalRequests)
	}
}

func TestComputeMetricsEmpty(t *testing.T) {
	m := scorer.ComputeMetrics("empty", nil)
	if m.TotalRequests != 0 || m.TPS != 0 {
		t.Errorf("empty events should give zeroes: %+v", m)
	}
}
