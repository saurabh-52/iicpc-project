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
		SELECT submission_id, system_name, judging_mode, filename, total_score, submitted_at
		FROM submission_results
		ORDER BY submitted_at DESC
		LIMIT 10
	`)
	if err != nil {
		log.Fatalf("Query failed: %v\n", err)
	}
	defer rows.Close()

	fmt.Println("Recent submissions:")
	for rows.Next() {
		var subID, sysName, mode, filename string
		var score float64
		var submittedAt interface{}
		err := rows.Scan(&subID, &sysName, &mode, &filename, &score, &submittedAt)
		if err != nil {
			log.Fatalf("Scan failed: %v\n", err)
		}
		fmt.Printf("ID: %s | Team: %s | Mode: %s | Score: %.2f | Filename: %q | At: %v\n", subID, sysName, mode, score, filename, submittedAt)
	}
}
