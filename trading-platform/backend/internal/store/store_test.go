//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/Saurabh-52/trading-platform/internal/store"
)

func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")
	return "postgres://test:test@" + host + ":" + port.Port() + "/testdb?sslmode=disable"
}

func TestStoreCreateAndLeaderboard(t *testing.T) {
	url := startPostgres(t)
	ctx := context.Background()

	s, err := store.NewStore(ctx, url)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer s.Close()

	// Insert 3 submissions with different scores
	submissions := []store.SubmissionResult{
		{SubmissionID: "sub-low", Language: "python", SubmittedAt: time.Now().UTC(), TotalScore: 30, Grade: "F", RawMetrics: json.RawMessage(`{}`), RawValidation: json.RawMessage(`{}`)},
		{SubmissionID: "sub-high", Language: "cpp", SubmittedAt: time.Now().UTC(), TotalScore: 95, Grade: "S", RawMetrics: json.RawMessage(`{}`), RawValidation: json.RawMessage(`{}`)},
		{SubmissionID: "sub-mid", Language: "go", SubmittedAt: time.Now().UTC(), TotalScore: 65, Grade: "B", RawMetrics: json.RawMessage(`{}`), RawValidation: json.RawMessage(`{}`)},
	}
	for _, sub := range submissions {
		if err := s.CreateSubmissionResult(ctx, sub); err != nil {
			t.Fatalf("CreateSubmissionResult(%s): %v", sub.SubmissionID, err)
		}
	}

	// Verify leaderboard ordering
	leaderboard, err := s.GetLeaderboard(ctx, 10)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(leaderboard) != 3 {
		t.Fatalf("expected 3 results, got %d", len(leaderboard))
	}
	if leaderboard[0].SubmissionID != "sub-high" {
		t.Errorf("rank 1 should be sub-high, got %s", leaderboard[0].SubmissionID)
	}
	if leaderboard[1].SubmissionID != "sub-mid" {
		t.Errorf("rank 2 should be sub-mid, got %s", leaderboard[1].SubmissionID)
	}
	if leaderboard[2].SubmissionID != "sub-low" {
		t.Errorf("rank 3 should be sub-low, got %s", leaderboard[2].SubmissionID)
	}

	// Verify single lookup
	got, err := s.GetSubmission(ctx, "sub-high")
	if err != nil {
		t.Fatalf("GetSubmission: %v", err)
	}
	if got.Grade != "S" {
		t.Errorf("expected Grade S, got %s", got.Grade)
	}
	if got.TotalScore != 95 {
		t.Errorf("expected TotalScore 95, got %.2f", got.TotalScore)
	}
}
