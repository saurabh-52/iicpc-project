package validator

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Saurabh-52/trading-platform/internal/telemetry"
)

// ValidationResult summarises whether the engine's orderbook behaviour was valid
// during the observation window.
type ValidationResult struct {
	SubmissionID    string    `json:"submission_id"`
	Valid           bool      `json:"valid"`
	Errors          []string  `json:"errors,omitempty"`
	OrdersProcessed int      `json:"orders_processed"`
	CrossEvents     int      `json:"cross_events"`
	Timestamp       time.Time `json:"timestamp"`
}

// RunValidator consumes telemetry events for a submission and replays them
// through an in-memory Orderbook, checking for crossed-book violations.
//
// It reads events already present in the Redis Stream (non-blocking) and then
// polls at short intervals until windowDuration elapses or ctx is cancelled.
func RunValidator(
	ctx context.Context,
	redisClient *redis.Client,
	submissionID string,
	windowDuration time.Duration,
) (ValidationResult, error) {
	result := ValidationResult{
		SubmissionID: submissionID,
		Timestamp:    time.Now().UTC(),
	}
	ob := &Orderbook{}

	deadline := time.After(windowDuration)
	lastID := "0"

	for {
		select {
		case <-ctx.Done():
			result.Valid = result.CrossEvents == 0
			return result, ctx.Err()
		case <-deadline:
			result.Valid = result.CrossEvents == 0
			return result, nil
		default:
		}

		events, nextID, err := telemetry.ConsumeEvents(ctx, redisClient, lastID, 500)
		if err != nil {
			result.Valid = result.CrossEvents == 0
			return result, err
		}

		if len(events) == 0 {
			// No new events — wait a bit before polling again.
			select {
			case <-time.After(20 * time.Millisecond):
			case <-deadline:
				result.Valid = result.CrossEvents == 0
				return result, nil
			case <-ctx.Done():
				result.Valid = result.CrossEvents == 0
				return result, ctx.Err()
			}
			continue
		}

		lastID = nextID

		for _, e := range events {
			if e.SubmissionID != submissionID {
				continue
			}
			if e.Action == "LOG" {
				continue
			}

			result.OrdersProcessed++

			switch e.Action {
			case "NEW":
				ob.AddOrder(e.Side, e.Price, e.Quantity)
			case "CANCEL":
				ob.CancelOrder(e.Side, e.Price, e.Quantity)
			}

			if err := ob.ValidateCross(); err != nil {
				result.CrossEvents++
				result.Errors = append(result.Errors, err.Error())
			}
		}
	}
}

// RunValidatorFromEvents replays a pre-fetched slice of events through the
// orderbook validator.  Useful in tests and the scoring pipeline where events
// have already been consumed from Redis.
func RunValidatorFromEvents(submissionID string, events []telemetry.TelemetryEvent) ValidationResult {
	result := ValidationResult{
		SubmissionID: submissionID,
		Timestamp:    time.Now().UTC(),
	}
	ob := &Orderbook{}

	for _, e := range events {
		if e.SubmissionID != submissionID {
			continue
		}
		if e.Action == "LOG" {
			continue
		}
		result.OrdersProcessed++

		switch e.Action {
		case "NEW":
			ob.AddOrder(e.Side, e.Price, e.Quantity)
		case "CANCEL":
			ob.CancelOrder(e.Side, e.Price, e.Quantity)
		}

		if err := ob.ValidateCross(); err != nil {
			result.CrossEvents++
			result.Errors = append(result.Errors, err.Error())
		}
	}
	result.Valid = result.CrossEvents == 0
	return result
}
