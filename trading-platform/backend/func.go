//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"trading-platform/internal/botfleet"
	"trading-platform/internal/sandbox"
	"trading-platform/internal/scorer"
	"trading-platform/internal/store"
	"trading-platform/internal/telemetry"
	"trading-platform/internal/validator"
	"trading-platform/internal/ws"
)

func runFinalization(ctx context.Context, contestID string, contest store.Contest, teams []string, db *store.Store, hub *ws.Hub, redisClient *redis.Client) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in finalization goroutine: %v", r)
		}
	}()

	var finalScores []store.FinalScoreInput
	totalTeams := len(teams)

	problems, _ := db.GetContestProblems(ctx, contestID)
	var hiddenStrategies []string
	if len(problems) > 0 {
		for _, p := range problems {
			hiddenStrategies = append(hiddenStrategies, p.HiddenStrategies...)
		}
	}
	if len(hiddenStrategies) == 0 {
		hiddenStrategies = contest.FinalStrategies
	}

	bestLiveScores, err := db.GetBestLiveScoresForContest(ctx, contestID)
	if err != nil {
		log.Printf("[Finalize] WARNING: could not fetch best live scores: %v", err)
		bestLiveScores = make(map[string]store.BestLiveScore)
	}

	for idx, teamName := range teams {
		var roundResults []map[string]interface{}
		var allScores, allLat, allThr, allCor, allP99, allTPS []float64

		if best, ok := bestLiveScores[teamName]; ok {
			allScores = append(allScores, best.TotalScore)
			allLat = append(allLat, best.LatencyScore)
			allThr = append(allThr, best.ThroughputScore)
			allCor = append(allCor, best.CorrectnessScore)
			allP99 = append(allP99, best.P99LatencyMs)
			allTPS = append(allTPS, best.TPS)
			roundResults = append(roundResults, map[string]interface{}{
				"label": fmt.Sprintf("Best Live (%s)", best.Strategy),
				"score": best.TotalScore,
				"grade": best.Grade,
			})
		}

		targetURL, err := sandbox.GetSandboxTargetURL(ctx, teamName)
		if err != nil {
			roundResults = append(roundResults, map[string]interface{}{
				"label": "Hidden Strategies",
				"score": 0.0,
				"error": "sandbox not running — scored from live submissions only",
			})
		} else {
			for i, stratName := range hiddenStrategies {
				strategy := botfleet.NormalizeStrategy(stratName)
				submissionID := fmt.Sprintf("finalize-%s-%d-%d", contestID, time.Now().UnixNano(), i)
				seed := botfleet.DeterministicSeedForStrategy(strategy)

				runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				metrics, runErr := botfleet.Run(runCtx, botfleet.Config{
					Target: targetURL, Protocol: "http", Strategy: strategy, Bots: 32,
					Requests: 0, Duration: 10 * time.Second, Timeout: 2 * time.Second,
					Method: "POST", Path: "/", ExpectReply: true, RampUpDuration: 0,
					TelemetryClient: nil, SubmissionID: submissionID,
					JudgingMode: "contest_final", ContestID: contestID, Seed: seed,
				})
				cancel()
				time.Sleep(1 * time.Second)

				roundScore, roundLat, roundThr, roundCor, roundP99, roundTPS := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0
				roundGrade := "F"

				if runErr == nil && metrics.Successes > 0 {
					perfMetrics := scorer.PerformanceMetrics{
						SubmissionID: submissionID, TotalRequests: metrics.Requests, Successes: metrics.Successes, Failures: metrics.Failures,
						TPS: metrics.RequestsPerSecond, MinLatencyMs: metrics.MinLatencyMs, AvgLatencyMs: metrics.AverageLatencyMs,
						P50LatencyMs: metrics.P50LatencyMs, P90LatencyMs: metrics.P90LatencyMs, P99LatencyMs: metrics.P99LatencyMs,
						MaxLatencyMs: metrics.MaxLatencyMs, StdDevMs: metrics.StdDevMs,
					}
					valResult := validator.ValidationResult{}
					sc := scorer.ComputeScore(perfMetrics, valResult)
					roundScore, roundLat, roundThr, roundCor, roundGrade, roundP99, roundTPS = sc.TotalScore, sc.LatencyScore, sc.ThroughputScore, sc.CorrectnessScore, sc.Grade, perfMetrics.P99LatencyMs, perfMetrics.TPS
				}

				if roundScore > 0 {
					allScores = append(allScores, roundScore)
					allLat = append(allLat, roundLat)
					allThr = append(allThr, roundThr)
					allCor = append(allCor, roundCor)
					allP99 = append(allP99, roundP99)
					allTPS = append(allTPS, roundTPS)
				}
				roundResults = append(roundResults, map[string]interface{}{"label": stratName, "score": roundScore, "grade": roundGrade})
			}
		}

		avgScore, avgLat, avgThr, avgCor, avgP99, avgTPS := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0
		finalGrade := "N/A"
		if len(allScores) > 0 {
			for _, v := range allScores { avgScore += v }
			for _, v := range allLat { avgLat += v }
			for _, v := range allThr { avgThr += v }
			for _, v := range allCor { avgCor += v }
			for _, v := range allP99 { avgP99 += v }
			for _, v := range allTPS { avgTPS += v }
			n := float64(len(allScores))
			avgScore /= n
			avgLat /= n
			avgThr /= n
			avgCor /= n
			avgP99 /= n
			avgTPS /= n
			if avgScore >= 95 { finalGrade = "A+" } else if avgScore >= 90 { finalGrade = "A" } else if avgScore >= 80 { finalGrade = "B" } else if avgScore >= 70 { finalGrade = "C" } else if avgScore >= 60 { finalGrade = "D" } else { finalGrade = "F" }
		}

		mockRoundsJSONBytes, _ := json.Marshal(roundResults)
		finalScores = append(finalScores, store.FinalScoreInput{
			SystemName: teamName, AvgScore: avgScore, AvgLatency: avgLat, AvgThroughput: avgThr,
			AvgCorrectness: avgCor, AvgP99: avgP99, AvgTPS: avgTPS, RoundScores: string(mockRoundsJSONBytes), FinalGrade: finalGrade,
		})

		hub.Broadcast(ws.FinalizationProgressUpdate{Type: "finalization_progress", Contest: contestID, Progress: int(float64(idx+1) / float64(totalTeams) * 100)})
	}

	if err := db.SaveFinalScoresAndCompleteContest(ctx, contestID, finalScores); err != nil { log.Printf("ERROR: %v", err) }
	if err := sandbox.CleanupContestSandboxes(ctx, contestID); err != nil { log.Printf("WARNING: %v", err) }
	if redisClient != nil { _ = telemetry.TrimStream(ctx, redisClient) }
	if finalScoresList, err := db.GetContestFinalScores(ctx, contestID, 50); err == nil {
		hub.Broadcast(ws.LeaderboardUpdate{Type: "contest_finalized", Payload: finalScoresList})
	}
}
