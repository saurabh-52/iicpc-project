package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Stream is the Redis Stream key used for all telemetry events.
const Stream = "telemetry:events"

// RedisClient wraps go-redis with telemetry helpers.
type RedisClient struct {
	c *redis.Client
}

// NewRedisClient connects to Redis at addr and pings it.
// Returns an error if Redis is unreachable.
func NewRedisClient(addr string) (*RedisClient, error) {
	c := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("telemetry: redis ping failed (%s): %w", addr, err)
	}
	return &RedisClient{c: c}, nil
}

// Underlying returns the raw *redis.Client.
func (r *RedisClient) Underlying() *redis.Client { return r.c }

// Close shuts down the underlying connection.
func (r *RedisClient) Close() error { return r.c.Close() }

// PublishEvent writes one TelemetryEvent to the Redis Stream via XADD.
// The stream is soft-capped at 100 000 entries.
func PublishEvent(ctx context.Context, client *redis.Client, event TelemetryEvent) error {
	return client.XAdd(ctx, &redis.XAddArgs{
		Stream: Stream,
		MaxLen: 100_000,
		Approx: true,
		ID:     "*",
		Values: event.ToMap(),
	}).Err()
}

// PublishEventsBatch writes multiple TelemetryEvents to the Redis Stream via a Pipeline.
// This significantly reduces connection overhead and drops when dealing with high TPS.
func PublishEventsBatch(ctx context.Context, client *redis.Client, events []TelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}
	pipe := client.Pipeline()
	for _, event := range events {
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: Stream,
			MaxLen: 100_000,
			Approx: true,
			ID:     "*",
			Values: event.ToMap(),
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}

// ConsumeEvents reads up to count events starting AFTER lastID using XRANGE
// (always non-blocking — returns immediately even on an empty stream).
// Pass "0" as lastID to start from the beginning of the stream.
// Returns events, the new cursor ID, and any error.
func ConsumeEvents(ctx context.Context, client *redis.Client, lastID string, count int64) ([]TelemetryEvent, string, error) {
	// Build an exclusive start: "(ID" tells Redis to start strictly after lastID.
	start := lastID
	if start != "0" {
		start = "(" + lastID
	}

	msgs, err := client.XRangeN(ctx, Stream, start, "+", count).Result()
	if err != nil && err != redis.Nil {
		return nil, lastID, fmt.Errorf("telemetry: XRANGE failed: %w", err)
	}

	var events []TelemetryEvent
	newLastID := lastID

	for _, msg := range msgs {
		raw := make(map[string]string, len(msg.Values))
		for k, v := range msg.Values {
			if s, ok := v.(string); ok {
				raw[k] = s
			}
		}
		events = append(events, FromMap(raw))
		newLastID = msg.ID
	}

	return events, newLastID, nil
}

// ConsumeAllForSubmission pages through the entire stream and returns only
// events that match the given submissionID.
func ConsumeAllForSubmission(ctx context.Context, client *redis.Client, submissionID string) ([]TelemetryEvent, error) {
	var all []TelemetryEvent
	lastID := "0"
	for {
		events, next, err := ConsumeEvents(ctx, client, lastID, 500)
		if err != nil {
			return nil, err
		}
		for _, e := range events {
			if e.SubmissionID == submissionID {
				all = append(all, e)
			}
		}
		if len(events) == 0 {
			break
		}
		lastID = next
	}
	return all, nil
}

// TrimStream removes all entries from the telemetry Redis Stream.
// Call this after contest finalization to prevent stale events from
// accumulating and degrading performance of subsequent ConsumeAllForSubmission calls.
func TrimStream(ctx context.Context, client *redis.Client) error {
	return client.XTrimMaxLen(ctx, Stream, 0).Err()
}

