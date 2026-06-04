package store

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

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

	// Seed mock contests if empty
	if err := seedMockContests(ctx, pool); err != nil {
		log.Printf("WARNING: failed to seed mock contests: %v", err)
	}

	return &Store{pool: pool}, nil
}

func seedMockContests(ctx context.Context, pool *pgxpool.Pool) error {
	log.Println("Checking and seeding default mock contests and registrations into PostgreSQL...")
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	contests := []struct {
		id          string
		name        string
		description string
		visibility  string
		code        string
		offsetDays  int
		duration    int
	}{
		{"c1", "IICPC Qualifier Round", "Qualify for the main event — top 50 teams advance.", "public", "", 3, 120},
		{"c2", "Market Maker Challenge", "Build the most profitable market-making engine under adversarial conditions.", "public", "", 7, 90},
		{"c3", "Flash Crash Stress Test", "Survive 60 minutes of extreme volatility and flash crash scenarios.", "private", "FLASH", 14, 60},
		{"p1", "IICPC Practice Round #3", "Community practice round with 4 problems.", "public", "", -5, 90},
		{"p2", "Spread Optimization Sprint", "Optimize bid-ask spread in a simulated order book.", "public", "", -12, 60},
	}

	for _, c := range contests {
		startTime := time.Now().AddDate(0, 0, c.offsetDays)
		const q = `
INSERT INTO contests (id, name, description, visibility, code, start_time, duration_minutes)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO NOTHING
`
		if _, err := tx.Exec(ctx, q, c.id, c.name, c.description, c.visibility, c.code, startTime, c.duration); err != nil {
			return fmt.Errorf("failed to seed contest %s: %w", c.id, err)
		}
	}

	mockRegs := []struct {
		contestID string
		count     int
	}{
		{"c1", 184},
		{"c2", 67},
		{"c3", 42},
		{"p1", 213},
		{"p2", 98},
	}

	for _, mr := range mockRegs {
		var regCount int
		err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM contest_registrations WHERE contest_id = $1", mr.contestID).Scan(&regCount)
		if err != nil {
			return err
		}
		if regCount > 0 {
			continue
		}

		for i := 0; i < mr.count; i++ {
			systemName := fmt.Sprintf("MockTeam-%d", i)
			const qReg = `
INSERT INTO contest_registrations (contest_id, system_name)
VALUES ($1, $2)
ON CONFLICT (contest_id, system_name) DO NOTHING
`
			if _, err := tx.Exec(ctx, qReg, mr.contestID, systemName); err != nil {
				return fmt.Errorf("failed to seed registration for %s: %w", mr.contestID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	log.Println("Seeding check completed successfully")
	return nil
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

// ProblemData is a db-friendly representation of problem data.
type ProblemData struct {
	ID                    string
	Code                  string
	Title                 string
	Statement             string
	TimeLimit             int
	MemoryLimit           int
	SampleStrategies      []string
	SampleBotFilesJSON    string // Marshalled JSON array of BotFile
	SampleShowCustom      bool
	SampleTargetInjection string
	SampleProtocol        string
	SampleTelemetryFormat string
	HiddenStrategies      []string
	HiddenBotFilesJSON    string // Marshalled JSON array of BotFile
	HiddenShowCustom      bool
	HiddenTargetInjection string
	HiddenProtocol        string
	HiddenTelemetryFormat string
}

// SaveContestDraft saves or updates the current contest draft.
func (s *Store) SaveContestDraft(ctx context.Context, details []byte, problems []byte) error {
	const q = `
INSERT INTO contest_drafts (id, details, problems, updated_at)
VALUES ('current_draft', $1, $2, NOW())
ON CONFLICT (id) DO UPDATE SET
	details = EXCLUDED.details,
	problems = EXCLUDED.problems,
	updated_at = NOW()
`
	_, err := s.pool.Exec(ctx, q, details, problems)
	return err
}

// PublishContest inserts or updates a contest and its associated problems.
func (s *Store) PublishContest(
	ctx context.Context,
	contestID string,
	name string,
	description string,
	visibility string,
	code string,
	startTime time.Time,
	durationMinutes int,
	registrationDeadline *time.Time,
	problems []ProblemData,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert or update contest
	const qContest = `
INSERT INTO contests (id, name, description, visibility, code, start_time, duration_minutes, registration_deadline)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
	name = EXCLUDED.name,
	description = EXCLUDED.description,
	visibility = EXCLUDED.visibility,
	code = EXCLUDED.code,
	start_time = EXCLUDED.start_time,
	duration_minutes = EXCLUDED.duration_minutes,
	registration_deadline = EXCLUDED.registration_deadline
`
	_, err = tx.Exec(ctx, qContest, contestID, name, description, visibility, code, startTime, durationMinutes, registrationDeadline)
	if err != nil {
		return fmt.Errorf("publish_contest: failed to insert contest: %w", err)
	}

	// Delete old problems for this contest
	_, err = tx.Exec(ctx, "DELETE FROM problems WHERE contest_id = $1", contestID)
	if err != nil {
		return fmt.Errorf("publish_contest: failed to delete old problems: %w", err)
	}

	// Insert new problems
	const qProblem = `
INSERT INTO problems (
	id, contest_id, code, title, statement, time_limit, memory_limit,
	sample_strategies, sample_bot_files, sample_show_custom, sample_target_injection, sample_protocol, sample_telemetry_format,
	hidden_strategies, hidden_bot_files, hidden_show_custom, hidden_target_injection, hidden_protocol, hidden_telemetry_format,
	sequence
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
`
	for idx, p := range problems {
		var sampleBotFilesJSON interface{} = p.SampleBotFilesJSON
		if p.SampleBotFilesJSON == "" {
			sampleBotFilesJSON = "[]"
		}
		var hiddenBotFilesJSON interface{} = p.HiddenBotFilesJSON
		if p.HiddenBotFilesJSON == "" {
			hiddenBotFilesJSON = "[]"
		}
		_, err = tx.Exec(ctx, qProblem,
			p.ID, contestID, p.Code, p.Title, p.Statement, p.TimeLimit, p.MemoryLimit,
			p.SampleStrategies, sampleBotFilesJSON, p.SampleShowCustom, p.SampleTargetInjection, p.SampleProtocol, p.SampleTelemetryFormat,
			p.HiddenStrategies, hiddenBotFilesJSON, p.HiddenShowCustom, p.HiddenTargetInjection, p.HiddenProtocol, p.HiddenTelemetryFormat,
			idx,
		)
		if err != nil {
			return fmt.Errorf("publish_contest: failed to insert problem %s: %w", p.ID, err)
		}
	}

	return tx.Commit(ctx)
}

// Contest represents a row in the contests table.
type Contest struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	Description          string     `json:"description"`
	Visibility           string     `json:"visibility"`
	Code                 string     `json:"code"`
	StartTime            time.Time  `json:"startTime"`
	DurationMinutes      int        `json:"durationMinutes"`
	RegistrationDeadline *time.Time `json:"registrationDeadline"`
	CreatedAt            time.Time  `json:"createdAt"`
	Participants         int        `json:"participants"`
}

// GetContests retrieves all contests from the database.
func (s *Store) GetContests(ctx context.Context) ([]Contest, error) {
	const q = `
SELECT c.id, c.name, c.description, c.visibility, c.code, c.start_time, c.duration_minutes, c.registration_deadline, c.created_at,
       (SELECT COUNT(*)::int FROM contest_registrations r WHERE r.contest_id = c.id) AS participants
FROM contests c
ORDER BY c.start_time ASC
`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Contest
	for rows.Next() {
		var c Contest
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Description, &c.Visibility, &c.Code, &c.StartTime, &c.DurationMinutes, &c.RegistrationDeadline, &c.CreatedAt, &c.Participants,
		); err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// RegisterSystemForContest registers a system/team for a contest.
func (s *Store) RegisterSystemForContest(ctx context.Context, contestID string, systemName string, code string) error {
	// First, check if the contest exists and is private, if so, verify the code
	var visibility, expectedCode string
	err := s.pool.QueryRow(ctx, "SELECT visibility, code FROM contests WHERE id = $1", contestID).Scan(&visibility, &expectedCode)
	if err != nil {
		return fmt.Errorf("contest not found: %w", err)
	}

	if visibility == "private" {
		if strings.TrimSpace(code) == "" {
			return fmt.Errorf("contest code is required for private contests")
		}
		if code != expectedCode {
			return fmt.Errorf("invalid contest code")
		}
	}

	// Insert registration
	const q = `
INSERT INTO contest_registrations (contest_id, system_name, registered_at)
VALUES ($1, $2, NOW())
ON CONFLICT (contest_id, system_name) DO NOTHING
`
	_, err = s.pool.Exec(ctx, q, contestID, systemName)
	return err
}


