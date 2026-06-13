package main

import (
	"fmt"

	"github.com/Saurabh-52/trading-platform/internal/scorer"
	"github.com/Saurabh-52/trading-platform/internal/validator"
)

func main() {
	metrics := scorer.PerformanceMetrics{
		SubmissionID:  "test-submission-001",
		TotalRequests: 7,
		Successes:     7,
		TPS:           2.9572714246881954,
		P99LatencyMs:  7.2307,
	}
	val := validator.ValidationResult{
		SubmissionID:    "test-submission-001",
		Valid:           true,
		OrdersProcessed: 7,
		CrossEvents:     0,
	}

	sc := scorer.ComputeScore(metrics, val)
	fmt.Printf("Computed Score: Total=%.4f Latency=%.4f Throughput=%.4f Correctness=%.4f Grade=%s\n",
		sc.TotalScore, sc.LatencyScore, sc.ThroughputScore, sc.CorrectnessScore, sc.Grade)
}
