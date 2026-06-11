package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUsernameTaken = errors.New("username_taken")
	ErrEmailTaken    = errors.New("email_taken")
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

// WithTx executes the given function within a database transaction.
func (s *Store) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
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
	cross_events, orders_processed, raw_metrics, raw_validation,
	judging_mode, contest_id, final_round, seed_used, user_id, source_code
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::jsonb,$16::jsonb,$17,$18,$19,$20,$21,$22)
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
	raw_validation = EXCLUDED.raw_validation,
	judging_mode = EXCLUDED.judging_mode,
	contest_id = EXCLUDED.contest_id,
	final_round = EXCLUDED.final_round,
	seed_used = EXCLUDED.seed_used,
	user_id = EXCLUDED.user_id,
	source_code = EXCLUDED.source_code
`
	var contestID interface{} = r.ContestID
	if r.ContestID == "" {
		contestID = nil
	}
	var userID interface{} = r.UserID
	if r.UserID == "" {
		userID = nil
	}
	_, err := s.pool.Exec(ctx, q,
		r.SubmissionID, r.SystemName, r.Strategy, r.Language, r.SubmittedAt,
		r.TotalScore, r.LatencyScore, r.ThroughputScore, r.CorrectnessScore,
		r.Grade, r.P99LatencyMs, r.TPS,
		r.CrossEvents, r.OrdersProcessed,
		string(r.RawMetrics), string(r.RawValidation),
		r.JudgingMode, contestID, r.FinalRound, r.SeedUsed, userID, r.SourceCode,
	)
	return err
}

// GetLeaderboard returns the top-N practice submissions ordered by total_score DESC.
// Only 'practice' mode runs are eligible for the global public leaderboard —
// contest_live, contest_final, and demo runs are excluded to prevent strategy leakage.
func (s *Store) GetLeaderboard(ctx context.Context, limit int) ([]SubmissionResult, error) {
	const q = `
SELECT * FROM (
	SELECT DISTINCT ON (COALESCE(u.username, sr.system_name)) 
		   sr.submission_id, sr.system_name, sr.strategy, sr.language, sr.submitted_at, sr.total_score, sr.latency_score,
		   sr.throughput_score, sr.correctness_score, sr.grade, sr.p99_latency_ms, sr.tps,
		   sr.cross_events, sr.orders_processed, sr.raw_metrics, sr.raw_validation,
		   sr.judging_mode, COALESCE(sr.contest_id, ''), sr.final_round, sr.seed_used,
		   COALESCE(sr.user_id, ''), COALESCE(u.username, sr.system_name)
	FROM submission_results sr
	LEFT JOIN users u ON sr.user_id = u.id
	WHERE sr.judging_mode = 'practice'
	ORDER BY COALESCE(u.username, sr.system_name), sr.total_score DESC
) sub
ORDER BY total_score DESC
LIMIT $1
`
	return s.scanLeaderboard(ctx, q, limit)
}

// GetLeaderboardByStrategy returns the top-N practice submissions for a specific strategy.
// Filtered to 'practice' mode only to prevent contest/demo leakage.
func (s *Store) GetLeaderboardByStrategy(ctx context.Context, strategy string, limit int) ([]SubmissionResult, error) {
	const q = `
SELECT * FROM (
	SELECT DISTINCT ON (COALESCE(u.username, sr.system_name)) 
		   sr.submission_id, sr.system_name, sr.strategy, sr.language, sr.submitted_at, sr.total_score, sr.latency_score,
		   sr.throughput_score, sr.correctness_score, sr.grade, sr.p99_latency_ms, sr.tps,
		   sr.cross_events, sr.orders_processed, sr.raw_metrics, sr.raw_validation,
		   sr.judging_mode, COALESCE(sr.contest_id, ''), sr.final_round, sr.seed_used,
		   COALESCE(sr.user_id, ''), COALESCE(u.username, sr.system_name)
	FROM submission_results sr
	LEFT JOIN users u ON sr.user_id = u.id
	WHERE sr.strategy = $1 AND sr.judging_mode = 'practice'
	ORDER BY COALESCE(u.username, sr.system_name), sr.total_score DESC
) sub
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
			&r.JudgingMode, &r.ContestID, &r.FinalRound, &r.SeedUsed,
			&r.UserID, &r.Username,
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
			&r.JudgingMode, &r.ContestID, &r.FinalRound, &r.SeedUsed,
			&r.UserID, &r.Username,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetSubmissionHistory returns all practice submissions for a specific system name.
