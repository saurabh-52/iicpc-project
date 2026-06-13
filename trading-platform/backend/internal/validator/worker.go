package validator

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Saurabh-52/trading-platform/internal/telemetry"
)

// ValidationResult summarises whether the engine's orderbook behaviour was valid
// during the observation window.
type ValidationResult struct {
	SubmissionID      string    `json:"submission_id"`
	Valid             bool      `json:"valid"`
	Errors            []string  `json:"errors,omitempty"`
	OrdersProcessed   int       `json:"orders_processed"`
	CrossEvents       int       `json:"cross_events"`
	MismatchEvents    int       `json:"mismatch_events"`
	UnparseableEvents int       `json:"unparseable_events"`
	Timestamp         time.Time `json:"timestamp"`
}

// TotalErrors returns the sum of all error types for scoring purposes.
func (r ValidationResult) TotalErrors() int {
	return r.CrossEvents + r.MismatchEvents + r.UnparseableEvents
}

// CorrectnessHint returns a human-readable hint when the submission gets 0%
// correctness.
func (r ValidationResult) CorrectnessHint() string {
	if r.OrdersProcessed == 0 {
		return "No orders were successfully processed by the validator. This usually means your engine failed to receive or respond to the stress test requests (e.g., due to connection reset, timeout, or crash). Check your engine's network protocol, port mapping, and startup logs."
	}
	if r.OrdersProcessed > 0 && r.TotalErrors() >= r.OrdersProcessed {
		if r.UnparseableEvents == r.OrdersProcessed {
			return "Your engine responses do not include best_bid and best_ask fields. " +
				"To receive a correctness score, your engine must return JSON like: " +
				`{"status":"accepted","best_bid":99.95,"best_ask":100.05} ` +
				"so the validator can verify your orderbook state after each order."
		}
		if r.CrossEvents == r.OrdersProcessed {
			return "All responses reported a crossed book (best_bid >= best_ask). Ensure your matching engine correctly executes trades when prices cross, rather than keeping overlapping buy and sell orders in the book."
		}
		if r.MismatchEvents == r.OrdersProcessed {
			return "All responses had BBO price mismatches compared to the reference matching engine. Ensure your price-time priority sorting, order matching, and BBO reporting logic are correct."
		}
		return fmt.Sprintf("All processed orders failed correctness validation (%d crosses, %d mismatches, %d unparseable). Ensure your engine logic matches standard price-time FIFO matching rules.",
			r.CrossEvents, r.MismatchEvents, r.UnparseableEvents)
	}
	if r.OrdersProcessed > 0 && r.UnparseableEvents > 0 {
		return fmt.Sprintf(
			"%d of %d responses could not be parsed for best_bid/best_ask. "+
				"Ensure every response includes these fields for full correctness credit.",
			r.UnparseableEvents, r.OrdersProcessed,
		)
	}
	return ""
}

// validateEvent processes a single telemetry event against the reference
// orderbook and the engine's reported output.
func validateEvent(ob *Orderbook, e telemetry.TelemetryEvent, result *ValidationResult) {
	// Feed the order into the reference matching engine.
	switch e.Action {
	case "NEW":
		ob.AddOrder(e.Side, e.Price, e.Quantity)
	case "CANCEL":
		ob.CancelOrder(e.Side, e.Price, e.Quantity)
	}

	// Parse the engine's response for reported BBO.
	parsed := ParseEngineOutput(e.EngineOutput)

	if !parsed.Parsed {
		// Engine didn't report BBO — can't verify correctness.
		result.UnparseableEvents++
		return
	}

	// Check 1: Is the user's reported book crossed?
	if IsCrossed(parsed) {
		result.CrossEvents++
		result.Errors = append(result.Errors,
			fmt.Sprintf("engine reports crossed book: best_bid=%.6f >= best_ask=%.6f",
				parsed.BestBid, parsed.BestAsk))
		return
	}

	// Check 2: Does the user's BBO match the reference?
	refBid, hasBid, refAsk, hasAsk := ob.BBO()
	if !BBOMatches(parsed, refBid, hasBid, refAsk, hasAsk) {
		result.MismatchEvents++
		result.Errors = append(result.Errors,
			fmt.Sprintf("BBO mismatch: engine reports bid=%.6f ask=%.6f, reference bid=%.6f ask=%.6f",
				parsed.BestBid, parsed.BestAsk, refBid, refAsk))
	}
}

// RunValidator consumes telemetry events for a submission and validates them
// using response-based comparison against a reference matching engine.
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
			result.Valid = result.TotalErrors() == 0
			return result, ctx.Err()
		case <-deadline:
			result.Valid = result.TotalErrors() == 0
			return result, nil
		default:
		}

		events, nextID, err := telemetry.ConsumeEvents(ctx, redisClient, lastID, 500)
		if err != nil {
			result.Valid = result.TotalErrors() == 0
			return result, err
		}

		if len(events) == 0 {
			// No new events — wait a bit before polling again.
			select {
			case <-time.After(20 * time.Millisecond):
			case <-deadline:
				result.Valid = result.TotalErrors() == 0
				return result, nil
			case <-ctx.Done():
				result.Valid = result.TotalErrors() == 0
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
			validateEvent(ob, e, &result)
		}
	}
}

// RunValidatorFromEvents replays a pre-fetched slice of events through the
// response-based validator. Useful in tests and the scoring pipeline where
// events have already been consumed from Redis.
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
		validateEvent(ob, e, &result)
	}
	result.Valid = result.TotalErrors() == 0
	return result
}
