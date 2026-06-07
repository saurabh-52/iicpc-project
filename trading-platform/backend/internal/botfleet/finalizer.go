package botfleet

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Saurabh-52/trading-platform/internal/scorer"
	"github.com/Saurabh-52/trading-platform/internal/telemetry"
	"github.com/Saurabh-52/trading-platform/internal/validator"
)

// FinalRoundDef describes a single deterministic round for post-contest evaluation.
type FinalRoundDef struct {
	Strategy Strategy
	Seed     int64
	Label    string // Human-readable description
}

// DefaultFinalRounds returns the 5 predefined final-round configurations.
// These are hardcoded, same for every contest, ensuring fair evaluation.
var DefaultFinalRounds = []FinalRoundDef{
	{Strategy: StrategyBBOHeavy, Seed: 0xF10A1001, Label: "Baseline BBO throughput"},
	{Strategy: StrategyFlashCrash, Seed: 0xF10A1002, Label: "Extreme volatility handling"},
	{Strategy: StrategyHighCancel, Seed: 0xF10A1003, Label: "Cancel storm resilience"},
	{Strategy: StrategyIceberg, Seed: 0xF10A1004, Label: "Large hidden order processing"},
	{Strategy: StrategyMomentumBurst, Seed: 0xF10A1005, Label: "Trending market correctness"},
}

// FinalSubmission describes a team's submission to be evaluated in final rounds.
type FinalSubmission struct {
	SystemName   string
	SubmissionID string
	TargetURL    string
	Protocol     Protocol
}

// FinalRoundScore holds the score for a single round of final evaluation.
type FinalRoundScore struct {
	RoundIndex   int            `json:"round_index"`
	Strategy     string         `json:"strategy"`
	Label        string         `json:"label"`
	Score        scorer.Score   `json:"score"`
	Metrics      Summary        `json:"metrics"`
}

// FinalResult holds the aggregated result for one team across all final rounds.
type FinalResult struct {
	SystemName   string            `json:"system_name"`
	RoundScores  []FinalRoundScore `json:"round_scores"`
	AverageScore float64           `json:"average_score"`
	FinalGrade   string            `json:"final_grade"`
}

// FinalRoundConfig configures the entire finalization process.
type FinalRoundConfig struct {
	ContestID       string
	Rounds          []FinalRoundDef
	BotCount        int
	RequestCount    int
	Duration        time.Duration
	Timeout         time.Duration
	ExpectReply     bool
	RedisClient     *redis.Client
}

// RunFinalRounds evaluates a single submission across all final rounds and returns
// the aggregated result. Each round is fully deterministic.
func RunFinalRounds(ctx context.Context, frc FinalRoundConfig, sub FinalSubmission) (FinalResult, error) {
	result := FinalResult{
		SystemName:  sub.SystemName,
		RoundScores: make([]FinalRoundScore, 0, len(frc.Rounds)),
	}

	if frc.BotCount <= 0 {
		frc.BotCount = 32
	}
	if frc.Duration <= 0 {
		frc.Duration = 10 * time.Second
	}
	if frc.Timeout <= 0 {
		frc.Timeout = 2 * time.Second
	}

	var totalScore float64

	for i, round := range frc.Rounds {
		log.Printf("[Finalizer] Contest %s | Team %s | Round %d/%d: %s (%s)",
			frc.ContestID, sub.SystemName, i+1, len(frc.Rounds), round.Label, round.Strategy)

		submissionID := fmt.Sprintf("final-%s-%s-r%d-%d", frc.ContestID, sub.SystemName, i+1, time.Now().UnixNano())

		roundCtx, roundCancel := context.WithTimeout(ctx, frc.Duration+frc.Timeout+5*time.Second)

		metrics, err := Run(roundCtx, Config{
			Target:          sub.TargetURL,
			Protocol:        sub.Protocol,
			Strategy:        round.Strategy,
			Bots:            frc.BotCount,
			Requests:        frc.RequestCount,
			Duration:        frc.Duration,
			Timeout:         frc.Timeout,
			Seed:            round.Seed,
			ExpectReply:     frc.ExpectReply,
			TelemetryClient: frc.RedisClient,
			SubmissionID:    submissionID,
			JudgingMode:     ModeContestFinal,
			ContestID:       frc.ContestID,
			FinalRound:      i + 1,
		})
		roundCancel()

		if err != nil {
			log.Printf("[Finalizer] Round %d failed for %s: %v", i+1, sub.SystemName, err)
			// Record a zero-score round on failure instead of aborting
			result.RoundScores = append(result.RoundScores, FinalRoundScore{
				RoundIndex: i + 1,
				Strategy:   string(round.Strategy),
				Label:      round.Label,
				Score:      scorer.Score{SubmissionID: submissionID, Grade: "F"},
			})
			continue
		}

		// Score this round
		var sc scorer.Score
		if frc.RedisClient != nil {
			time.Sleep(200 * time.Millisecond)
			scoreCtx, scoreCancel := context.WithTimeout(ctx, 10*time.Second)
			events, consumeErr := telemetry.ConsumeAllForSubmission(scoreCtx, frc.RedisClient, submissionID)
			scoreCancel()
			if consumeErr == nil && len(events) > 0 {
				perfMetrics := scorer.ComputeMetrics(submissionID, events)
				valResult := validator.RunValidatorFromEvents(submissionID, events)
				sc = scorer.ComputeScore(perfMetrics, valResult)
			}
		}

		roundScore := FinalRoundScore{
			RoundIndex: i + 1,
			Strategy:   string(round.Strategy),
			Label:      round.Label,
			Score:      sc,
			Metrics:    metrics,
		}
		result.RoundScores = append(result.RoundScores, roundScore)
		totalScore += sc.TotalScore

		log.Printf("[Finalizer] Round %d complete for %s: score=%.2f grade=%s",
			i+1, sub.SystemName, sc.TotalScore, sc.Grade)
	}

	if len(frc.Rounds) > 0 {
		result.AverageScore = totalScore / float64(len(frc.Rounds))
	}
	result.FinalGrade = scorer.AssignGrade(result.AverageScore)

	log.Printf("[Finalizer] Final result for %s: avg_score=%.2f grade=%s",
		sub.SystemName, result.AverageScore, result.FinalGrade)

	return result, nil
}
