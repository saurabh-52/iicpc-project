package scorer

import (
	"github.com/Saurabh-52/trading-platform/internal/validator"
)

// Scoring thresholds — tuned for competitive trading engine benchmarking.
const (
	// Latency scoring (0–25 points)
	LatencyMaxScore   = 25.0
	LatencyPerfectP99 = 5.0   // ms — full score at or below
	LatencyZeroP99    = 100.0 // ms — zero score at or above

	// Throughput scoring (0–25 points)
	ThroughputMaxScore   = 25.0
	ThroughputPerfectTPS = 5000.0 // full score at or above
	ThroughputZeroTPS    = 100.0  // zero score at or below

	// Correctness scoring (0–50 points)
	CorrectnessMaxScore = 50.0
)

// Score is the final graded result for a submission.
type Score struct {
	SubmissionID     string  `json:"submission_id"`
	TotalScore       float64 `json:"total_score"`
	LatencyScore     float64 `json:"latency_score"`
	ThroughputScore  float64 `json:"throughput_score"`
	CorrectnessScore float64 `json:"correctness_score"`
	Grade            string  `json:"grade"`
}

// ComputeScore combines PerformanceMetrics and ValidationResult into a final Score.
func ComputeScore(metrics PerformanceMetrics, validation validator.ValidationResult) Score {
	s := Score{SubmissionID: metrics.SubmissionID}

	// --- Latency (0–50) ---
	// Linear interpolation between perfect and zero thresholds.
	p99 := metrics.P99LatencyMs
	switch {
	case p99 <= LatencyPerfectP99:
		s.LatencyScore = LatencyMaxScore
	case p99 >= LatencyZeroP99:
		s.LatencyScore = 0
	default:
		ratio := (LatencyZeroP99 - p99) / (LatencyZeroP99 - LatencyPerfectP99)
		s.LatencyScore = LatencyMaxScore * ratio
	}

	// --- Throughput (0–30) ---
	tps := metrics.TPS
	switch {
	case tps >= ThroughputPerfectTPS:
		s.ThroughputScore = ThroughputMaxScore
	case tps <= ThroughputZeroTPS:
		s.ThroughputScore = 0
	default:
		ratio := (tps - ThroughputZeroTPS) / (ThroughputPerfectTPS - ThroughputZeroTPS)
		s.ThroughputScore = ThroughputMaxScore * ratio
	}

	// --- Correctness (0–20) ---
	// Total errors = crosses + mismatches + unparseable responses.
	if validation.OrdersProcessed == 0 {
		s.CorrectnessScore = 0 // Zero orders processed = zero correctness points
	} else {
		totalErrors := validation.TotalErrors()
		if totalErrors == 0 {
			s.CorrectnessScore = CorrectnessMaxScore
		} else {
			// Proportional deduction: lose points for every error relative to total orders.
			errorRate := float64(totalErrors) / float64(validation.OrdersProcessed)
			if errorRate >= 1 {
				s.CorrectnessScore = 0
			} else {
				s.CorrectnessScore = CorrectnessMaxScore * (1 - errorRate)
			}
		}
	}

	s.TotalScore = s.LatencyScore + s.ThroughputScore + s.CorrectnessScore
	s.Grade = AssignGrade(s.TotalScore)
	return s
}

// AssignGrade maps a total score to a letter grade.
func AssignGrade(total float64) string {
	switch {
	case total >= 90:
		return "S"
	case total >= 75:
		return "A"
	case total >= 60:
		return "B"
	case total >= 45:
		return "C"
	default:
		return "F"
	}
}
