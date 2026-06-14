//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/Saurabh-52/trading-platform/internal/botfleet"
	"github.com/Saurabh-52/trading-platform/internal/sandbox"
	"github.com/Saurabh-52/trading-platform/internal/scorer"
	"github.com/Saurabh-52/trading-platform/internal/store"
	"github.com/Saurabh-52/trading-platform/internal/telemetry"
	"github.com/Saurabh-52/trading-platform/internal/validator"
	"github.com/Saurabh-52/trading-platform/internal/ws"
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

	// Determine which strategies to use for finalization
	finalStrategies := contest.FinalStrategies
	if len(finalStrategies) == 0 {
		finalStrategies = []string{"bbo_heavy", "flash_crash", "high_cancel", "iceberg", "momentum_burst"}
	}

	bestLiveScores, err := db.GetBestLiveScoresForContest(ctx, contestID)
	if err != nil {
		log.Printf("[Finalize] WARNING: could not fetch best live scores: %v", err)
		bestLiveScores = make(map[string]store.BestLiveScore)
	}

	// runFinalStressTest runs a single strategy against a target and returns scored results.
	// It uses telemetry + validator for proper correctness scoring when redisClient is available.
	runFinalStressTest := func(targetURL, protocol, submissionID string, strategy botfleet.Strategy, seed int64) (roundScore, roundLat, roundThr, roundCor, roundP99, roundTPS float64, roundGrade string) {
		roundGrade = "F"

		runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		metrics, runErr := botfleet.Run(runCtx, botfleet.Config{
			Target: targetURL, Protocol: botfleet.NormalizeProtocol(protocol), Strategy: strategy, Bots: 32,
			Requests: 0, Duration: 10 * time.Second, Timeout: 2 * time.Second,
			Method: "POST", Path: "/", ExpectReply: true, RampUpDuration: 0,
			TelemetryClient: redisClient, SubmissionID: submissionID,
			JudgingMode: "contest_final", ContestID: contestID, Seed: seed,
		})
		cancel()

		if runErr != nil || metrics.Successes == 0 {
			if runErr != nil {
				log.Printf("[Finalize] Stress test failed for submission %s: %v", submissionID, runErr)
			}
			roundP99 = 100.0 // Set to maximum latency penalty on failure
			return
		}

		// Allow telemetry to flush before consuming events
		time.Sleep(500 * time.Millisecond)

		// Try to score with full telemetry pipeline (validator + correctness)
		if redisClient != nil {
			scoreCtx, scoreCancel := context.WithTimeout(ctx, 10*time.Second)
			events, consumeErr := telemetry.ConsumeAllForSubmission(scoreCtx, redisClient, submissionID)
			scoreCancel()

			if consumeErr == nil && len(events) > 0 {
				perfMetrics := scorer.ComputeMetrics(submissionID, events)
				valResult := validator.RunValidatorFromEvents(submissionID, events)
				sc := scorer.ComputeScore(perfMetrics, valResult)
				roundScore = sc.TotalScore
				roundLat = sc.LatencyScore
				roundThr = sc.ThroughputScore
				roundCor = sc.CorrectnessScore
				roundGrade = sc.Grade
				roundP99 = perfMetrics.P99LatencyMs
				roundTPS = perfMetrics.TPS
				log.Printf("[Finalize] Scored %s via telemetry: score=%.2f lat=%.2f thr=%.2f cor=%.2f grade=%s (orders=%d, errors=%d)",
					submissionID, roundScore, roundLat, roundThr, roundCor, roundGrade, valResult.OrdersProcessed, valResult.TotalErrors())
				return
			}
			log.Printf("[Finalize] WARNING: telemetry events unavailable for %s (err=%v, events=%d), falling back to summary metrics", submissionID, consumeErr, len(events))
		}

		// Fallback: score from botfleet Summary metrics (no correctness possible)
		perfMetrics := scorer.PerformanceMetrics{
			SubmissionID: submissionID, TotalRequests: metrics.Requests, Successes: metrics.Successes, Failures: metrics.Failures,
			TPS: metrics.RequestsPerSecond, MinLatencyMs: metrics.MinLatencyMs, AvgLatencyMs: metrics.AverageLatencyMs,
			P50LatencyMs: metrics.P50LatencyMs, P90LatencyMs: metrics.P90LatencyMs, P99LatencyMs: metrics.P99LatencyMs,
			MaxLatencyMs: metrics.MaxLatencyMs, StdDevMs: metrics.StdDevMs,
		}
		valResult := validator.ValidationResult{}
		sc := scorer.ComputeScore(perfMetrics, valResult)
		roundScore = sc.TotalScore
		roundLat = sc.LatencyScore
		roundThr = sc.ThroughputScore
		roundCor = sc.CorrectnessScore
		roundGrade = sc.Grade
		roundP99 = perfMetrics.P99LatencyMs
		roundTPS = perfMetrics.TPS
		return
	}

	// launchSandboxFromSource writes source code to a temp file, launches a sandbox, and returns the target URL + cleanup func.
	launchSandboxFromSource := func(teamName, sourceCode, language, protocol string, label string) (string, func()) {
		port := extractPortFromSourceCode([]byte(sourceCode))
		if port <= 0 {
			port = 8080
		}

		ext, extErr := extensionForLanguage(language)
		if extErr != nil {
			log.Printf("[Finalize] Unsupported language %s for %s (%s)", language, teamName, label)
			return "", nil
		}

		sanitizedSystem := sanitizeDNSName(teamName)
		submissionName := fmt.Sprintf("finalize-%s-%d-%s%s", sanitizedSystem, time.Now().UnixNano(), randomString(4), ext)
		filePath := filepath.Join("./workspace", submissionName)

		if writeErr := os.WriteFile(filePath, []byte(sourceCode), 0644); writeErr != nil {
			log.Printf("[Finalize] Failed to write temp file for %s (%s): %v", teamName, label, writeErr)
			return "", nil
		}

		execCtx, execCancel := context.WithTimeout(ctx, 135*time.Second)
		res, execErr := sandbox.ExecuteCode(execCtx, filePath, language, port, teamName)
		execCancel()
		os.Remove(filePath)

		if execErr != nil || res.Phase != "Running" {
			log.Printf("[Finalize] ExecuteCode failed for %s (%s): %v", teamName, label, execErr)
			return "", nil
		}

		targetURL := buildTargetURL(res, protocol, port)
		cleanupFunc := func() {
			log.Printf("[Finalize] Cleaning up sandbox for %s (%s): %s", teamName, label, res.PodID)
			_ = sandbox.CleanupSandbox(res.PodID)
		}
		log.Printf("[Finalize] Sandbox for %s (%s) successfully started at %s (NodePort: %d)", teamName, label, targetURL, res.NodePort)
		return targetURL, cleanupFunc
	}

	for idx, teamName := range teams {
		var roundResults []map[string]interface{}
		var allScores, allLat, allThr, allCor, allP99, allTPS []float64

		if best, ok := bestLiveScores[teamName]; ok {
			// Do NOT append to allScores, allLat, etc. so it is not counted in finalized averages.
			// Only include it in roundResults for display.
			roundResults = append(roundResults, map[string]interface{}{
				"problem_code":  "Live",
				"problem_title": "Best Live Score",
				"strategy":      best.Strategy,
				"label":         fmt.Sprintf("Best Live (%s)", best.Strategy),
				"score":         best.TotalScore,
				"grade":         best.Grade,
			})
		}

		if len(problems) > 0 {
			// ── Per-problem finalization ──
			for _, p := range problems {
				probStrategies := p.HiddenStrategies
				if len(probStrategies) == 0 {
					probStrategies = finalStrategies
				}

				probProtocol := "http"
				if p.HiddenProtocol != "" {
					probProtocol = strings.ToLower(strings.TrimSpace(p.HiddenProtocol))
				}

				submissionProtocol := probProtocol

				// Try to get the LATEST live submission for this team+problem
				sub, subErr := db.GetLatestLiveSubmissionForTeamAndProblem(ctx, contestID, teamName, p.ID)
				if subErr == nil && sub != nil {
					// 1. Detect protocol from source code
					if len(sub.SourceCode) > 0 {
						if proto := detectProtocolFromSourceCode([]byte(sub.SourceCode)); proto != "" {
							submissionProtocol = proto
						}
					}
					// 2. Fallback: Parse from sub.RawMetrics
					if submissionProtocol == probProtocol && len(sub.RawMetrics) > 0 {
						var payload struct {
							Rounds []struct {
								Metrics struct {
									Protocol string `json:"protocol"`
								} `json:"metrics"`
							} `json:"rounds"`
						}
						if err := json.Unmarshal(sub.RawMetrics, &payload); err == nil && len(payload.Rounds) > 0 {
							if pVal := strings.TrimSpace(payload.Rounds[0].Metrics.Protocol); pVal != "" {
								submissionProtocol = strings.ToLower(pVal)
							}
						}
					}
				}

				var targetURL string
				var cleanupFunc func()

				if subErr == nil && sub != nil && sub.SourceCode != "" {
					log.Printf("[Finalize] Launching latest engine for team %s, problem %s (%s) using protocol %s...", teamName, p.Code, p.Title, submissionProtocol)
					targetURL, cleanupFunc = launchSandboxFromSource(teamName, sub.SourceCode, sub.Language, submissionProtocol, fmt.Sprintf("problem %s", p.Code))
				}

				// Fallback: try already-running sandbox
				if targetURL == "" {
					log.Printf("[Finalize] Warning: could not launch sandbox for %s (problem %s), falling back to running sandbox...", teamName, p.Code)
					targetURL, _ = sandbox.GetSandboxTargetURL(ctx, teamName, submissionProtocol)
				}

				if targetURL == "" {
					for _, stratName := range probStrategies {
						allScores = append(allScores, 0.0)
						allLat = append(allLat, 0.0)
						allThr = append(allThr, 0.0)
						allCor = append(allCor, 0.0)
						allP99 = append(allP99, 100.0) // Set to max latency penalty
						allTPS = append(allTPS, 0.0)
						roundResults = append(roundResults, map[string]interface{}{
							"problem_code":  p.Code,
							"problem_title": p.Title,
							"strategy":      stratName,
							"label":         fmt.Sprintf("%s - %s", p.Code, stratName),
							"score":         0.0,
							"grade":         "F",
							"error":         "no submission code and sandbox not running",
						})
					}
				} else {
					for i, stratName := range probStrategies {
						strategy := botfleet.NormalizeStrategy(stratName)
						submissionID := fmt.Sprintf("finalize-%s-%s-%d-%d", contestID, p.Code, time.Now().UnixNano(), i)
						seed := botfleet.DeterministicSeedForStrategy(strategy)

						roundScore, roundLat, roundThr, roundCor, roundP99, roundTPS, roundGrade := runFinalStressTest(
							targetURL, submissionProtocol, submissionID, strategy, seed,
						)

						allScores = append(allScores, roundScore)
						allLat = append(allLat, roundLat)
						allThr = append(allThr, roundThr)
						allCor = append(allCor, roundCor)
						allP99 = append(allP99, roundP99)
						allTPS = append(allTPS, roundTPS)

						roundResults = append(roundResults, map[string]interface{}{
							"problem_code":  p.Code,
							"problem_title": p.Title,
							"strategy":      stratName,
							"label":         fmt.Sprintf("%s - %s", p.Code, stratName),
							"score":         roundScore,
							"grade":         roundGrade,
						})
					}

					if cleanupFunc != nil {
						cleanupFunc()
					}
				}
			}
		} else {
			log.Printf("[Finalize] WARNING: No problems defined for contest %s, skipping scoring for team %s", contestID, teamName)
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
			finalGrade = scorer.AssignGrade(avgScore)
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
