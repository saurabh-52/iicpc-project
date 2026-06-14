package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/Saurabh-52/trading-platform/internal/scorer"
	"github.com/Saurabh-52/trading-platform/internal/validator"
)

func main() {
	pgURL := os.Getenv("POSTGRES_URL")
	if pgURL == "" {
		pgURL = "postgres://user:password@127.0.0.1:5432/postgres?sslmode=disable"
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer conn.Close(ctx)

	var subID string
	var rawMetrics, rawVal []byte
	err = conn.QueryRow(ctx, `
		SELECT submission_id, raw_metrics, raw_validation
		FROM submission_results
		WHERE submission_id = 'stress-1781418735035669900'
	`).Scan(&subID, &rawMetrics, &rawVal)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Printf("SubID: %s\n", subID)
	fmt.Printf("rawMetrics:\n%s\n", string(rawMetrics))
	fmt.Printf("rawVal:\n%s\n", string(rawVal))

	// Since rawMetrics might contain multiStrategyPayload (if rounds > 1), let's handle that.
	type roundResult struct {
		SubmissionID string                     `json:"submission_id"`
		Strategy     string                     `json:"strategy"`
		Metrics      interface{}                `json:"metrics"`
		Score        *scorer.Score              `json:"score,omitempty"`
		PerfMetrics  *scorer.PerformanceMetrics `json:"perf_metrics,omitempty"`
		ValResult    *validator.ValidationResult `json:"val_result,omitempty"`
	}
	type multiStrategyPayload struct {
		IsMultiStrategy bool          `json:"is_multi_strategy"`
		Rounds          []roundResult `json:"rounds"`
	}

	var payload multiStrategyPayload
	if err := json.Unmarshal(rawMetrics, &payload); err == nil && len(payload.Rounds) > 0 {
		fmt.Printf("Rounds count = %d\n", len(payload.Rounds))
		for idx, r := range payload.Rounds {
			fmt.Printf("\n--- Round %d ---\n", idx+1)
			if r.PerfMetrics != nil && r.ValResult != nil {
				sc := scorer.ComputeScore(*r.PerfMetrics, *r.ValResult)
				fmt.Printf("Round strategy: %s\n", r.Strategy)
				fmt.Printf("Round metrics: TPS=%.4f P99=%.4f\n", r.PerfMetrics.TPS, r.PerfMetrics.P99LatencyMs)
				fmt.Printf("Round validation: OrdersProcessed=%d CrossEvents=%d MismatchEvents=%d UnparseableEvents=%d\n",
					r.ValResult.OrdersProcessed, r.ValResult.CrossEvents, r.ValResult.MismatchEvents, r.ValResult.UnparseableEvents)
				fmt.Printf("Re-computed Round Score: Total=%.4f Latency=%.4f Throughput=%.4f Correctness=%.4f Grade=%s\n",
					sc.TotalScore, sc.LatencyScore, sc.ThroughputScore, sc.CorrectnessScore, sc.Grade)
				if r.Score != nil {
					fmt.Printf("Stored Round Score:      Total=%.4f Latency=%.4f Throughput=%.4f Correctness=%.4f Grade=%s\n",
						r.Score.TotalScore, r.Score.LatencyScore, r.Score.ThroughputScore, r.Score.CorrectnessScore, r.Score.Grade)
				}
			} else {
				fmt.Printf("PerfMetrics or ValResult is nil in round!\n")
			}
		}
	} else {
		// Single strategy
		var metrics scorer.PerformanceMetrics
		if err := json.Unmarshal(rawMetrics, &metrics); err != nil {
			log.Fatalf("Unmarshal rawMetrics failed: %v", err)
		}
		var val validator.ValidationResult
		if err := json.Unmarshal(rawVal, &val); err != nil {
			log.Fatalf("Unmarshal rawVal failed: %v", err)
		}

		sc := scorer.ComputeScore(metrics, val)
		fmt.Printf("Parsed Metrics: TPS=%.4f, P99=%.4f, Successes=%d, TotalRequests=%d\n",
			metrics.TPS, metrics.P99LatencyMs, metrics.Successes, metrics.TotalRequests)
		fmt.Printf("Single strategy re-computed score:\n")
		fmt.Printf("Total=%.4f Latency=%.4f Throughput=%.4f Correctness=%.4f Grade=%s\n",
			sc.TotalScore, sc.LatencyScore, sc.ThroughputScore, sc.CorrectnessScore, sc.Grade)
	}
}

