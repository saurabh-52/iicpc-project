package store

import (
	"encoding/json"
	"time"

	"github.com/Saurabh-52/trading-platform/internal/scorer"
	"github.com/Saurabh-52/trading-platform/internal/validator"
)

// SubmissionResult is the full persisted record of a benchmarked submission.
type SubmissionResult struct {
	SubmissionID     string          `json:"submission_id"`
	SystemName       string          `json:"system_name"`
	Strategy         string          `json:"strategy"`
	Language         string          `json:"language"`
	SubmittedAt      time.Time       `json:"submitted_at"`
	TotalScore       float64         `json:"total_score"`
	LatencyScore     float64         `json:"latency_score"`
	ThroughputScore  float64         `json:"throughput_score"`
	CorrectnessScore float64         `json:"correctness_score"`
	Grade            string          `json:"grade"`
	P99LatencyMs     float64         `json:"p99_latency_ms"`
	TPS              float64         `json:"tps"`
	CrossEvents      int             `json:"cross_events"`
	OrdersProcessed  int             `json:"orders_processed"`
	RawMetrics       json.RawMessage `json:"raw_metrics"`
	RawValidation    json.RawMessage `json:"raw_validation"`
}

// MigrationSQL is executed on first connection to create the submissions table.
const MigrationSQL = `
CREATE TABLE IF NOT EXISTS submission_results (
	submission_id     TEXT PRIMARY KEY,
	system_name       TEXT NOT NULL DEFAULT '',
	strategy          TEXT NOT NULL DEFAULT 'bbo_heavy',
	language          TEXT NOT NULL DEFAULT '',
	submitted_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	total_score       FLOAT8 NOT NULL DEFAULT 0,
	latency_score     FLOAT8 NOT NULL DEFAULT 0,
	throughput_score  FLOAT8 NOT NULL DEFAULT 0,
	correctness_score FLOAT8 NOT NULL DEFAULT 0,
	grade             TEXT NOT NULL DEFAULT 'F',
	p99_latency_ms    FLOAT8 NOT NULL DEFAULT 0,
	tps               FLOAT8 NOT NULL DEFAULT 0,
	cross_events      INT NOT NULL DEFAULT 0,
	orders_processed  INT NOT NULL DEFAULT 0,
	raw_metrics       JSONB,
	raw_validation    JSONB
);

-- Add new columns for existing tables (safe to re-run)
ALTER TABLE submission_results ADD COLUMN IF NOT EXISTS system_name TEXT NOT NULL DEFAULT '';
ALTER TABLE submission_results ADD COLUMN IF NOT EXISTS strategy TEXT NOT NULL DEFAULT 'bbo_heavy';

CREATE INDEX IF NOT EXISTS idx_submission_results_total_score 
ON submission_results (total_score DESC);

CREATE INDEX IF NOT EXISTS idx_submission_results_strategy
ON submission_results (strategy, total_score DESC);
`

// NewSubmissionResult builds a SubmissionResult from scoring outputs.
func NewSubmissionResult(
	submissionID, systemName, strategy, language string,
	sc scorer.Score,
	metrics scorer.PerformanceMetrics,
	val validator.ValidationResult,
) SubmissionResult {
	rawMetrics, _ := json.Marshal(metrics)
	rawValidation, _ := json.Marshal(val)

	return SubmissionResult{
		SubmissionID:     submissionID,
		SystemName:       systemName,
		Strategy:         strategy,
		Language:         language,
		SubmittedAt:      time.Now().UTC(),
		TotalScore:       sc.TotalScore,
		LatencyScore:     sc.LatencyScore,
		ThroughputScore:  sc.ThroughputScore,
		CorrectnessScore: sc.CorrectnessScore,
		Grade:            sc.Grade,
		P99LatencyMs:     metrics.P99LatencyMs,
		TPS:              metrics.TPS,
		CrossEvents:      val.CrossEvents,
		OrdersProcessed:  val.OrdersProcessed,
		RawMetrics:       rawMetrics,
		RawValidation:    rawValidation,
	}
}