func (s *Store) GetSubmissionHistory(ctx context.Context, systemName string, limit int) ([]SubmissionResult, error) {
	const q = `
SELECT submission_id, system_name, strategy, language, submitted_at, total_score, latency_score,
       throughput_score, correctness_score, grade, p99_latency_ms, tps,
       cross_events, orders_processed, raw_metrics, raw_validation,
       judging_mode, COALESCE(contest_id, ''), final_round, seed_used
FROM submission_results
WHERE system_name = $1 AND judging_mode = 'practice'
ORDER BY submitted_at DESC
LIMIT $2
`
	rows, err := s.pool.Query(ctx, q, systemName, limit)
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
			&r.JudgingMode, &r.ContestID, &r.FinalRound, &r.SeedUsed,
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
       cross_events, orders_processed, raw_metrics, raw_validation,
       judging_mode, COALESCE(contest_id, ''), final_round, seed_used
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
		&r.JudgingMode, &r.ContestID, &r.FinalRound, &r.SeedUsed,
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
	strategy string,
	finalStrategies []string,
	problems []ProblemData,
	createdBy string,
) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if len(finalStrategies) == 0 {
		finalStrategies = []string{"bbo_heavy", "flash_crash", "high_cancel", "iceberg", "momentum_burst"}
	}

	// Insert or update contest
	const qContest = `
INSERT INTO contests (id, name, description, visibility, code, start_time, duration_minutes, registration_deadline, strategy, final_strategies, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (id) DO UPDATE SET
	name = EXCLUDED.name,
	description = EXCLUDED.description,
	visibility = EXCLUDED.visibility,
	code = EXCLUDED.code,
	start_time = EXCLUDED.start_time,
	duration_minutes = EXCLUDED.duration_minutes,
	registration_deadline = EXCLUDED.registration_deadline,
	strategy = EXCLUDED.strategy,
	final_strategies = EXCLUDED.final_strategies,
	created_by = EXCLUDED.created_by
`
	_, err = tx.Exec(ctx, qContest, contestID, name, description, visibility, code, startTime, durationMinutes, registrationDeadline, strategy, finalStrategies, createdBy)
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
	Strategy             string     `json:"strategy"`
	FinalStrategies      []string   `json:"finalStrategies"`
	CreatedBy            string     `json:"createdBy"`
	Phase                string     `json:"phase"`
}

