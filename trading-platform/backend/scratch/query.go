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
		SELECT submission_id, raw_validation
		FROM submission_results
		WHERE submission_id = 'stress-1781227642682795900'
	`)
	if err != nil {
		log.Fatalf("Query failed: %v\n", err)
	}
	defer rows.Close()

	if rows.Next() {
		var subID string
		var rawVal []byte
		err := rows.Scan(&subID, &rawVal)
		if err != nil {
			log.Fatalf("Scan failed: %v\n", err)
		}
		
		fmt.Printf("Raw Validation for %s:\n%s\n", subID, string(rawVal))
	}
}
