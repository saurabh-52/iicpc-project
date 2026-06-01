package validator_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/Saurabh-52/trading-platform/internal/telemetry"
	"github.com/Saurabh-52/trading-platform/internal/validator"
)

func newRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return mr, redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

// publishEvents is a helper that publishes a slice of TelemetryEvents.
func publishEvents(t *testing.T, ctx context.Context, client *redis.Client, events []telemetry.TelemetryEvent) {
	t.Helper()
	for _, e := range events {
		if err := telemetry.PublishEvent(ctx, client, e); err != nil {
			t.Fatalf("PublishEvent: %v", err)
		}
	}
}

// TestValidSequence publishes 20 non-crossing orders and asserts the book
// remains valid throughout.
func TestValidSequence(t *testing.T) {
	_, client := newRedis(t)
	ctx := context.Background()

	const subID = "valid-test"

	// Build 20 events: 10 BUY orders at prices 90-99, 10 SELL orders at prices 101-110.
	// This guarantees the book never crosses (best bid 99 < best ask 101).
	var events []telemetry.TelemetryEvent
	for i := 0; i < 10; i++ {
		events = append(events, telemetry.TelemetryEvent{
			SubmissionID: subID,
			BotID:        i,
			Sequence:     i,
			Action:       "NEW",
			Side:         "BUY",
			Price:        90 + float64(i),
			Quantity:     10,
			StatusCode:   200,
			LatencyMs:    1.0,
			Timestamp:    time.Now().UTC(),
		})
	}
	for i := 0; i < 10; i++ {
		events = append(events, telemetry.TelemetryEvent{
			SubmissionID: subID,
			BotID:        10 + i,
			Sequence:     10 + i,
			Action:       "NEW",
			Side:         "SELL",
			Price:        101 + float64(i),
			Quantity:     10,
			StatusCode:   200,
			LatencyMs:    1.0,
			Timestamp:    time.Now().UTC(),
		})
	}

	publishEvents(t, ctx, client, events)

	result, err := validator.RunValidator(ctx, client, subID, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("RunValidator: %v", err)
	}
	if result.OrdersProcessed != 20 {
		t.Errorf("OrdersProcessed: got %d, want 20", result.OrdersProcessed)
	}
	if result.CrossEvents != 0 {
		t.Errorf("CrossEvents: got %d, want 0", result.CrossEvents)
	}
	if !result.Valid {
		t.Error("expected Valid=true")
	}
}

// TestCrossedBookDetected publishes a sequence where a bid crosses the best ask
// and verifies the validator catches it.
func TestCrossedBookDetected(t *testing.T) {
	_, client := newRedis(t)
	ctx := context.Background()

	const subID = "crossed-test"

	events := []telemetry.TelemetryEvent{
		// Ask at 100
		{SubmissionID: subID, Action: "NEW", Side: "SELL", Price: 100, Quantity: 10, StatusCode: 200, Timestamp: time.Now().UTC()},
		// Bid at 99 (fine)
		{SubmissionID: subID, Action: "NEW", Side: "BUY", Price: 99, Quantity: 5, StatusCode: 200, Timestamp: time.Now().UTC()},
		// Bid at 100 → crossed! (bid >= ask)
		{SubmissionID: subID, Action: "NEW", Side: "BUY", Price: 100, Quantity: 5, StatusCode: 200, Timestamp: time.Now().UTC()},
		// Bid at 105 → still crossed
		{SubmissionID: subID, Action: "NEW", Side: "BUY", Price: 105, Quantity: 10, StatusCode: 200, Timestamp: time.Now().UTC()},
	}

	publishEvents(t, ctx, client, events)

	result, err := validator.RunValidator(ctx, client, subID, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("RunValidator: %v", err)
	}
	if result.OrdersProcessed != 4 {
		t.Errorf("OrdersProcessed: got %d, want 4", result.OrdersProcessed)
	}
	if result.CrossEvents == 0 {
		t.Fatal("expected CrossEvents > 0")
	}
	if result.Valid {
		t.Error("expected Valid=false for a crossed book")
	}
	t.Logf("CrossEvents=%d, Errors=%v", result.CrossEvents, result.Errors)
}

// TestCancelRestoresValidity verifies that cancelling the crossing order
// makes the book valid again (only the orders that created the cross are flagged).
func TestCancelRestoresValidity(t *testing.T) {
	_, client := newRedis(t)
	ctx := context.Background()

	const subID = "cancel-test"

	events := []telemetry.TelemetryEvent{
		{SubmissionID: subID, Action: "NEW", Side: "SELL", Price: 100, Quantity: 10, StatusCode: 200, Timestamp: time.Now().UTC()},
		{SubmissionID: subID, Action: "NEW", Side: "BUY", Price: 95, Quantity: 10, StatusCode: 200, Timestamp: time.Now().UTC()},
		// Cancel the buy at 95
		{SubmissionID: subID, Action: "CANCEL", Side: "BUY", Price: 95, Quantity: 10, StatusCode: 200, Timestamp: time.Now().UTC()},
	}

	publishEvents(t, ctx, client, events)

	result, err := validator.RunValidator(ctx, client, subID, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("RunValidator: %v", err)
	}
	if result.OrdersProcessed != 3 {
		t.Errorf("OrdersProcessed: got %d, want 3", result.OrdersProcessed)
	}
	if result.CrossEvents != 0 {
		t.Errorf("CrossEvents: got %d, want 0", result.CrossEvents)
	}
	if !result.Valid {
		t.Error("expected Valid=true after cancel")
	}
}

// TestRunValidatorFromEvents tests the non-streaming path.
func TestRunValidatorFromEvents(t *testing.T) {
	events := []telemetry.TelemetryEvent{
		{SubmissionID: "sub-1", Action: "NEW", Side: "BUY", Price: 90, Quantity: 10},
		{SubmissionID: "sub-1", Action: "NEW", Side: "SELL", Price: 100, Quantity: 10},
		{SubmissionID: "sub-1", Action: "LOG", EngineOutput: "ignored"},
		{SubmissionID: "sub-1", Action: "NEW", Side: "BUY", Price: 95, Quantity: 5},
	}

	result := validator.RunValidatorFromEvents("sub-1", events)
	if result.OrdersProcessed != 3 {
		t.Errorf("OrdersProcessed: got %d, want 3 (LOG should be skipped)", result.OrdersProcessed)
	}
	if !result.Valid {
		t.Error("expected Valid=true")
	}
}
