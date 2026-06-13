//go:build integration

package main_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Saurabh-52/trading-platform/internal/botfleet"
	"github.com/Saurabh-52/trading-platform/internal/scorer"
	"github.com/Saurabh-52/trading-platform/internal/store"
	"github.com/Saurabh-52/trading-platform/internal/telemetry"
	"github.com/Saurabh-52/trading-platform/internal/validator"
)

// startPostgresContainer boots a real Postgres in Docker and returns a DSN.
func startPostgresContainer(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "integration",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")
	return "postgres://test:test@" + host + ":" + port.Port() + "/integration?sslmode=disable"
}

// TestFullPhase3Pipeline validates the entire telemetry → scoring → persistence
// pipeline end-to-end.
func TestFullPhase3Pipeline(t *testing.T) {
	ctx := context.Background()

	// 1. In-process Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer redisClient.Close()

	// 2. Real Postgres in Docker
	pgDSN := startPostgresContainer(t)
	db, err := store.NewStore(ctx, pgDSN)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer db.Close()

	// 3. Mock trading engine — always 200 OK
	var engineHits int64
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&engineHits, 1)
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer engine.Close()

	const submissionID = "test-submission-001"

	// 4. Run bot fleet with telemetry
	summary, err := botfleet.Run(ctx, botfleet.Config{
		Target:          engine.URL,
		Protocol:        botfleet.ProtocolHTTP,
		Strategy:        botfleet.StrategyBBOHeavy,
		Bots:            8,
		Duration:        1 * time.Second,
		Timeout:         2 * time.Second,
		Method:          http.MethodPost,
		TelemetryClient: redisClient,
		SubmissionID:    submissionID,
	})
	if err != nil {
		t.Fatalf("botfleet.Run: %v", err)
	}
	t.Logf("Bot fleet summary: Successes=%d, Failures=%d, TPS=%.0f", summary.Successes, summary.Failures, summary.RequestsPerSecond)

	// Let async publishes flush
	time.Sleep(500 * time.Millisecond)

	// 5. Consume telemetry events
	events, err := telemetry.ConsumeAllForSubmission(ctx, redisClient, submissionID)
	if err != nil {
		t.Fatalf("ConsumeAllForSubmission: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected >0 telemetry events, got 0")
	}
	t.Logf("Telemetry events consumed: %d", len(events))

	// 6. Compute metrics
	perfMetrics := scorer.ComputeMetrics(submissionID, events)
	if perfMetrics.TotalRequests == 0 {
		t.Fatal("TotalRequests should be >0")
	}
	if perfMetrics.Successes == 0 {
		t.Fatal("Successes should be >0")
	}
	if perfMetrics.TPS <= 0 {
		t.Fatalf("TPS should be >0, got %.2f", perfMetrics.TPS)
	}
	if perfMetrics.P99LatencyMs <= 0 {
		t.Fatalf("P99 should be >0, got %.2f", perfMetrics.P99LatencyMs)
	}
	t.Logf("Metrics: TotalReq=%d Successes=%d TPS=%.0f P99=%.2fms",
		perfMetrics.TotalRequests, perfMetrics.Successes, perfMetrics.TPS, perfMetrics.P99LatencyMs)

	// 7. Validate orderbook
	valResult := validator.RunValidatorFromEvents(submissionID, events)
	if !valResult.Valid {
		t.Errorf("expected Valid=true, got false (CrossEvents=%d)", valResult.CrossEvents)
	}
	t.Logf("Validation: OrdersProcessed=%d CrossEvents=%d Valid=%t",
		valResult.OrdersProcessed, valResult.CrossEvents, valResult.Valid)

	// 8. Score
	sc := scorer.ComputeScore(perfMetrics, valResult)
	if sc.TotalScore <= 0 {
		t.Fatalf("TotalScore should be >0, got %.2f", sc.TotalScore)
	}
	if sc.Grade == "" {
		t.Fatal("Grade should not be empty")
	}
	t.Logf("Score: Total=%.1f Latency=%.1f Throughput=%.1f Correctness=%.1f Grade=%s",
		sc.TotalScore, sc.LatencyScore, sc.ThroughputScore, sc.CorrectnessScore, sc.Grade)

	// 9. Persist to Postgres
	sr := store.NewSubmissionResult(submissionID, "test-engine", "bbo_heavy", "go", "", "", sc, perfMetrics, valResult)
	if err := db.CreateSubmissionResult(ctx, sr); err != nil {
		t.Fatalf("CreateSubmissionResult: %v", err)
	}

	// 10. Leaderboard
	leaderboard, err := db.GetLeaderboard(ctx, 1)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(leaderboard) != 1 {
		t.Fatalf("expected 1 leaderboard entry, got %d", len(leaderboard))
	}
	entry := leaderboard[0]
	if entry.SubmissionID != submissionID {
		t.Errorf("expected SubmissionID=%q, got %q", submissionID, entry.SubmissionID)
	}
	if entry.Grade == "" {
		t.Error("leaderboard entry Grade should not be empty")
	}
	if entry.TPS <= 0 {
		t.Errorf("leaderboard entry TPS should be >0, got %.2f", entry.TPS)
	}

	// Cross-check leaderboard TPS matches computed metrics
	if entry.TPS != perfMetrics.TPS {
		t.Errorf("leaderboard TPS (%.2f) != computed TPS (%.2f)", entry.TPS, perfMetrics.TPS)
	}

	// Verify RawMetrics is valid JSON
	var rawCheck map[string]interface{}
	if err := json.Unmarshal(entry.RawMetrics, &rawCheck); err != nil {
		t.Errorf("RawMetrics is not valid JSON: %v", err)
	}

	t.Logf("✅ Full pipeline passed: %s → Grade %s, TPS=%.0f, Score=%.1f",
		submissionID, entry.Grade, entry.TPS, entry.TotalScore)
}
