//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	pgURL := os.Getenv("POSTGRES_URL")
	if pgURL == "" {
		pgURL = "postgres://user:password@127.0.0.1:5432/postgres?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		fmt.Printf("Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	rows, err := pool.Query(context.Background(), "SELECT contest_id, system_name, user_id FROM contest_registrations")
	if err != nil {
		fmt.Printf("Error querying records: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	for rows.Next() {
		var c, s, u string
		rows.Scan(&c, &s, &u)
		fmt.Printf("Contest: %s, System: %s, User: %s\n", c, s, u)
	}
}
