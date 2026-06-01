package scorer_test

import (
	"testing"

	"github.com/Saurabh-52/trading-platform/internal/scorer"
	"github.com/Saurabh-52/trading-platform/internal/validator"
)

func TestPerfectSubmission(t *testing.T) {
	metrics := scorer.PerformanceMetrics{
		SubmissionID: "perfect",
		TotalRequests: 10000,
		Successes:     10000,
		TPS:           10000,
		P99LatencyMs:  2.0,
	}
	val := validator.ValidationResult{
		SubmissionID:    "perfect",
		Valid:           true,
		OrdersProcessed: 10000,
		CrossEvents:    0,
	}

	s := scorer.ComputeScore(metrics, val)

	if s.Grade != "S" {
		t.Errorf("expected Grade S, got %q (total=%.2f)", s.Grade, s.TotalScore)
	}
	if s.TotalScore < 90 {
		t.Errorf("expected TotalScore >= 90, got %.2f", s.TotalScore)
	}
	if s.LatencyScore != scorer.LatencyMaxScore {
		t.Errorf("expected full LatencyScore=%.0f, got %.2f", scorer.LatencyMaxScore, s.LatencyScore)
	}
	if s.ThroughputScore != scorer.ThroughputMaxScore {
		t.Errorf("expected full ThroughputScore=%.0f, got %.2f", scorer.ThroughputMaxScore, s.ThroughputScore)
	}
	if s.CorrectnessScore != scorer.CorrectnessMaxScore {
		t.Errorf("expected full CorrectnessScore=%.0f, got %.2f", scorer.CorrectnessMaxScore, s.CorrectnessScore)
	}
	t.Logf("Score: Total=%.1f  Latency=%.1f  Throughput=%.1f  Correctness=%.1f  Grade=%s",
		s.TotalScore, s.LatencyScore, s.ThroughputScore, s.CorrectnessScore, s.Grade)
}

func TestFailingSubmission(t *testing.T) {
	metrics := scorer.PerformanceMetrics{
		SubmissionID: "failing",
		TotalRequests: 100,
		Successes:     100,
		TPS:           50,
		P99LatencyMs:  200,
	}
	val := validator.ValidationResult{
		SubmissionID:    "failing",
		Valid:           false,
		OrdersProcessed: 100,
		CrossEvents:    5,
	}

	s := scorer.ComputeScore(metrics, val)

	if s.Grade != "F" {
		t.Errorf("expected Grade F, got %q (total=%.2f)", s.Grade, s.TotalScore)
	}
	if s.TotalScore >= 45 {
		t.Errorf("expected TotalScore < 45, got %.2f", s.TotalScore)
	}
	if s.LatencyScore != 0 {
		t.Errorf("P99=200ms should give LatencyScore=0, got %.2f", s.LatencyScore)
	}
	if s.ThroughputScore != 0 {
		t.Errorf("TPS=50 should give ThroughputScore=0, got %.2f", s.ThroughputScore)
	}
	t.Logf("Score: Total=%.1f  Latency=%.1f  Throughput=%.1f  Correctness=%.1f  Grade=%s",
		s.TotalScore, s.LatencyScore, s.ThroughputScore, s.CorrectnessScore, s.Grade)
}

func TestAssignGradeBoundaries(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{100, "S"},
		{90, "S"},
		{89.9, "A"},
		{75, "A"},
		{74.9, "B"},
		{60, "B"},
		{59.9, "C"},
		{45, "C"},
		{44.9, "F"},
		{0, "F"},
	}
	for _, tt := range tests {
		got := scorer.AssignGrade(tt.score)
		if got != tt.grade {
			t.Errorf("AssignGrade(%.1f): got %q, want %q", tt.score, got, tt.grade)
		}
	}
}

func TestMidRangeSubmission(t *testing.T) {
	// P99=50ms, TPS=2500 → should land around grade B.
	metrics := scorer.PerformanceMetrics{
		SubmissionID: "mid",
		TotalRequests: 5000,
		Successes:     5000,
		TPS:           2500,
		P99LatencyMs:  50,
	}
	val := validator.ValidationResult{
		SubmissionID:    "mid",
		Valid:           true,
		OrdersProcessed: 5000,
		CrossEvents:    0,
	}
	s := scorer.ComputeScore(metrics, val)
	t.Logf("Mid score: Total=%.1f Grade=%s", s.TotalScore, s.Grade)
	if s.Grade != "B" && s.Grade != "A" {
		t.Errorf("expected Grade B or A for mid-range, got %q (%.2f)", s.Grade, s.TotalScore)
	}
}