// GetContests retrieves all contests from the database.
// The code field is intentionally excluded from the list query to avoid leaking private contest codes.
func (s *Store) GetContests(ctx context.Context) ([]Contest, error) {
	const q = `
SELECT c.id, c.name, c.description, c.visibility, c.start_time, c.duration_minutes, c.registration_deadline, c.created_at, c.strategy, c.final_strategies, c.created_by, c.phase,
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
			&c.ID, &c.Name, &c.Description, &c.Visibility, &c.StartTime, &c.DurationMinutes, &c.RegistrationDeadline, &c.CreatedAt, &c.Strategy, &c.FinalStrategies, &c.CreatedBy, &c.Phase, &c.Participants,
		); err != nil {
			return nil, err
		}
		// Don't expose the code in the listing
		c.Code = ""
		results = append(results, c)
	}
	return results, rows.Err()
}

// RegisterSystemForContest registers a system/team for a contest, linked to a user account.
func (s *Store) RegisterSystemForContest(ctx context.Context, contestID string, systemName string, code string, userID string) error {
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

	// Insert registration with user linkage
	const q = `
INSERT INTO contest_registrations (contest_id, system_name, registered_at, user_id)
VALUES ($1, $2, NOW(), $3)
ON CONFLICT (contest_id, system_name) DO UPDATE SET user_id = EXCLUDED.user_id
`
	_, err = s.pool.Exec(ctx, q, contestID, systemName, userID)
	return err
}

// SaveFinalScore upserts a team's averaged final score for a contest.
func (s *Store) SaveFinalScore(ctx context.Context, contestID, systemName string, avgScore float64, roundScoresJSON string, finalGrade string) error {
	const q = `
INSERT INTO contest_final_scores (contest_id, system_name, avg_score, round_scores, final_grade, finalized_at)
VALUES ($1, $2, $3, $4::jsonb, $5, NOW())
ON CONFLICT (contest_id, system_name) DO UPDATE SET
	avg_score = EXCLUDED.avg_score,
	round_scores = EXCLUDED.round_scores,
	final_grade = EXCLUDED.final_grade,
	finalized_at = EXCLUDED.finalized_at
`
	_, err := s.pool.Exec(ctx, q, contestID, systemName, avgScore, roundScoresJSON, finalGrade)
	return err
}

// BestLiveScore represents the best contest_live submission for a team in a contest.
type BestLiveScore struct {
	SystemName       string
	TotalScore       float64
	LatencyScore     float64
	ThroughputScore  float64
	CorrectnessScore float64
	P99LatencyMs     float64
	TPS              float64
	Grade            string
	Strategy         string
}

// GetBestLiveScoresForContest returns the single best-scoring contest_live submission for each team.
func (s *Store) GetBestLiveScoresForContest(ctx context.Context, contestID string) (map[string]BestLiveScore, error) {
	const q = `
SELECT DISTINCT ON (system_name)
	system_name, total_score, latency_score, throughput_score, correctness_score,
	p99_latency_ms, tps, grade, strategy
FROM submission_results
WHERE contest_id = $1 AND judging_mode = 'contest_live' AND total_score > 0
ORDER BY system_name, total_score DESC
`
	rows, err := s.pool.Query(ctx, q, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]BestLiveScore)
	for rows.Next() {
		var b BestLiveScore
		if err := rows.Scan(&b.SystemName, &b.TotalScore, &b.LatencyScore, &b.ThroughputScore,
			&b.CorrectnessScore, &b.P99LatencyMs, &b.TPS, &b.Grade, &b.Strategy); err != nil {
			return nil, err
		}
		results[b.SystemName] = b
	}
	return results, rows.Err()
}

// GetContestLeaderboard returns contest-specific leaderboard.
// If the contest is completed (has final scores), it returns final scores.
// Otherwise it returns live submission scores for that contest.
func (s *Store) GetContestLeaderboard(ctx context.Context, contestID string, limit int) ([]ContestFinalScore, []SubmissionResult, error) {
	// Check if final scores exist
	var finalCount int
	_ = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM contest_final_scores WHERE contest_id = $1", contestID).Scan(&finalCount)

	if finalCount > 0 {
		finals, err := s.GetContestFinalScores(ctx, contestID, limit)
		return finals, nil, err
	}

	// Fall back to live submission results for this contest, fetching only the BEST submission per system
	const q = `
WITH LatestSubmissions AS (
    SELECT DISTINCT ON (COALESCE(u.username, sr.system_name))
           sr.submission_id, sr.system_name, sr.strategy, sr.language, sr.submitted_at, sr.total_score, sr.latency_score,
           sr.throughput_score, sr.correctness_score, sr.grade, sr.p99_latency_ms, sr.tps,
           sr.cross_events, sr.orders_processed, sr.raw_metrics, sr.raw_validation,
           sr.judging_mode, COALESCE(sr.contest_id, '') AS c_id, sr.final_round, sr.seed_used,
           sr.user_id, u.username
    FROM submission_results sr
    LEFT JOIN users u ON sr.user_id = u.id
    WHERE sr.contest_id = $1 AND sr.judging_mode = 'contest_live'
    ORDER BY COALESCE(u.username, sr.system_name), sr.total_score DESC, sr.submitted_at DESC
)
SELECT * FROM LatestSubmissions
ORDER BY total_score DESC
LIMIT $2
`
	rows, err := s.pool.Query(ctx, q, contestID, limit)
	if err != nil {
		return nil, nil, err
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
			&r.JudgingMode, &r.ContestID, &r.FinalRound, &r.SeedUsed,
			&r.UserID, &r.Username,
		); err != nil {
			return nil, nil, err
		}
		results = append(results, r)
	}
	return nil, results, rows.Err()
}

// UpdateContestPhase transitions a contest to a new phase.
func (s *Store) UpdateContestPhase(ctx context.Context, contestID string, phase string) error {
	const q = `UPDATE contests SET phase = $1 WHERE id = $2`
	_, err := s.pool.Exec(ctx, q, phase, contestID)
	return err
}

// FinalScoreInput holds the data for a team's final score.
type FinalScoreInput struct {
	SystemName     string
	AvgScore       float64
	AvgLatency     float64
	AvgThroughput  float64
	AvgCorrectness float64
	AvgP99         float64
	AvgTPS         float64
	RoundScores    string
	FinalGrade     string
}

// SaveFinalScoresAndCompleteContest saves all final scores and marks the contest as completed atomically.
func (s *Store) SaveFinalScoresAndCompleteContest(ctx context.Context, contestID string, scores []FinalScoreInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const scoreQuery = `
INSERT INTO contest_final_scores (contest_id, system_name, avg_score, avg_latency, avg_throughput, avg_correctness, avg_p99, avg_tps, round_scores, final_grade)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (contest_id, system_name) DO UPDATE SET
	avg_score = EXCLUDED.avg_score,
	avg_latency = EXCLUDED.avg_latency,
	avg_throughput = EXCLUDED.avg_throughput,
	avg_correctness = EXCLUDED.avg_correctness,
	avg_p99 = EXCLUDED.avg_p99,
	avg_tps = EXCLUDED.avg_tps,
	round_scores = EXCLUDED.round_scores,
	final_grade = EXCLUDED.final_grade,
	finalized_at = NOW()
`
	for _, sc := range scores {
		if _, err := tx.Exec(ctx, scoreQuery, contestID, sc.SystemName, sc.AvgScore, sc.AvgLatency, sc.AvgThroughput, sc.AvgCorrectness, sc.AvgP99, sc.AvgTPS, sc.RoundScores, sc.FinalGrade); err != nil {
			return err
		}
	}

	const phaseQuery = `UPDATE contests SET phase = 'completed' WHERE id = $1`
	if _, err := tx.Exec(ctx, phaseQuery, contestID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetContestRegistrations returns all registered system names for a contest.
func (s *Store) GetContestRegistrations(ctx context.Context, contestID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT system_name FROM contest_registrations WHERE contest_id = $1", contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// GetUnregisteredParticipants returns unique system names that submitted code but didn't register.
func (s *Store) GetUnregisteredParticipants(ctx context.Context, contestID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT DISTINCT system_name FROM submission_results WHERE contest_id = $1", contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// GetContestFinalScores returns the final averaged scores for a contest, ranked by avg_score DESC.
func (s *Store) GetContestFinalScores(ctx context.Context, contestID string, limit int) ([]ContestFinalScore, error) {
	const q = `
SELECT contest_id, system_name, avg_score, avg_latency, avg_throughput, avg_correctness, avg_p99, avg_tps, round_scores, final_grade, finalized_at
FROM contest_final_scores
WHERE contest_id = $1
ORDER BY avg_score DESC
LIMIT $2
`
	rows, err := s.pool.Query(ctx, q, contestID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ContestFinalScore
	for rows.Next() {
		var r ContestFinalScore
		if err := rows.Scan(&r.ContestID, &r.SystemName, &r.AvgScore, &r.AvgLatency, &r.AvgThroughput, &r.AvgCorrectness, &r.AvgP99, &r.AvgTPS, &r.RoundScores, &r.FinalGrade, &r.FinalizedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetContest returns a single contest by ID.
func (s *Store) GetContest(ctx context.Context, contestID string) (Contest, error) {
	const q = `
SELECT c.id, c.name, c.description, c.visibility, c.code, c.start_time, c.duration_minutes, c.registration_deadline, c.created_at, c.strategy, c.final_strategies, c.created_by, c.phase,
       (SELECT COUNT(*)::int FROM contest_registrations r WHERE r.contest_id = c.id) AS participants
FROM contests c
WHERE c.id = $1
`
	var c Contest
	err := s.pool.QueryRow(ctx, q, contestID).Scan(
		&c.ID, &c.Name, &c.Description, &c.Visibility, &c.Code, &c.StartTime, &c.DurationMinutes, &c.RegistrationDeadline, &c.CreatedAt, &c.Strategy, &c.FinalStrategies, &c.CreatedBy, &c.Phase, &c.Participants,
	)
	return c, err
}

// GetContestProblems returns all problems for a given contest.
func (s *Store) GetContestProblems(ctx context.Context, contestID string) ([]ProblemData, error) {
	const q = `
SELECT id, code, title, statement, time_limit, memory_limit,
       sample_strategies, sample_bot_files, sample_show_custom, sample_target_injection, sample_protocol, sample_telemetry_format,
       hidden_strategies, hidden_bot_files, hidden_show_custom, hidden_target_injection, hidden_protocol, hidden_telemetry_format
FROM problems
WHERE contest_id = $1
ORDER BY sequence ASC
`
	rows, err := s.pool.Query(ctx, q, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var problems []ProblemData
	for rows.Next() {
		var p ProblemData
		if err := rows.Scan(
			&p.ID, &p.Code, &p.Title, &p.Statement, &p.TimeLimit, &p.MemoryLimit,
			&p.SampleStrategies, &p.SampleBotFilesJSON, &p.SampleShowCustom, &p.SampleTargetInjection, &p.SampleProtocol, &p.SampleTelemetryFormat,
			&p.HiddenStrategies, &p.HiddenBotFilesJSON, &p.HiddenShowCustom, &p.HiddenTargetInjection, &p.HiddenProtocol, &p.HiddenTelemetryFormat,
		); err != nil {
			return nil, err
		}
		problems = append(problems, p)
	}
	return problems, rows.Err()
}

// DeleteContest deletes a contest and all associated data (via CASCADE).
func (s *Store) DeleteContest(ctx context.Context, contestID string) error {
	const q = `DELETE FROM contests WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, contestID)
	return err
}

