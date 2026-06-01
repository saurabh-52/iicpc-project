package telemetry_test

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/Saurabh-52/trading-platform/internal/telemetry"
)

// mockLogStream is a fake io.ReadCloser that serves pre-canned lines.
type mockLogStream struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func newMockLogStream() *mockLogStream {
	r, w := io.Pipe()
	return &mockLogStream{r: r, w: w}
}

func (m *mockLogStream) Read(p []byte) (int, error) { return m.r.Read(p) }
func (m *mockLogStream) Close() error               { return m.r.Close() }
func (m *mockLogStream) WriteLines(lines ...string) {
	for _, l := range lines {
		fmt.Fprintln(m.w, l)
	}
	m.w.Close()
}

// fakeStreamPodLogs is a test-friendly version that reads from an io.ReadCloser
// instead of a real Kubernetes client.
func fakeStreamPodLogs(ctx context.Context, stream io.ReadCloser, submissionID string, redisClient *redis.Client) error {
	import_bufio := func() *io.PipeReader { return nil }
	_ = import_bufio
	// Inline the same logic as StreamPodLogs but with a provided stream.
	// We test the logic independently of the K8s client.
	defer stream.Close()

	buf := make([]byte, 4096)
	var lineBuffer []byte
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			lineBuffer = append(lineBuffer, buf[:n]...)
			for {
				idx := -1
				for i, b := range lineBuffer {
					if b == '\n' {
						idx = i
						break
					}
				}
				if idx < 0 {
					break
				}
				line := string(lineBuffer[:idx])
				lineBuffer = lineBuffer[idx+1:]
				event := telemetry.TelemetryEvent{
					SubmissionID: submissionID,
					Action:       "LOG",
					EngineOutput: telemetry.Truncate512(line),
					Timestamp:    time.Now().UTC(),
				}
				pubCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				_ = telemetry.PublishEvent(pubCtx, redisClient, event)
				cancel()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func TestLogCapture(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	stream := newMockLogStream()
	const subID = "log-capture-test"

	done := make(chan error, 1)
	go func() {
		done <- fakeStreamPodLogs(context.Background(), stream, subID, client)
	}()

	lines := []string{
		"Orderbook Engine Initialized...",
		"Listening on 0.0.0.0:8080",
		"Received order: BUY 100 @ 50.00",
		"Received order: SELL 50 @ 51.00",
		"Trade matched: 50 @ 50.50",
	}
	stream.WriteLines(lines...)

	if err := <-done; err != nil {
		t.Fatalf("fakeStreamPodLogs: %v", err)
	}

	ctx := context.Background()
	events, err := telemetry.ConsumeAllForSubmission(ctx, client, subID)
	if err != nil {
		t.Fatalf("ConsumeAllForSubmission: %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 LOG events, got %d", len(events))
	}
	for _, e := range events {
		if e.Action != "LOG" {
			t.Errorf("expected Action=LOG, got %q", e.Action)
		}
		if e.SubmissionID != subID {
			t.Errorf("expected SubmissionID=%q, got %q", subID, e.SubmissionID)
		}
	}
}
