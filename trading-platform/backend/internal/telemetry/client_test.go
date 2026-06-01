package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/Saurabh-52/trading-platform/internal/telemetry"
)

// newTestRedis starts an in-memory miniredis and returns a *redis.Client for it.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestPublishAndConsumeRoundTrip(t *testing.T) {
	client := newTestRedis(t)
	ctx := context.Background()

	want := telemetry.TelemetryEvent{
		SubmissionID: "pod-001",
		BotID:        3,
		Sequence:     7,
		Action:       "NEW",
		Side:         "BUY",
		Price:        100.50,
		Quantity:     25,
		StatusCode:   200,
		LatencyMs:    2.345,
		Timestamp:    time.Now().UTC().Truncate(time.Millisecond),
		EngineOutput: `{"status":"ok"}`,
	}
	if err := telemetry.PublishEvent(ctx, client, want); err != nil {
		t.Fatalf("PublishEvent: %v", err)
	}

	events, nextID, err := telemetry.ConsumeEvents(ctx, client, "0", 10)
	if err != nil {
		t.Fatalf("ConsumeEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if nextID == "0" {
		t.Fatal("cursor should have advanced past '0'")
	}
	got := events[0]
	checks := []struct {
		name string
		ok   bool
	}{
		{"SubmissionID", got.SubmissionID == want.SubmissionID},
		{"BotID", got.BotID == want.BotID},
		{"Sequence", got.Sequence == want.Sequence},
		{"Action", got.Action == want.Action},
		{"Side", got.Side == want.Side},
		{"StatusCode", got.StatusCode == want.StatusCode},
		{"EngineOutput", got.EngineOutput == want.EngineOutput},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("field %s mismatch: got %+v, want %+v", c.name, got, want)
		}
	}
	if d := got.LatencyMs - want.LatencyMs; d < -0.001 || d > 0.001 {
		t.Errorf("LatencyMs: got %.6f, want %.6f", got.LatencyMs, want.LatencyMs)
	}
}

func TestConsumeEventsEmptyStream(t *testing.T) {
	client := newTestRedis(t)
	ctx := context.Background()

	events, lastID, err := telemetry.ConsumeEvents(ctx, client, "0", 10)
	if err != nil {
		t.Fatalf("unexpected error on empty stream: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
	if lastID != "0" {
		t.Errorf("cursor should stay '0' on empty stream, got %q", lastID)
	}
}

func TestConsumeEventsIncrementalCursor(t *testing.T) {
	client := newTestRedis(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		e := telemetry.TelemetryEvent{
			SubmissionID: "sub-001", BotID: i, Sequence: i,
			Action: "NEW", StatusCode: 200, Timestamp: time.Now().UTC(),
		}
		if err := telemetry.PublishEvent(ctx, client, e); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	batch1, cursor, err := telemetry.ConsumeEvents(ctx, client, "0", 3)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}
	if len(batch1) != 3 {
		t.Fatalf("expected 3 in batch1, got %d", len(batch1))
	}

	batch2, _, err := telemetry.ConsumeEvents(ctx, client, cursor, 10)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}
	if len(batch2) != 2 {
		t.Fatalf("expected 2 in batch2, got %d", len(batch2))
	}
}

func TestConsumeAllForSubmission(t *testing.T) {
	client := newTestRedis(t)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		_ = telemetry.PublishEvent(ctx, client, telemetry.TelemetryEvent{
			SubmissionID: "target", Action: "NEW", StatusCode: 200, Timestamp: time.Now().UTC(),
		})
	}
	for i := 0; i < 3; i++ {
		_ = telemetry.PublishEvent(ctx, client, telemetry.TelemetryEvent{
			SubmissionID: "other", Action: "NEW", StatusCode: 200, Timestamp: time.Now().UTC(),
		})
	}

	events, err := telemetry.ConsumeAllForSubmission(ctx, client, "target")
	if err != nil {
		t.Fatalf("ConsumeAllForSubmission: %v", err)
	}
	if len(events) != 4 {
		t.Errorf("expected 4, got %d", len(events))
	}
	for _, e := range events {
		if e.SubmissionID != "target" {
			t.Errorf("got wrong submission: %q", e.SubmissionID)
		}
	}
}

func TestTruncate512(t *testing.T) {
	long := string(make([]byte, 1000))
	got := telemetry.Truncate512(long)
	if len(got) != 512 {
		t.Errorf("expected 512, got %d", len(got))
	}
	if s := telemetry.Truncate512("hi"); s != "hi" {
		t.Error("short string should be unchanged")
	}
}

func TestNewRedisClientBadAddr(t *testing.T) {
	_, err := telemetry.NewRedisClient("localhost:19999")
	if err == nil {
		t.Fatal("expected error connecting to non-existent Redis")
	}
}
