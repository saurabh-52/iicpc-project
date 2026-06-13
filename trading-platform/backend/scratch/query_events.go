package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/Saurabh-52/trading-platform/internal/telemetry"
)

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}

	c := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	ctx := context.Background()
	if err := c.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis ping failed: %v", err)
	}

	subIDs := []string{"stress-1781234612458541500"}

	for _, subID := range subIDs {
		fmt.Printf("\n==================== Events for %s ====================\n", subID)
		events, err := telemetry.ConsumeAllForSubmission(ctx, c, subID)
		if err != nil {
			log.Printf("Failed to consume events: %v", err)
			continue
		}
		fmt.Printf("Total events found: %d\n", len(events))
		if len(events) == 0 {
			continue
		}

		// Print first 10 events
		limit := 10
		if len(events) < limit {
			limit = len(events)
		}
		fmt.Println("First 10 events:")
		for i := 0; i < limit; i++ {
			e := events[i]
			fmt.Printf("  Seq: %d | Action: %s | Side: %s | Price: %.4f | Qty: %d | Status: %d | Latency: %.2f ms | Output: %s\n",
				e.Sequence, e.Action, e.Side, e.Price, e.Quantity, e.StatusCode, e.LatencyMs, e.EngineOutput)
		}

		// Check if any event has StatusCode >= 400 or empty side
		errorsCount := 0
		for _, e := range events {
			if e.StatusCode >= 400 || e.Action == "" || e.Side == "" {
				errorsCount++
			}
		}
		fmt.Printf("Events with HTTP/TCP errors or empty fields: %d\n", errorsCount)
	}
}