// GetUserRegistrations returns the list of contest IDs that a user is registered for.
func (s *Store) GetUserRegistrations(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, "SELECT contest_id FROM contest_registrations WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ─── User Methods ─────────────────────────────────────────────────────────────

// CreateUser inserts a new user into the database.
// It returns ErrUsernameTaken or ErrEmailTaken if constraints are violated.
func (s *Store) CreateUser(ctx context.Context, id, username, email, passwordHash string) error {
	const q = `
INSERT INTO users (id, username, email, password_hash)
VALUES ($1, $2, $3, $4)
`
	_, err := s.pool.Exec(ctx, q, id, username, email, passwordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "username") {
				return ErrUsernameTaken
			}
			if strings.Contains(pgErr.ConstraintName, "email") {
				return ErrEmailTaken
			}
		}
	}
	return err
}

// GetUserByUsername retrieves a user by their username.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, error) {
	const q = `SELECT id, username, email, password_hash, created_at FROM users WHERE username = $1`
	var u User
	err := s.pool.QueryRow(ctx, q, username).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

// GetUserByEmail retrieves a user by their email.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	const q = `SELECT id, username, email, password_hash, created_at FROM users WHERE email = $1`
	var u User
	err := s.pool.QueryRow(ctx, q, email).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

// GetUserByID retrieves a user by their ID.
func (s *Store) GetUserByID(ctx context.Context, id string) (User, error) {
	const q = `SELECT id, username, email, password_hash, created_at FROM users WHERE id = $1`
	var u User
	err := s.pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

// GetUserHistory returns all submissions for a specific user ID, most recent first.
func (s *Store) GetUserHistory(ctx context.Context, userID string, limit int) ([]SubmissionResult, error) {
	const q = `
SELECT submission_id, system_name, strategy, language, submitted_at, total_score, latency_score,
       throughput_score, correctness_score, grade, p99_latency_ms, tps,
       cross_events, orders_processed, raw_metrics, raw_validation,
       judging_mode, COALESCE(contest_id, ''), final_round, seed_used
FROM submission_results
WHERE user_id = $1
ORDER BY submitted_at DESC
LIMIT $2
`
	rows, err := s.pool.Query(ctx, q, userID, limit)
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
			&r.JudgingMode, &r.ContestID, &r.FinalRound, &r.SeedUsed,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// UsernameExists checks if a username is already taken.
func (s *Store) UsernameExists(ctx context.Context, username string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	var exists bool
	err := s.pool.QueryRow(ctx, q, username).Scan(&exists)
	return exists, err
}

// GetBestScore returns the highest scoring 'practice' submission for a user.
func (s *Store) GetBestScore(ctx context.Context, userID string) (*SubmissionResult, error) {
	const q = `
SELECT submission_id, system_name, strategy, language, submitted_at, total_score, latency_score,
       throughput_score, correctness_score, grade, p99_latency_ms, tps,
       cross_events, orders_processed, raw_metrics, raw_validation,
       judging_mode, COALESCE(contest_id, ''), final_round, seed_used
FROM submission_results
WHERE user_id = $1 AND judging_mode = 'practice'
ORDER BY total_score DESC
LIMIT 1
`
	var r SubmissionResult
	err := s.pool.QueryRow(ctx, q, userID).Scan(
		&r.SubmissionID, &r.SystemName, &r.Strategy, &r.Language, &r.SubmittedAt,
		&r.TotalScore, &r.LatencyScore, &r.ThroughputScore, &r.CorrectnessScore,
		&r.Grade, &r.P99LatencyMs, &r.TPS,
		&r.CrossEvents, &r.OrdersProcessed,
		&r.RawMetrics, &r.RawValidation,
		&r.JudgingMode, &r.ContestID, &r.FinalRound, &r.SeedUsed,
	)
	if err != nil {
		// pgx returns pgx.ErrNoRows if nothing matches, which is useful to distinguish
		return nil, err
	}
	return &r, nil
}

// GetPaginatedUserHistory returns a paginated list of practice submissions for a specific user ID.
func (s *Store) GetPaginatedUserHistory(ctx context.Context, userID string, page, pageSize int) ([]SubmissionResult, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	// Get total count
	var total int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM submission_results WHERE user_id = $1 AND judging_mode = 'practice'", userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	const q = `
SELECT submission_id, system_name, strategy, language, submitted_at, total_score, latency_score,
       throughput_score, correctness_score, grade, p99_latency_ms, tps,
       cross_events, orders_processed, raw_metrics, raw_validation,
       judging_mode, COALESCE(contest_id, ''), final_round, seed_used
FROM submission_results
WHERE user_id = $1 AND judging_mode = 'practice'
ORDER BY submitted_at DESC
LIMIT $2 OFFSET $3
`
	rows, err := s.pool.Query(ctx, q, userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
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
			&r.JudgingMode, &r.ContestID, &r.FinalRound, &r.SeedUsed,
		); err != nil {
			return nil, 0, err
		}
		results = append(results, r)
	}
	return results, total, rows.Err()
}

