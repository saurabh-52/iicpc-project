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

	subIDs := []string{"stress-1781225654796689100", "stress-1781227642682795900"}

	for _, subID := range subIDs {
		var rawVal []byte
		err := conn.QueryRow(ctx, `
			SELECT raw_validation
			FROM submission_results
			WHERE submission_id = $1
		`, subID).Scan(&rawVal)
		if err != nil {
			log.Printf("Query for %s failed: %v", subID, err)
			continue
		}
		fmt.Printf("\nRaw Validation for %s:\n%s\n", subID, string(rawVal))
	}
}
