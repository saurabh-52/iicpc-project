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
		SELECT submission_id, total_score, latency_score, throughput_score, correctness_score, grade, tps, p99_latency_ms
		FROM submission_results
		ORDER BY submitted_at DESC
	`)
	if err != nil {
		log.Fatalf("Query failed: %v\n", err)
	}
	defer rows.Close()

	fmt.Println("All Submission Results:")
	fmt.Printf("%-25s %-6s %-6s %-6s %-6s %-5s %-8s %-8s\n", 
		"Submission ID", "Total", "LatSc", "ThrSc", "CorrSc", "Grade", "TPS", "P99")
	fmt.Println(string(make([]byte, 80)))

	for rows.Next() {
		var subID, grade string
		var total, lat, thr, corr, tps, p99 float64
		err := rows.Scan(&subID, &total, &lat, &thr, &corr, &grade, &tps, &p99)
		if err != nil {
			log.Fatalf("Scan failed: %v\n", err)
		}
		
		fmt.Printf("%-25s %-6.2f %-6.2f %-6.2f %-6.2f %-5s %-8.2f %-8.2f\n",
			subID, total, lat, thr, corr, grade, tps, p99)
	}
}
