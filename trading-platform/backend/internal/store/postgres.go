package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides access to the PostgreSQL database.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore connects to PostgreSQL, runs migrations, and returns a ready Store.
func NewStore(ctx context.Context, databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: parse config failed: %w", err)
	}
	// Configure for high concurrency benchmarking environment
	config.MaxConns = 50
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("store: connect failed: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping failed: %w", err)
	}
	if _, err := pool.Exec(ctx, MigrationSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: migration failed: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close shuts down the connection pool.
func (s *Store) Close() { s.pool.Close() }

// CreateSubmissionResult inserts or upserts a submission result row.
func (s *Store) CreateSubmissionResult(ctx context.Context, r SubmissionResult) error {
	const q = `
INSERT INTO submission_results (
	submission_id, system_name, strategy, language, submitted_at, total_score, latency_score,
	throughput_score, correctness_score, grade, p99_latency_ms, tps,
	cross_events, orders_processed, raw_metrics, raw_validation
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::jsonb,$16::jsonb)
ON CONFLICT (submission_id) DO UPDATE SET
	system_name = EXCLUDED.system_name,
	strategy = EXCLUDED.strategy,
	total_score = EXCLUDED.total_score,
	latency_score = EXCLUDED.latency_score,
	throughput_score = EXCLUDED.throughput_score,
	correctness_score = EXCLUDED.correctness_score,
	grade = EXCLUDED.grade,
	p99_latency_ms = EXCLUDED.p99_latency_ms,
	tps = EXCLUDED.tps,
	cross_events = EXCLUDED.cross_events,
	orders_processed = EXCLUDED.orders_processed,
	raw_metrics = EXCLUDED.raw_metrics,
	raw_validation = EXCLUDED.raw_validation
`
	_, err := s.pool.Exec(ctx, q,
		r.SubmissionID, r.SystemName, r.Strategy, r.Language, r.SubmittedAt,
		r.TotalScore, r.LatencyScore, r.ThroughputScore, r.CorrectnessScore,
		r.Grade, r.P99LatencyMs, r.TPS,
		r.CrossEvents, r.OrdersProcessed,
		string(r.RawMetrics), string(r.RawValidation),
	)
	return err
}

// GetLeaderboard returns the top-N submissions ordered by total_score DESC.
func (s *Store) GetLeaderboard(ctx context.Context, limit int) ([]SubmissionResult, error) {
	const q = `
SELECT submission_id, system_name, strategy, language, submitted_at, total_score, latency_score,
       throughput_score, correctness_score, grade, p99_latency_ms, tps,
       cross_events, orders_processed, raw_metrics, raw_validation
FROM submission_results
ORDER BY total_score DESC
LIMIT $1
`
	return s.scanLeaderboard(ctx, q, limit)
}

// GetLeaderboardByStrategy returns the top-N submissions for a specific strategy.
func (s *Store) GetLeaderboardByStrategy(ctx context.Context, strategy string, limit int) ([]SubmissionResult, error) {
	const q = `
SELECT submission_id, system_name, strategy, language, submitted_at, total_score, latency_score,
       throughput_score, correctness_score, grade, p99_latency_ms, tps,
       cross_events, orders_processed, raw_metrics, raw_validation
FROM submission_results
WHERE strategy = $1
ORDER BY total_score DESC
LIMIT $2
`
	rows, err := s.pool.Query(ctx, q, strategy, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SubmissionResult
	for rows.Next() {
		var r SubmissionResult
		if err := rows.Scan(
			&r.SubmissionID, &r.SystemName, &r.Strategy, &r.Language, &r.SubmittedAt,
			&r.TotalScore, &r.LatencyScore, &r.ThroughputScore, &r.CorrectnessScore,
			&r.Grade, &r.P99LatencyMs, &r.TPS,
			&r.CrossEvents, &r.OrdersProcessed,
			&r.RawMetrics, &r.RawValidation,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// scanLeaderboard is a helper that executes a leaderboard query with a single $1 limit param.
func (s *Store) scanLeaderboard(ctx context.Context, query string, limit int) ([]SubmissionResult, error) {
	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SubmissionResult
	for rows.Next() {
		var r SubmissionResult
		if err := rows.Scan(
			&r.SubmissionID, &r.SystemName, &r.Strategy, &r.Language, &r.SubmittedAt,
			&r.TotalScore, &r.LatencyScore, &r.ThroughputScore, &r.CorrectnessScore,
			&r.Grade, &r.P99LatencyMs, &r.TPS,
			&r.CrossEvents, &r.OrdersProcessed,
			&r.RawMetrics, &r.RawValidation,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetSubmission returns a single submission by ID.
func (s *Store) GetSubmission(ctx context.Context, submissionID string) (SubmissionResult, error) {
	const q = `
SELECT submission_id, system_name, strategy, language, submitted_at, total_score, latency_score,
       throughput_score, correctness_score, grade, p99_latency_ms, tps,
       cross_events, orders_processed, raw_metrics, raw_validation
FROM submission_results
WHERE submission_id = $1
`
	var r SubmissionResult
	err := s.pool.QueryRow(ctx, q, submissionID).Scan(
		&r.SubmissionID, &r.SystemName, &r.Strategy, &r.Language, &r.SubmittedAt,
		&r.TotalScore, &r.LatencyScore, &r.ThroughputScore, &r.CorrectnessScore,
		&r.Grade, &r.P99LatencyMs, &r.TPS,
		&r.CrossEvents, &r.OrdersProcessed,
		&r.RawMetrics, &r.RawValidation,
	)
	return r, err
}
