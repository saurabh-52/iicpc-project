package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
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

	rows, err := conn.Query(ctx, `
		SELECT submission_id, total_score, latency_score, throughput_score, correctness_score, raw_metrics, raw_validation
		FROM submission_results
		ORDER BY submitted_at DESC
		LIMIT 10
	`)
	if err != nil {
		log.Fatalf("Query failed: %v\n", err)
	}
	defer rows.Close()

	for rows.Next() {
		var subID string
		var total, lat, thr, corr float64
		var rawMetrics, rawVal []byte
		err := rows.Scan(&subID, &total, &lat, &thr, &corr, &rawMetrics, &rawVal)
		if err != nil {
			log.Fatalf("Scan failed: %v\n", err)
		}
		
		fmt.Printf("=========================================\n")
		fmt.Printf("Submission ID: %s\n", subID)
		fmt.Printf("Scores: Total=%.2f Latency=%.2f Throughput=%.2f Correctness=%.2f\n", total, lat, thr, corr)
		fmt.Printf("Raw Metrics: %s\n", string(rawMetrics))
		fmt.Printf("Raw Validation: %s\n", string(rawVal))
	}
}
