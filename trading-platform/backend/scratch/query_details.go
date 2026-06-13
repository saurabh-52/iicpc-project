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
		SELECT submission_id, strategy, total_score, latency_score, throughput_score, correctness_score, orders_processed, cross_events, raw_validation
		FROM submission_results
		ORDER BY submitted_at DESC
		LIMIT 20
	`)
	if err != nil {
		log.Fatalf("Query failed: %v\n", err)
	}
	defer rows.Close()

	for rows.Next() {
		var subID, strategy string
		var total, lat, thr, corr float64
		var orders, cross int
		var rawVal []byte
		err := rows.Scan(&subID, &strategy, &total, &lat, &thr, &corr, &orders, &cross, &rawVal)
		if err != nil {
			log.Fatalf("Scan failed: %v\n", err)
		}
		
		fmt.Printf("SubID: %s | Strat: %15s | Total: %6.2f | Lat: %6.2f | Thr: %6.2f | Corr: %6.2f | Orders: %6d | Cross: %6d\n",
			subID, strategy, total, lat, thr, corr, orders, cross)
	}
}
