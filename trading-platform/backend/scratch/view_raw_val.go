package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	redis "github.com/redis/go-redis/v9"
	"github.com/Saurabh-52/trading-platform/internal/telemetry"
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

	var subID, language, filename, sourceCode string
	var total float64
	err = conn.QueryRow(ctx, `
		SELECT submission_id, language, filename, source_code, total_score
		FROM submission_results
		ORDER BY submitted_at DESC
		LIMIT 1
	`).Scan(&subID, &language, &filename, &sourceCode, &total)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Printf("SubID: %s\n", subID)
	fmt.Printf("Language: %s | Filename: %s | Total Score: %.2f\n", language, filename, total)
	if len(sourceCode) > 200 {
		fmt.Printf("Source Code (truncated):\n%s\n", sourceCode[:200])
	} else {
		fmt.Printf("Source Code:\n%s\n", sourceCode)
	}

	// Connect to Redis to look up telemetry events for this submission
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	rClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rClient.Close()

	fmt.Printf("\nFetching telemetry events from Redis...\n")
	scoreCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	events, err := telemetry.ConsumeAllForSubmission(scoreCtx, rClient, subID)
	if err != nil {
		log.Printf("ConsumeAllForSubmission failed: %v", err)
	} else {
		fmt.Printf("Found %d events in telemetry.\n", len(events))
		if len(events) > 5 {
			fmt.Printf("Sample Events:\n")
			for i := 0; i < 5; i++ {
				e := events[i]
				fmt.Printf("Seq: %d | Action: %s | Output: %q\n", e.Sequence, e.Action, e.EngineOutput)
			}
		} else {
			for _, e := range events {
				fmt.Printf("Seq: %d | Action: %s | Output: %q\n", e.Sequence, e.Action, e.EngineOutput)
			}
		}
	}
}
