package scorer

import (
	"math"
	"sort"

	"github.com/Saurabh-52/trading-platform/internal/telemetry"
)

// PerformanceMetrics captures latency and throughput statistics for a submission.
type PerformanceMetrics struct {
	SubmissionID  string  `json:"submission_id"`
	TotalRequests int     `json:"total_requests"`
	Successes     int     `json:"successes"`
	Failures      int     `json:"failures"`
	TPS           float64 `json:"tps"`
	MinLatencyMs  float64 `json:"min_latency_ms"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P50LatencyMs  float64 `json:"p50_latency_ms"`
	P90LatencyMs  float64 `json:"p90_latency_ms"`
	P99LatencyMs  float64 `json:"p99_latency_ms"`
	MaxLatencyMs  float64 `json:"max_latency_ms"`
	StdDevMs      float64 `json:"std_dev_ms"`
	WindowSeconds float64 `json:"window_seconds"`
}

// ComputeMetrics analyses a slice of TelemetryEvents and returns aggregate
// latency/throughput metrics.
// Events with Action=="LOG" are ignored.
func ComputeMetrics(submissionID string, events []telemetry.TelemetryEvent) PerformanceMetrics {
	m := PerformanceMetrics{SubmissionID: submissionID}

	var latencies []float64
	var minTS, maxTS int64

	for _, e := range events {
		if e.SubmissionID != submissionID {
			continue
		}
		if e.Action == "LOG" {
			continue
		}
		m.TotalRequests++

		ts := e.Timestamp.UnixNano()
		if minTS == 0 || ts < minTS {
			minTS = ts
		}
		if ts > maxTS {
			maxTS = ts
		}

		if e.StatusCode >= 200 && e.StatusCode < 300 {
			m.Successes++
			latencies = append(latencies, e.LatencyMs)
		} else {
			m.Failures++
		}
	}

	// Always compute window and TPS, even if 100% failures
	if maxTS > minTS {
		m.WindowSeconds = float64(maxTS-minTS) / 1e9
		m.TPS = float64(m.Successes) / m.WindowSeconds
	} else if m.TotalRequests > 0 {
		// All events at the same timestamp — treat as 1s window.
		m.WindowSeconds = 1
		m.TPS = float64(m.Successes)
	}

	if len(latencies) == 0 {
		return m
	}

	sort.Float64s(latencies)

	m.MinLatencyMs = latencies[0]
	m.MaxLatencyMs = latencies[len(latencies)-1]
	m.P50LatencyMs = percentile(latencies, 0.50)
	m.P90LatencyMs = percentile(latencies, 0.90)
	m.P99LatencyMs = percentile(latencies, 0.99)

	var sum float64
	for _, l := range latencies {
		sum += l
	}
	m.AvgLatencyMs = sum / float64(len(latencies))

	var variance float64
	for _, l := range latencies {
		d := l - m.AvgLatencyMs
		variance += d * d
	}
	m.StdDevMs = math.Sqrt(variance / float64(len(latencies)))

	return m
}

// percentile returns the p-th percentile (0..1) of a sorted slice.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}
