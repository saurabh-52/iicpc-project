package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
	"github.com/Saurabh-52/trading-platform/internal/telemetry"
	"github.com/Saurabh-52/trading-platform/internal/validator"
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

	subIDs := []string{"stress-1781225654796689100", "stress-1781227642682795900"}

	for _, subID := range subIDs {
		fmt.Printf("\n--- Replaying events for %s ---\n", subID)
		events, err := telemetry.ConsumeAllForSubmission(ctx, c, subID)
		if err != nil {
			log.Fatalf("Failed to consume events: %v", err)
		}
		fmt.Printf("Loaded %d events from Redis.\n", len(events))

		res := validator.RunValidatorFromEvents(subID, events)
		fmt.Printf("ValidationResult: OrdersProcessed=%d | CrossEvents=%d | Valid=%v\n",
			res.OrdersProcessed, res.CrossEvents, res.Valid)
		if len(res.Errors) > 0 {
			fmt.Printf("First 5 errors:\n")
			limit := 5
			if len(res.Errors) < limit {
				limit = len(res.Errors)
			}
			for i := 0; i < limit; i++ {
				fmt.Printf("  %s\n", res.Errors[i])
			}
		}
	}
}
