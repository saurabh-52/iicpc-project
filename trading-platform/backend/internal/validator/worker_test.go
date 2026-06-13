package validator_test

import (
	"context"
	"fmt"
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

// goodBBOOutput returns a valid JSON engine response with the given BBO.
func goodBBOOutput(bestBid, bestAsk float64) string {
	if bestBid == 0 && bestAsk == 0 {
		return `{"status":"accepted"}`
	}
	if bestBid == 0 {
		return `{"status":"accepted","best_ask":` + fmtFloat(bestAsk) + `}`
	}
	if bestAsk == 0 {
		return `{"status":"accepted","best_bid":` + fmtFloat(bestBid) + `}`
	}
	return `{"status":"accepted","best_bid":` + fmtFloat(bestBid) + `,"best_ask":` + fmtFloat(bestAsk) + `}`
}

func fmtFloat(f float64) string {
	return fmt.Sprintf("%.1f", f)
}

// TestValidSequence publishes 20 non-crossing orders with correct BBO responses
// and asserts the book remains valid throughout.
func TestValidSequence(t *testing.T) {
	_, client := newRedis(t)
	ctx := context.Background()

	const subID = "valid-test"

	// Build 20 events: 10 BUY orders at prices 90-99, 10 SELL orders at prices 101-110.
	// This guarantees the book never crosses (best bid 99 < best ask 101).
	var events []telemetry.TelemetryEvent

	// Reference engine state for generating correct BBO output.
	refOB := &validator.Orderbook{}

	for i := 0; i < 10; i++ {
		price := 90 + float64(i)
		refOB.AddOrder("BUY", price, 10)
		bid, _, ask, _ := refOB.BBO()

		events = append(events, telemetry.TelemetryEvent{
			SubmissionID: subID,
			BotID:        i,
			Sequence:     i,
			Action:       "NEW",
			Side:         "BUY",
			Price:        price,
			Quantity:     10,
			StatusCode:   200,
			LatencyMs:    1.0,
			Timestamp:    time.Now().UTC(),
			EngineOutput: goodBBOOutput(bid, ask),
		})
	}
	for i := 0; i < 10; i++ {
		price := 101 + float64(i)
		refOB.AddOrder("SELL", price, 10)
		bid, _, ask, _ := refOB.BBO()

		events = append(events, telemetry.TelemetryEvent{
			SubmissionID: subID,
			BotID:        10 + i,
			Sequence:     10 + i,
			Action:       "NEW",
			Side:         "SELL",
			Price:        price,
			Quantity:     10,
			StatusCode:   200,
			LatencyMs:    1.0,
			Timestamp:    time.Now().UTC(),
			EngineOutput: goodBBOOutput(bid, ask),
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
	if result.MismatchEvents != 0 {
		t.Errorf("MismatchEvents: got %d, want 0", result.MismatchEvents)
	}
	if result.UnparseableEvents != 0 {
		t.Errorf("UnparseableEvents: got %d, want 0", result.UnparseableEvents)
	}
	if !result.Valid {
		t.Error("expected Valid=true")
	}
}

// TestCrossedBookDetected verifies that ValidateCross returns an error when
// bids and asks overlap (crossed book).
func TestCrossedBookDetected(t *testing.T) {
	ob := &validator.Orderbook{}

	// Bids higher than Asks -> crossed!
	ob.AddOrder("SELL", 100.0, 10)
	ob.AddOrder("BUY", 95.0, 5)
	if err := ob.ValidateCross(); err != nil {
		t.Errorf("expected clean book, got: %v", err)
	}

	// Manually inject a crossing bid level to simulate a bug/cross
	ob.Bids = []validator.Level{{Price: 101.0, TotalQty: 5}}
	if err := ob.ValidateCross(); err == nil {
		t.Error("expected crossed book error when bid >= ask")
	}
}

// TestCancelRestoresValidity verifies that cancelling the crossing order
// makes the book valid again (only the orders that created the cross are flagged).
func TestCancelRestoresValidity(t *testing.T) {
	_, client := newRedis(t)
	ctx := context.Background()

	const subID = "cancel-test"

	// Build a reference to compute correct BBO.
	refOB := &validator.Orderbook{}
	refOB.AddOrder("SELL", 100, 10)
	bidAfterSell, _, askAfterSell, _ := refOB.BBO()

	refOB.AddOrder("BUY", 95, 10)
	bidAfterBuy, _, askAfterBuy, _ := refOB.BBO()

	refOB.CancelOrder("BUY", 95, 10)
	bidAfterCancel, _, askAfterCancel, _ := refOB.BBO()

	events := []telemetry.TelemetryEvent{
		{SubmissionID: subID, Action: "NEW", Side: "SELL", Price: 100, Quantity: 10, StatusCode: 200, Timestamp: time.Now().UTC(),
			EngineOutput: goodBBOOutput(bidAfterSell, askAfterSell)},
		{SubmissionID: subID, Action: "NEW", Side: "BUY", Price: 95, Quantity: 10, StatusCode: 200, Timestamp: time.Now().UTC(),
			EngineOutput: goodBBOOutput(bidAfterBuy, askAfterBuy)},
		// Cancel the buy at 95
		{SubmissionID: subID, Action: "CANCEL", Side: "BUY", Price: 95, Quantity: 10, StatusCode: 200, Timestamp: time.Now().UTC(),
			EngineOutput: goodBBOOutput(bidAfterCancel, askAfterCancel)},
	}

	publishEvents(t, ctx, client, events)

	result, err := validator.RunValidator(ctx, client, subID, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("RunValidator: %v", err)
	}
	if result.OrdersProcessed != 3 {
		t.Errorf("OrdersProcessed: got %d, want 3", result.OrdersProcessed)
	}
	if result.TotalErrors() != 0 {
		t.Errorf("TotalErrors: got %d, want 0 (cross=%d, mismatch=%d, unparseable=%d)",
			result.TotalErrors(), result.CrossEvents, result.MismatchEvents, result.UnparseableEvents)
	}
	if !result.Valid {
		t.Error("expected Valid=true after cancel")
	}
}

// TestRunValidatorFromEvents tests the non-streaming path.
func TestRunValidatorFromEvents(t *testing.T) {
	events := []telemetry.TelemetryEvent{
		{SubmissionID: "sub-1", Action: "NEW", Side: "BUY", Price: 90, Quantity: 10,
			EngineOutput: `{"status":"accepted","best_bid":90.0}`},
		{SubmissionID: "sub-1", Action: "NEW", Side: "SELL", Price: 100, Quantity: 10,
			EngineOutput: `{"status":"accepted","best_bid":90.0,"best_ask":100.0}`},
		{SubmissionID: "sub-1", Action: "LOG", EngineOutput: "ignored"},
		{SubmissionID: "sub-1", Action: "NEW", Side: "BUY", Price: 95, Quantity: 5,
			EngineOutput: `{"status":"accepted","best_bid":95.0,"best_ask":100.0}`},
	}

	result := validator.RunValidatorFromEvents("sub-1", events)
	if result.OrdersProcessed != 3 {
		t.Errorf("OrdersProcessed: got %d, want 3 (LOG should be skipped)", result.OrdersProcessed)
	}
	if !result.Valid {
		t.Errorf("expected Valid=true, got errors: cross=%d mismatch=%d unparseable=%d",
			result.CrossEvents, result.MismatchEvents, result.UnparseableEvents)
	}
}

// TestUnparseableEngineOutput verifies that responses without BBO are penalized.
func TestUnparseableEngineOutput(t *testing.T) {
	events := []telemetry.TelemetryEvent{
		{SubmissionID: "sub-2", Action: "NEW", Side: "BUY", Price: 90, Quantity: 10,
			EngineOutput: `{"status":"ok"}`},
		{SubmissionID: "sub-2", Action: "NEW", Side: "SELL", Price: 100, Quantity: 10,
			EngineOutput: `OK`},
		{SubmissionID: "sub-2", Action: "NEW", Side: "BUY", Price: 95, Quantity: 5,
			EngineOutput: ``},
	}

	result := validator.RunValidatorFromEvents("sub-2", events)
	if result.OrdersProcessed != 3 {
		t.Errorf("OrdersProcessed: got %d, want 3", result.OrdersProcessed)
	}
	if result.UnparseableEvents != 3 {
		t.Errorf("UnparseableEvents: got %d, want 3", result.UnparseableEvents)
	}
	if result.Valid {
		t.Error("expected Valid=false for all-unparseable responses")
	}

	// Verify the hint message is generated
	hint := result.CorrectnessHint()
	if hint == "" {
		t.Error("expected non-empty CorrectnessHint for all-unparseable submission")
	}
}

// TestCrossedBBOReported verifies that a user-reported crossed book is detected.
func TestCrossedBBOReported(t *testing.T) {
	events := []telemetry.TelemetryEvent{
		{SubmissionID: "sub-3", Action: "NEW", Side: "BUY", Price: 90, Quantity: 10,
			EngineOutput: `{"status":"accepted","best_bid":90.0}`},
		{SubmissionID: "sub-3", Action: "NEW", Side: "SELL", Price: 100, Quantity: 10,
			EngineOutput: `{"status":"accepted","best_bid":90.0,"best_ask":100.0}`},
		// Engine reports crossed book! best_bid >= best_ask
		{SubmissionID: "sub-3", Action: "NEW", Side: "BUY", Price: 105, Quantity: 5,
			EngineOutput: `{"status":"accepted","best_bid":105.0,"best_ask":100.0}`},
	}

	result := validator.RunValidatorFromEvents("sub-3", events)
	if result.CrossEvents != 1 {
		t.Errorf("CrossEvents: got %d, want 1", result.CrossEvents)
	}
	if result.Valid {
		t.Error("expected Valid=false for crossed book report")
	}
}

// TestBBOMismatch verifies that incorrect BBO (different from reference) is detected.
func TestBBOMismatch(t *testing.T) {
	events := []telemetry.TelemetryEvent{
		{SubmissionID: "sub-4", Action: "NEW", Side: "BUY", Price: 90, Quantity: 10,
			EngineOutput: `{"status":"accepted","best_bid":90.0}`},
		{SubmissionID: "sub-4", Action: "NEW", Side: "SELL", Price: 100, Quantity: 10,
			// Engine reports wrong best_bid (should be 90.0, reports 85.0)
			EngineOutput: `{"status":"accepted","best_bid":85.0,"best_ask":100.0}`},
	}

	result := validator.RunValidatorFromEvents("sub-4", events)
	if result.MismatchEvents != 1 {
		t.Errorf("MismatchEvents: got %d, want 1", result.MismatchEvents)
	}
	if result.Valid {
		t.Error("expected Valid=false for BBO mismatch")
	}
}
