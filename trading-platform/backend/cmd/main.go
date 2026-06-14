// backend/cmd/main.go
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Saurabh-52/trading-platform/internal/auth"
	"github.com/Saurabh-52/trading-platform/internal/botfleet"
	"github.com/Saurabh-52/trading-platform/internal/sandbox"
	"github.com/Saurabh-52/trading-platform/internal/scorer"
	"github.com/Saurabh-52/trading-platform/internal/store"
	"github.com/Saurabh-52/trading-platform/internal/telemetry"
	"github.com/Saurabh-52/trading-platform/internal/validator"
	ws "github.com/Saurabh-52/trading-platform/internal/ws"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/websocket/v2"
)

var sourceCodeMap sync.Map

// sandboxOutcome bundles the result of an ExecuteCode call so it can be sent
// safely through a channel without sharing mutable state across goroutines.
type sandboxOutcome struct {
	result sandbox.ExecutionResult
	err    error
}

type stressTestRequest struct {
	Target        string `json:"target"`
	Protocol      string `json:"protocol"`
	Strategy      string `json:"strategy"`
	SystemName    string `json:"system_name"`
	Language      string `json:"language"`
	Bots          int    `json:"bots"`
	Requests      int    `json:"requests"`
	DurationSecs  int    `json:"duration_seconds"`
	TimeoutMillis int    `json:"timeout_ms"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	ExpectReply   bool   `json:"expect_reply"`
	RampUpSecs    int    `json:"ramp_up_seconds"`
	JudgingMode   string `json:"judging_mode"`
	ContestID     string `json:"contest_id"`
	ProblemID     string `json:"problem_id"`
	Filename      string `json:"filename"`
}

func extensionForLanguage(language string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "cpp", "c++", "cc", "cxx":
		return ".cpp", nil
	case "go":
		return ".go", nil
	case "rust":
		return ".rs", nil
	case "python", "py":
		return ".py", nil
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

func sanitizeDNSName(s string) string {
	var sb strings.Builder
	for _, r := range s {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			sb.WriteRune(r)
		} else if r == '-' {
			if sb.Len() > 0 {
				sb.WriteRune(r)
			}
		}
	}
	res := sb.String()
	for len(res) > 0 && res[len(res)-1] == '-' {
		res = res[:len(res)-1]
	}
	if len(res) > 50 {
		res = res[:50]
	}
	return strings.ToLower(res)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			b[i] = letters[0]
		} else {
			b[i] = letters[num.Int64()]
		}
	}
	return string(b)
}

func extensionAllowedForLanguage(filename string, language string) bool {
	allowedExtensions := map[string][]string{
		"cpp":    {".cpp", ".cc", ".cxx"},
		"c++":    {".cpp", ".cc", ".cxx"},
		"cc":     {".cpp", ".cc", ".cxx"},
		"cxx":    {".cpp", ".cc", ".cxx"},
		"go":     {".go"},
		"rust":   {".rs"},
		"python": {".py"},
		"py":     {".py"},
	}

	normalizedLanguage := strings.ToLower(strings.TrimSpace(language))
	extension := strings.ToLower(filepath.Ext(filename))

	for _, allowedExtension := range allowedExtensions[normalizedLanguage] {
		if extension == allowedExtension {
			return true
		}
	}

	return false
}

// buildTargetURL constructs the URL the bot fleet should use to reach the
// sandbox engine.  When running in-cluster the service DNS name is used;
// when running locally it retrieves the minikube IP or falls back to 127.0.0.1 and uses NodePort.
func buildTargetURL(result sandbox.ExecutionResult, protocol string, containerPort int) string {
	proto := strings.ToLower(strings.TrimSpace(protocol))

	if sandbox.InCluster() {
		host := fmt.Sprintf("%s.trading-sandbox.svc.cluster.local", result.ServiceName)
		if proto == "tcp" || proto == "fix" {
			return fmt.Sprintf("%s:%d", host, containerPort)
		}
		return fmt.Sprintf("http://%s:%d", host, containerPort)
	}

	// Running locally outside cluster: try to get minikube IP, fallback to 127.0.0.1.
	// Use NodePort if available to support parallel executions, fallback to containerPort.
	hostIP := "127.0.0.1"
	if minikubeIP := os.Getenv("MINIKUBE_IP"); minikubeIP != "" && runtime.GOOS == "linux" {
		hostIP = minikubeIP
	}

	portToUse := containerPort
	// If minikube IP is explicitly provided, use NodePort since that's how we reach it.
	// Otherwise, assume local development via minikube tunnel, which exposes the LoadBalancer on 127.0.0.1:containerPort
	if result.NodePort > 0 && hostIP != "127.0.0.1" {
		portToUse = int(result.NodePort)
	}

	if proto == "tcp" || proto == "fix" {
		return fmt.Sprintf("%s:%d", hostIP, portToUse)
	}
	return fmt.Sprintf("http://%s:%d", hostIP, portToUse)
}

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

func main() {
	// Clean up any leftover sandboxes on server startup
	go func() {
		log.Println("Starting global cleanup of old sandboxes...")
		if err := sandbox.CleanupAllSandboxes(context.Background()); err != nil {
			log.Printf("Startup cleanup warning: %v", err)
		} else {
			log.Println("Global cleanup of old sandboxes completed successfully")
		}
	}()

	app := fiber.New(fiber.Config{
		BodyLimit: 100 * 1024 * 1024, // 100MB limit to support base64 contest banner images
	})

	// Enable CORS for frontend communication
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173, http://localhost:5174, http://localhost:3000",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}))

	// Detect minikube IP if running locally
	if os.Getenv("MINIKUBE_IP") == "" && !sandbox.InCluster() {
		cmd := exec.Command("minikube", "ip")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			ip := strings.TrimSpace(out.String())
			if ip != "" && net.ParseIP(ip) != nil {
				os.Setenv("MINIKUBE_IP", ip)
				log.Printf("Automatically detected Minikube IP: %s", ip)
			}
		}
	}

	// Ensure our temporary workspace exists
	os.MkdirAll("./workspace", os.ModePerm)

	// --- Redis (optional — telemetry is best-effort) ---
	var redisClient *redis.Client
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	rc, err := telemetry.NewRedisClient(redisAddr)
	if err != nil {
		log.Printf("WARNING: Redis not available (%s) — telemetry disabled: %v", redisAddr, err)
	} else {
		redisClient = rc.Underlying()
		log.Printf("Redis connected at %s", redisAddr)
	}

	// --- Postgres (Required for authentication and scoring) ---
	var db *store.Store
	pgURL := os.Getenv("POSTGRES_URL")
	if pgURL == "" {
		pgURL = "postgres://user:password@127.0.0.1:5432/postgres?sslmode=disable"
	}
	
	// Try to connect to PostgreSQL with retries
	for i := 0; i < 10; i++ {
		dbConn, err := store.NewStore(context.Background(), pgURL)
		if err == nil {
			db = dbConn
			log.Println("PostgreSQL connected and migrated")
			break
		}
		log.Printf("WARNING: PostgreSQL not available (attempt %d/10): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	if db == nil {
		log.Println("ERROR: PostgreSQL not available after retries. Endpoints requiring db will return 'database not available'.")
	}

	// Health check endpoint for Kubernetes probes
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})

	// --- WebSocket hub for real-time leaderboard updates ---
	hub := ws.NewHub()
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		ws.HandleConnection(hub, c)
	}))

	// ─── Auth routes ────────────────────────────────────────────────────────
	app.Post("/register", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}

		var payload struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid payload", "error": err.Error()})
		}

		username := strings.TrimSpace(payload.Username)
		email := strings.TrimSpace(payload.Email)
		password := payload.Password

		if username == "" || email == "" || password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "username, email, and password are required"})
		}
		if len(password) < 6 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "password must be at least 6 characters"})
		}
		if len(username) < 3 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "username must be at least 3 characters"})
		}

		// Check uniqueness
		if _, err := db.GetUserByUsername(c.Context(), username); err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"message": "username already taken"})
		}
		if _, err := db.GetUserByEmail(c.Context(), email); err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"message": "email already registered"})
		}

		hash, err := auth.HashPassword(password)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to hash password"})
		}

		userID := fmt.Sprintf("usr-%d-%s", time.Now().UnixNano(), randomString(6))
		if err := db.CreateUser(c.Context(), userID, username, email, hash); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to create user", "error": err.Error()})
		}

		token, err := auth.GenerateJWT(userID, username)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to generate token"})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "Registration successful",
			"token":   token,
			"user": fiber.Map{
				"id":       userID,
				"username": username,
				"email":    email,
			},
		})
	})

	app.Post("/login", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}

		var payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid payload", "error": err.Error()})
		}

		username := strings.TrimSpace(payload.Username)
		password := payload.Password

		if username == "" || password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "username and password are required"})
		}

		user, err := db.GetUserByUsername(c.Context(), username)
		if err != nil {
			log.Printf("LOGIN FAILED: user '%s' not found or db error: %v", username, err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "invalid username or password"})
		}

		if !auth.CheckPassword(user.PasswordHash, password) {
			log.Printf("LOGIN FAILED: password mismatch for user '%s' (provided pass length: %d)", username, len(password))
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "invalid username or password"})
		}

		token, err := auth.GenerateJWT(user.ID, user.Username)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to generate token"})
		}

		return c.JSON(fiber.Map{
			"message": "Login successful",
			"token":   token,
			"user": fiber.Map{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
			},
		})
	})

	app.Get("/me", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		userID := c.Locals("user_id").(string)
		user, err := db.GetUserByID(c.Context(), userID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "user not found"})
		}
		return c.JSON(fiber.Map{
			"user": fiber.Map{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
			},
		})
	})

	app.Get("/users/:username/profile", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		username := c.Params("username")
		if strings.TrimSpace(username) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "username is required"})
		}

		user, err := db.GetUserByUsername(c.Context(), username)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "user not found"})
		}

		page, _ := strconv.Atoi(c.Query("page", "1"))
		pageSize, _ := strconv.Atoi(c.Query("pageSize", "10"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 || pageSize > 100 {
			pageSize = 10
		}

		bestScore, err := db.GetBestScore(c.Context(), user.ID)
		var bestScorePtr *store.SubmissionResult
		if err == nil {
			bestScorePtr = bestScore
		}

		history, total, err := db.GetPaginatedUserHistory(c.Context(), user.ID, page, pageSize)
		if err != nil {
			history = []store.SubmissionResult{}
		}

		return c.JSON(fiber.Map{
			"user": fiber.Map{
				"id":       user.ID,
				"username": user.Username,
				"email":    user.Email,
			},
			"best_score": bestScorePtr,
			"history":    history,
			"total":      total,
		})
	})

	// User's own submission history (requires auth)
	app.Get("/history/me", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		userID := c.Locals("user_id").(string)
		limit, _ := strconv.Atoi(c.Query("limit", "50"))
		if limit <= 0 || limit > 100 {
			limit = 50
		}
		contestID := c.Query("contest_id", "")
		problemID := c.Query("problem_id", "")
		judgingMode := c.Query("judging_mode", "")
		results, err := db.GetUserHistory(c.Context(), userID, contestID, problemID, judgingMode, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "history query failed", "error": err.Error()})
		}
		return c.JSON(fiber.Map{"history": results})
	})

	// --- Leaderboard & Submission lookup ---
	app.Get("/leaderboard", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		strategyFilter := strings.TrimSpace(c.Query("strategy", ""))

		var results []store.SubmissionResult
		var err error
		if strategyFilter != "" {
			results, err = db.GetLeaderboardByStrategy(c.Context(), strategyFilter, limit)
		} else {
			results, err = db.GetLeaderboard(c.Context(), limit)
		}
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "leaderboard query failed", "error": err.Error()})
		}
		return c.JSON(fiber.Map{"leaderboard": results})
	})

	app.Get("/history/:systemName", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		systemName := c.Params("systemName")
		if strings.TrimSpace(systemName) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "systemName is required"})
		}
		
		limit, _ := strconv.Atoi(c.Query("limit", "50"))
		if limit <= 0 || limit > 100 {
			limit = 50
		}

		results, err := db.GetSubmissionHistory(c.Context(), systemName, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "history query failed", "error": err.Error()})
		}
		return c.JSON(fiber.Map{"history": results})
	})

	app.Get("/submission/:id", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		result, err := db.GetSubmission(c.Context(), c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "submission not found", "error": err.Error()})
		}
		return c.JSON(result)
	})
	// The endpoint where contestants submit their code
	app.Post("/submit", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("PANIC in /submit:", r)
				c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"message": "server panic during processing",
					"error":   fmt.Sprintf("%v", r),
				})
			}
		}()

		file, err := c.FormFile("source_code")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "source_code file is required"})
		}

		systemName := strings.TrimSpace(c.FormValue("systemName", ""))
		if systemName == "" {
			systemName = "default"
		}

		language := c.FormValue("language", "cpp")
		protocol := strings.ToLower(strings.TrimSpace(c.FormValue("protocol", "http")))
		portValue := strings.TrimSpace(c.FormValue("port", ""))
		if portValue == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "port is required"})
		}

		port, err := strconv.Atoi(portValue)
		if err != nil || port < 1 || port > 65535 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "port must be a valid TCP port"})
		}

		ext, err := extensionForLanguage(language)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
		}

		if !extensionAllowedForLanguage(file.Filename, language) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": fmt.Sprintf("file extension %s does not match selected language %s", filepath.Ext(file.Filename), language)})
		}

		sanitizedSystem := sanitizeDNSName(systemName)
		if sanitizedSystem == "" {
			sanitizedSystem = "default"
		}
		submissionName := fmt.Sprintf("%s-%d-%s%s", sanitizedSystem, time.Now().UnixNano(), randomString(4), ext)

		filePath := filepath.Join("./workspace", submissionName)
		if err := c.SaveFile(file, filePath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to save uploaded file"})
		}
		// Ensure host disk cleanup once this request returns
		defer os.Remove(filePath)
		sourceCodeBytes, err := os.ReadFile(filePath)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to read uploaded file", "error": err.Error()})
		}
		if len(sourceCodeBytes) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "uploaded source_code file is empty"})
		}

		// Verify port and protocol match the uploaded source code.
		if detectedPort := extractPortFromSourceCode(sourceCodeBytes); detectedPort > 0 {
			if detectedPort != port {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"message": fmt.Sprintf("The port specified in the form (%d) does not match the port detected in your source code (%d). Please match them.", port, detectedPort),
				})
			}
		}

		if detectedProto := detectProtocolFromSourceCode(sourceCodeBytes); detectedProto != "" {
			if detectedProto != protocol {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"message": fmt.Sprintf("The protocol specified in the form (%s) does not match the protocol detected in your source code (%s). Please match them.", protocol, detectedProto),
				})
			}
		}

		fmt.Println("Attempting to start sandbox for:", filePath)

		// Execute sandbox with timeout context — increased to 135s because
		// waitForPodReady can take up to 120s for compilation + startup.
		ctx, cancel := context.WithTimeout(context.Background(), 135*time.Second)
		defer cancel()

		// Channel-based result passing eliminates the data race that occurred
		// when the goroutine and the timeout path both accessed shared locals.
		resultCh := make(chan sandboxOutcome, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("PANIC in sandbox execution:", r)
					resultCh <- sandboxOutcome{err: fmt.Errorf("sandbox panic: %v", r)}
				}
			}()
			result, err := sandbox.ExecuteCode(ctx, filePath, language, port, systemName)
			resultCh <- sandboxOutcome{result: result, err: err}
		}()

		var executionResult sandbox.ExecutionResult
		var executionErr error

		select {
		case outcome := <-resultCh:
			executionResult = outcome.result
			executionErr = outcome.err
			fmt.Println("Sandbox execution completed")
		case <-ctx.Done():
			executionErr = fmt.Errorf("sandbox execution timeout")
			fmt.Println("Sandbox execution timed out")
			go func() {
				if outcome, ok := <-resultCh; ok && outcome.result.PodID != "" {
					fmt.Printf("Cleaning up orphaned sandbox pod %s after timeout\n", outcome.result.PodID)
					_ = sandbox.CleanupSandbox(outcome.result.PodID)
				}
			}()
		}

		if executionErr != nil {
			if executionResult.PodID != "" {
				fmt.Printf("Cleaning up sandbox pod %s after execution error\n", executionResult.PodID)
				go sandbox.CleanupSandbox(executionResult.PodID)
			}
			fmt.Println("EXECUTION ERROR:", executionErr)
			response := fiber.Map{
				"message": "failed to execute submission",
				"error":   executionErr.Error(),
				"execution_result": fiber.Map{
					"pod_id":       executionResult.PodID,
					"service_name": executionResult.ServiceName,
					"phase":        executionResult.Phase,
					"output":       executionResult.Output,
					"node_port":    executionResult.NodePort,
				},
				"form_data": fiber.Map{
					"language":             language,
					"port":                 port,
					"original_filename":    file.Filename,
					"stored_filename":      submissionName,
					"stored_relative_path": filePath,
				},
			}

			if payload, marshalErr := json.Marshal(response); marshalErr == nil {
				fmt.Println("RESPONSE:", string(payload))
			}

			return c.Status(fiber.StatusInternalServerError).JSON(response)
		}

		// Construct the target URL the bot fleet should use.
		targetURL := buildTargetURL(executionResult, protocol, port)
		sourceCodeMap.Store(targetURL, string(sourceCodeBytes))

		fmt.Println("✓ Submission processed successfully, returning JSON response")
		response := fiber.Map{
			"message": "Submission processed",
			"form_data": fiber.Map{
				"language":             language,
				"port":                 port,
				"protocol":             protocol,
				"original_filename":    file.Filename,
				"stored_filename":      submissionName,
				"stored_relative_path": filePath,
			},
			"execution_result": fiber.Map{
				"pod_id":       executionResult.PodID,
				"service_name": executionResult.ServiceName,
				"phase":        executionResult.Phase,
				"output":       executionResult.Output,
				"node_port":    executionResult.NodePort,
				"target_url":   targetURL,
			},
		}

		if payload, marshalErr := json.Marshal(response); marshalErr == nil {
			fmt.Println("RESPONSE:", string(payload))
		}

		return c.JSON(response)
	})

	app.Post("/stress-test", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		var req stressTestRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid stress-test payload", "error": err.Error()})
		}
		if strings.TrimSpace(req.Target) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "target is required"})
		}

		targetURL := req.Target
		defer func() {
			go func() {
				// Wait 1.5 seconds to allow any pending telemetry to flush before pod is destroyed
				time.Sleep(1500 * time.Millisecond)
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cleanupCancel()
				if err := sandbox.CleanupSandboxByTarget(cleanupCtx, targetURL); err != nil {
					log.Printf("Auto-cleanup sandbox error for target %s: %v", targetURL, err)
				} else {
					log.Printf("Auto-cleanup sandbox success for target %s", targetURL)
				}
			}()
		}()

		if req.ContestID != "" && db != nil {
			contest, err := db.GetContest(c.Context(), req.ContestID)
			if err != nil {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "contest not found"})
			}
			if contest.Phase == "finalizing" || contest.Phase == "completed" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "contest has ended, submissions are locked"})
			}
			endTime := contest.StartTime.Add(time.Duration(contest.DurationMinutes) * time.Minute)
			if time.Now().After(endTime) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "contest has ended, submissions are locked"})
			}
		}

		duration := time.Duration(req.DurationSecs) * time.Second
		if duration <= 0 && req.Requests <= 0 {
			duration = 10 * time.Second
		}
		timeout := time.Duration(req.TimeoutMillis) * time.Millisecond
		if timeout <= 0 {
			timeout = 2 * time.Second
		}

		userStrategy := botfleet.NormalizeStrategy(req.Strategy)
		systemName := strings.TrimSpace(req.SystemName)
		judgingMode := botfleet.NormalizeJudgingMode(req.JudgingMode)

		// --- Helper: run one stress-test round and score it ---
		type roundResult struct {
			SubmissionID     string                      `json:"submission_id"`
			Strategy         string                      `json:"strategy"`
			JudgingMode      string                      `json:"judging_mode"`
			SeedUsed         int64                       `json:"seed_used"`
			Metrics          botfleet.Summary            `json:"metrics"`
			Score            *scorer.Score               `json:"score,omitempty"`
			PerfMetrics      *scorer.PerformanceMetrics  `json:"perf_metrics,omitempty"`
			ValResult        *validator.ValidationResult `json:"val_result,omitempty"`
			CorrectnessHint  string                      `json:"correctness_hint,omitempty"`
		}
		runRound := func(strategy botfleet.Strategy, idSuffix string) (roundResult, error) {
			submissionID := fmt.Sprintf("stress-%d%s", time.Now().UnixNano(), idSuffix)
			seed := botfleet.DeterministicSeedForStrategy(strategy)

			roundCtx, roundCancel := context.WithTimeout(context.Background(), duration+timeout+5*time.Second)
			defer roundCancel()

			metrics, err := botfleet.Run(roundCtx, botfleet.Config{
				Target:          req.Target,
				Protocol:        botfleet.NormalizeProtocol(req.Protocol),
				Strategy:        strategy,
				Bots:            req.Bots,
				Requests:        req.Requests,
				Duration:        duration,
				Timeout:         timeout,
				Method:          req.Method,
				Path:            req.Path,
				ExpectReply:     req.ExpectReply,
				RampUpDuration:  time.Duration(req.RampUpSecs) * time.Second,
				TelemetryClient: redisClient,
				SubmissionID:    submissionID,
				JudgingMode:     judgingMode,
				ContestID:       req.ContestID,
				Seed:            seed,
			})
			if err != nil {
				return roundResult{}, err
			}

			rr := roundResult{
				SubmissionID: submissionID,
				Strategy:     string(strategy),
				JudgingMode:  string(judgingMode),
				SeedUsed:     seed,
				Metrics:      metrics,
			}

			// Scoring pipeline (best-effort)
			if redisClient != nil {
				time.Sleep(200 * time.Millisecond)
				scoreCtx, scoreCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer scoreCancel()
				events, consumeErr := telemetry.ConsumeAllForSubmission(scoreCtx, redisClient, submissionID)
				
				var sc scorer.Score
				var perfMetrics scorer.PerformanceMetrics
				var valResult validator.ValidationResult

				if consumeErr == nil && len(events) > 0 {
					perfMetrics = scorer.ComputeMetrics(submissionID, events)
					valResult = validator.RunValidatorFromEvents(submissionID, events)
					sc = scorer.ComputeScore(perfMetrics, valResult)
					rr.Score = &sc
					rr.PerfMetrics = &perfMetrics
					rr.ValResult = &valResult
				} else {
					// Fallback to botfleet Summary metrics if telemetry events are missing
					sc = scorer.Score{
						Grade: "F",
					}
					perfMetrics = scorer.PerformanceMetrics{
						SubmissionID:  submissionID,
						TotalRequests: metrics.Requests,
						Successes:     metrics.Successes,
						Failures:      metrics.Failures,
						TPS:           metrics.RequestsPerSecond,
						MinLatencyMs:  metrics.MinLatencyMs,
						AvgLatencyMs:  metrics.AverageLatencyMs,
						P50LatencyMs:  metrics.P50LatencyMs,
						P90LatencyMs:  metrics.P90LatencyMs,
						P99LatencyMs:  metrics.P99LatencyMs,
						MaxLatencyMs:  metrics.MaxLatencyMs,
						StdDevMs:      metrics.StdDevMs,
					}
					if metrics.Successes > 0 {
						// If there are successes, let's still compute a rough score
						sc = scorer.ComputeScore(perfMetrics, valResult)
					}
					rr.Score = &sc
					rr.PerfMetrics = &perfMetrics
					rr.ValResult = &valResult
				}

				isRust := strings.ToLower(strings.TrimSpace(req.Language)) == "rust"
				isZeroCorrectness := rr.Score == nil || rr.Score.CorrectnessScore == 0
				if !isRust && isZeroCorrectness {
					if hint := valResult.CorrectnessHint(); hint != "" {
						rr.CorrectnessHint = hint
					}
				}
			}

			return rr, nil
		}

		// Retrieve sample strategies for this problem
		var sampleStrategies []string
		if req.ContestID != "" && req.ProblemID != "" && db != nil {
			probs, err := db.GetContestProblems(c.Context(), req.ContestID)
			if err == nil {
				for _, p := range probs {
					if p.ID == req.ProblemID {
						sampleStrategies = p.SampleStrategies
						break
					}
				}
			}
		}
		if len(sampleStrategies) == 0 {
			sampleStrategies = []string{req.Strategy}
		}

		// --- Run the appropriate strategies ---
		var rounds []roundResult
		if req.ContestID != "" && db != nil {
			log.Printf("Running contest stress test on problem %s sample strategies: %v, system_name=%q", req.ProblemID, sampleStrategies, systemName)
			for i, strat := range sampleStrategies {
				strategyEnum := botfleet.NormalizeStrategy(strat)
				suffix := ""
				if len(sampleStrategies) > 1 {
					suffix = fmt.Sprintf("-%d", i+1)
				}
				round, err := runRound(strategyEnum, suffix)
				if err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "stress test failed", "error": err.Error()})
				}
				rounds = append(rounds, round)
			}
		} else {
			log.Printf("Running stress test: strategy=%s system_name=%q", userStrategy, systemName)
			userRound, err := runRound(userStrategy, "")
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "stress test failed", "error": err.Error()})
			}
			rounds = append(rounds, userRound)

			if userStrategy != botfleet.StrategyBBOHeavy {
				log.Printf("Running baseline bbo_heavy stress test for %q", systemName)
				bboRound, err := runRound(botfleet.StrategyBBOHeavy, "-bbo")
				if err == nil {
					rounds = append(rounds, bboRound)
				}
			}
		}

		// Save aggregated results to the database if rounds were successfully run
		if len(rounds) > 0 && db != nil {
			submitUserID := ""
			if uid, ok := c.Locals("user_id").(string); ok {
				submitUserID = uid
			}
			sourceCodeStr := ""
			if src, ok := sourceCodeMap.LoadAndDelete(req.Target); ok {
				if str, valid := src.(string); valid {
					sourceCodeStr = str
				}
			}

			// Aggregate scores and metrics
			var totalScore, latencyScore, throughputScore, correctnessScore float64
			var totalTPS, totalP99 float64
			var totalCrossEvents, totalMismatchEvents, totalUnparseableEvents, totalOrdersProcessed int
			var scoredCount int

			for _, r := range rounds {
				if r.Score != nil {
					totalScore += r.Score.TotalScore
					latencyScore += r.Score.LatencyScore
					throughputScore += r.Score.ThroughputScore
					correctnessScore += r.Score.CorrectnessScore
					scoredCount++
				}
				if r.PerfMetrics != nil {
					totalTPS += r.PerfMetrics.TPS
					totalP99 += r.PerfMetrics.P99LatencyMs
				}
				if r.ValResult != nil {
					totalCrossEvents += r.ValResult.CrossEvents
					totalMismatchEvents += r.ValResult.MismatchEvents
					totalUnparseableEvents += r.ValResult.UnparseableEvents
					totalOrdersProcessed += r.ValResult.OrdersProcessed
				}
			}

			avgTotalScore := 0.0
			avgLatencyScore := 0.0
			avgThroughputScore := 0.0
			avgCorrectnessScore := 0.0
			avgTPS := 0.0
			avgP99 := 0.0

			if scoredCount > 0 {
				avgTotalScore = totalScore / float64(scoredCount)
				avgLatencyScore = latencyScore / float64(scoredCount)
				avgThroughputScore = throughputScore / float64(scoredCount)
				avgCorrectnessScore = correctnessScore / float64(scoredCount)
				avgTPS = totalTPS / float64(scoredCount)
				avgP99 = totalP99 / float64(scoredCount)
			}

			overallGrade := scorer.AssignGrade(avgTotalScore)

			overallVal := validator.ValidationResult{
				SubmissionID:      rounds[0].SubmissionID,
				CrossEvents:       totalCrossEvents,
				MismatchEvents:    totalMismatchEvents,
				UnparseableEvents: totalUnparseableEvents,
				OrdersProcessed:   totalOrdersProcessed,
				Valid:             totalCrossEvents == 0 && totalMismatchEvents == 0 && totalUnparseableEvents == 0,
			}

			// Package rounds in raw_metrics
			type multiStrategyPayload struct {
				IsMultiStrategy bool           `json:"is_multi_strategy"`
				Rounds          []roundResult  `json:"rounds"`
			}
			payload := multiStrategyPayload{
				IsMultiStrategy: len(rounds) > 1,
				Rounds:          rounds,
			}
			rawMetricsJSON, _ := json.Marshal(payload)
			rawValidationJSON, _ := json.Marshal(overallVal)

			sr := store.SubmissionResult{
				SubmissionID:     rounds[0].SubmissionID,
				SystemName:       systemName,
				Strategy:         req.Strategy,
				Language:         req.Language,
				SubmittedAt:      time.Now().UTC(),
				TotalScore:       avgTotalScore,
				LatencyScore:     avgLatencyScore,
				ThroughputScore:  avgThroughputScore,
				CorrectnessScore: avgCorrectnessScore,
				Grade:            overallGrade,
				P99LatencyMs:     avgP99,
				TPS:              avgTPS,
				CrossEvents:      totalCrossEvents,
				OrdersProcessed:  totalOrdersProcessed,
				RawMetrics:       rawMetricsJSON,
				RawValidation:    rawValidationJSON,
				JudgingMode:      string(judgingMode),
				ContestID:        req.ContestID,
				UserID:           submitUserID,
				SourceCode:       sourceCodeStr,
				ProblemID:        req.ProblemID,
				SeedUsed:         rounds[0].SeedUsed,
				Filename:         req.Filename,
			}

			scoreCtx, scoreCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer scoreCancel()
			if storeErr := db.CreateSubmissionResult(scoreCtx, sr); storeErr != nil {
				log.Printf("WARNING: failed to persist combined scoring result: %v", storeErr)
			}
		}

		// --- Broadcast updated leaderboard after all rounds ---
		if db != nil {
			go func() {
				lbCtx, lbCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer lbCancel()
				if lb, lbErr := db.GetLeaderboard(lbCtx, 20); lbErr == nil {
					hub.Broadcast(ws.LeaderboardUpdate{
						Type:    "leaderboard_update",
						Payload: lb,
					})
				}
			}()
		}

		return c.JSON(fiber.Map{
			"message": "stress test complete",
			"rounds":  rounds,
		})
	})

	// Cleanup endpoint: removes the pod, service, and configmap for a sandbox.
	app.Delete("/sandbox/:podId", func(c *fiber.Ctx) error {
		podID := c.Params("podId")
		if strings.TrimSpace(podID) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "podId is required"})
		}
		if err := sandbox.CleanupSandbox(podID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "cleanup failed",
				"error":   err.Error(),
			})
		}
		return c.JSON(fiber.Map{"message": "sandbox cleaned up", "pod_id": podID})
	})

	// Contest listing route
	app.Get("/contests", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		contests, err := db.GetContests(c.Context())
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to query contests", "error": err.Error()})
		}
		return c.JSON(fiber.Map{"contests": contests})
	})

	// Get the list of contest IDs the authenticated user is registered for
	app.Get("/contests/my-registrations", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		userID := ""
		if uid, ok := c.Locals("user_id").(string); ok {
			userID = uid
		}
		if userID == "" {
			return c.JSON(fiber.Map{"contest_ids": []string{}})
		}
		ids, err := db.GetUserRegistrations(c.Context(), userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to query registrations", "error": err.Error()})
		}
		if ids == nil {
			ids = []string{}
		}
		return c.JSON(fiber.Map{"contest_ids": ids})
	})

	// Contest registration route (requires auth so we can link to user)
	app.Post("/contests/:id/register", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		contestID := c.Params("id")
		if strings.TrimSpace(contestID) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "contest ID is required"})
		}

		var payload struct {
			SystemName string `json:"systemName"`
			Code       string `json:"code"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid registration payload", "error": err.Error()})
		}

		if strings.TrimSpace(payload.SystemName) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "systemName is required"})
		}

		// Extract authenticated user ID
		userID := ""
		if uid, ok := c.Locals("user_id").(string); ok {
			userID = uid
		}

		err := db.RegisterSystemForContest(c.Context(), contestID, payload.SystemName, payload.Code, userID)
		if err != nil {
			status := fiber.StatusInternalServerError
			if strings.Contains(err.Error(), "contest not found") {
				status = fiber.StatusNotFound
			} else if strings.Contains(err.Error(), "invalid contest code") || strings.Contains(err.Error(), "code is required") {
				status = fiber.StatusBadRequest
			}
			return c.Status(status).JSON(fiber.Map{"message": "registration failed", "error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"message": "Registered successfully",
			"contest_id": contestID,
			"system_name": payload.SystemName,
		})
	})

	// Contest Draft saving route (requires auth)
	app.Post("/contests/draft", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		var payload struct {
			Details  json.RawMessage `json:"details"`
			Problems json.RawMessage `json:"problems"`
		}
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid draft payload", "error": err.Error()})
		}
		if err := db.SaveContestDraft(c.Context(), payload.Details, payload.Problems); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to save draft", "error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "Draft saved successfully"})
	})

	// Contest Publishing route (requires auth — creator becomes the host)
	app.Post("/contests/publish", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		var payload struct {
			Details struct {
				ID                   string `json:"id"`
				Name                 string `json:"name"`
				Description          string `json:"description"`
				Visibility           string `json:"visibility"`
				Code                 string `json:"code"`
				StartTime            string `json:"startTime"`
				DurationMinutes      int    `json:"durationMinutes"`
				RegistrationDeadline string `json:"registrationDeadline"`
				Strategy             string `json:"strategy"`
				FinalStrategies      []string `json:"finalStrategies"`
				BannerPreview        string `json:"bannerPreview"`
			} `json:"details"`
			Problems []struct {
				ID                    string `json:"id"`
				Code                  string `json:"code"`
				Title                 string `json:"title"`
				Statement             string `json:"statement"`
				TimeLimit             int    `json:"timeLimit"`
				MemoryLimit           int    `json:"memoryLimit"`
				SampleStrategies      []string `json:"sampleStrategies"`
				SampleBotFiles        []struct {
					Name    string `json:"name"`
					Content string `json:"content"`
				} `json:"sampleBotFiles"`
				SampleShowCustom      bool   `json:"sampleShowCustom"`
				SampleTargetInjection string `json:"sampleTargetInjection"`
				SampleProtocol        string `json:"sampleProtocol"`
				SampleTelemetryFormat string `json:"sampleTelemetryFormat"`
				HiddenStrategies      []string `json:"hiddenStrategies"`
				HiddenBotFiles        []struct {
					Name    string `json:"name"`
					Content string `json:"content"`
				} `json:"hiddenBotFiles"`
				HiddenShowCustom      bool   `json:"hiddenShowCustom"`
				HiddenTargetInjection string `json:"hiddenTargetInjection"`
				HiddenProtocol        string `json:"hiddenProtocol"`
				HiddenTelemetryFormat string `json:"hiddenTelemetryFormat"`
			} `json:"problems"`
		}

		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid publish payload", "error": err.Error()})
		}

		if strings.TrimSpace(payload.Details.Name) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "contest name is required"})
		}
		if strings.TrimSpace(payload.Details.StartTime) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "start time is required"})
		}
		if payload.Details.Visibility == "private" && strings.TrimSpace(payload.Details.Code) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "contest code is required for private visibility"})
		}

		// Parse times (support RFC3339 and standard HTML datetime-local format)
		startTime, err := time.Parse(time.RFC3339, payload.Details.StartTime)
		if err != nil {
			startTime, err = time.Parse("2006-01-02T15:04", payload.Details.StartTime)
			if err != nil {
				// Fallback to location parsing
				startTime, err = time.ParseInLocation("2006-01-02T15:04", payload.Details.StartTime, time.Local)
				if err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "invalid startTime format", "error": err.Error()})
				}
			}
		}

		log.Printf("[PublishContest] payload Details ID: %q, StartTime: %v", payload.Details.ID, payload.Details.StartTime)
		var existingContest store.Contest
		var contestExists bool
		if db != nil && payload.Details.ID != "" {
			ec, err := db.GetContest(c.Context(), payload.Details.ID)
			if err != nil {
				log.Printf("[PublishContest] GetContest error: %v", err)
			} else {
				existingContest = ec
				contestExists = true
				log.Printf("[PublishContest] Found existing contest. ID: %s, StartTime: %v", ec.ID, ec.StartTime)

				// Block editing if the contest has already finished (completed, finalizing, or past duration)
				endTime := ec.StartTime.Add(time.Duration(ec.DurationMinutes) * time.Minute)
				if ec.Phase == "completed" || ec.Phase == "finalizing" || time.Now().After(endTime) {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"message": "cannot edit contest details after it has finished",
					})
				}
			}
		}

		hasStarted := false
		if contestExists {
			hasStarted = existingContest.StartTime.Before(time.Now())
			log.Printf("[PublishContest] hasStarted calculated: %v (existing StartTime: %v, time.Now: %v)", hasStarted, existingContest.StartTime, time.Now())
		}

		if hasStarted {
			// Block changing the banner after the contest has started
			if payload.Details.BannerPreview != existingContest.Banner {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "cannot change contest banner after the contest has started"})
			}

			// Ensure startTime is not modified (compare at minute-level resolution to account for seconds/milliseconds truncation in HTML input)
			if startTime.Unix()/60 != existingContest.StartTime.Unix()/60 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "cannot change start time of an ongoing or completed contest"})
			}

			// Validate problems: do not allow adding new ones, and do not allow editing post-contest evaluation
			existingProblems, err := db.GetContestProblems(c.Context(), payload.Details.ID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to query existing problems", "error": err.Error()})
			}

			existingIDs := make(map[string]store.ProblemData)
			for _, ep := range existingProblems {
				existingIDs[ep.ID] = ep
			}

			// Check if any new problems are added
			for _, p := range payload.Problems {
				_, exists := existingIDs[p.ID]
				if !exists {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "cannot add new problems after the contest has started"})
				}
			}
		} else {
			if startTime.Before(time.Now().Add(-1 * time.Minute)) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "contest start time cannot be in the past"})
			}
		}

		var regDeadline *time.Time
		if strings.TrimSpace(payload.Details.RegistrationDeadline) != "" {
			parsedDead, err := time.Parse(time.RFC3339, payload.Details.RegistrationDeadline)
			if err != nil {
				parsedDead, err = time.Parse("2006-01-02T15:04", payload.Details.RegistrationDeadline)
				if err != nil {
					parsedDead, err = time.ParseInLocation("2006-01-02T15:04", payload.Details.RegistrationDeadline, time.Local)
				}
			}
			if err == nil {
				regDeadline = &parsedDead
			}
		}

		// Convert problems to store.ProblemData
		var problemsData []store.ProblemData
		for _, p := range payload.Problems {
			sampleBotJSON, _ := json.Marshal(p.SampleBotFiles)
			hiddenBotJSON, _ := json.Marshal(p.HiddenBotFiles)

			problemsData = append(problemsData, store.ProblemData{
				ID:                    p.ID,
				Code:                  p.Code,
				Title:                 p.Title,
				Statement:             p.Statement,
				TimeLimit:             p.TimeLimit,
				MemoryLimit:           p.MemoryLimit,
				SampleStrategies:      p.SampleStrategies,
				SampleBotFilesJSON:    string(sampleBotJSON),
				SampleShowCustom:      p.SampleShowCustom,
				SampleTargetInjection: p.SampleTargetInjection,
				SampleProtocol:        p.SampleProtocol,
				SampleTelemetryFormat: p.SampleTelemetryFormat,
				HiddenStrategies:      p.HiddenStrategies,
				HiddenBotFilesJSON:    string(hiddenBotJSON),
				HiddenShowCustom:      p.HiddenShowCustom,
				HiddenTargetInjection: p.HiddenTargetInjection,
				HiddenProtocol:        p.HiddenProtocol,
				HiddenTelemetryFormat: p.HiddenTelemetryFormat,
			})
		}

		// Generate dynamic contest ID if not provided
		contestID := payload.Details.ID
		if strings.TrimSpace(contestID) == "" {
			contestID = fmt.Sprintf("contest-%d-%s", time.Now().Unix(), randomString(4))
		}

		// Extract the authenticated user as the contest creator
		createdBy := ""
		if uid, ok := c.Locals("user_id").(string); ok {
			createdBy = uid
		}

		err = db.PublishContest(c.Context(), contestID,
			payload.Details.Name, payload.Details.Description, payload.Details.Visibility, payload.Details.Code,
			startTime, payload.Details.DurationMinutes, regDeadline, payload.Details.Strategy, payload.Details.FinalStrategies, problemsData, createdBy,
			payload.Details.BannerPreview,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to publish contest", "error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"message":    "Contest published successfully",
			"contest_id": contestID,
		})
	})

	// Contest details route (public view)
	app.Get("/contests/:id/public", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		
		contestID := c.Params("id")
		contest, err := db.GetContest(c.Context(), contestID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "contest not found"})
		}
		// Don't expose private code
		contest.Code = ""
		
		problems, err := db.GetContestProblems(c.Context(), contestID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to get problems", "error": err.Error()})
		}
		
		// Strip hidden strategies and bot files
		for i := range problems {
			problems[i].HiddenStrategies = nil
			problems[i].HiddenBotFilesJSON = ""
		}

		return c.JSON(fiber.Map{
			"details": contest,
			"problems": problems,
		})
	})

	app.Get("/contests/:id/participants", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		contestID := c.Params("id")
		teams, err := db.GetContestRegistrations(c.Context(), contestID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to get registrations", "error": err.Error()})
		}
		return c.JSON(fiber.Map{
			"participants": teams,
		})
	})

	app.Get("/contests/:id/full", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		contestID := c.Params("id")
		contest, err := db.GetContest(c.Context(), contestID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "contest not found", "error": err.Error()})
		}
		requesterID := ""
		if uid, ok := c.Locals("user_id").(string); ok {
			requesterID = uid
		}
		if contest.CreatedBy == "" || requesterID != contest.CreatedBy {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "only the contest host can access full details"})
		}
		problems, err := db.GetContestProblems(c.Context(), contestID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to get problems", "error": err.Error()})
		}
		return c.JSON(fiber.Map{
			"details": contest,
			"problems": problems,
		})
	})

	// --- Delete a contest (host only) ---
	app.Delete("/contests/:id", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}
		contestID := c.Params("id")
		contest, err := db.GetContest(c.Context(), contestID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "contest not found", "error": err.Error()})
		}
		requesterID := ""
		if uid, ok := c.Locals("user_id").(string); ok {
			requesterID = uid
		}
		if contest.CreatedBy == "" || requesterID != contest.CreatedBy {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"message": "only the contest host can delete it"})
		}
		// Block deletion if the contest has already finished (completed, finalizing, or past duration)
		endTime := contest.StartTime.Add(time.Duration(contest.DurationMinutes) * time.Minute)
		if contest.Phase == "completed" || contest.Phase == "finalizing" || time.Now().After(endTime) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "cannot delete contest after it has finished",
			})
		}
		if err := db.DeleteContest(c.Context(), contestID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to delete contest", "error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "Contest deleted successfully"})
	})

	// --- Contest finalization: only the host can trigger post-contest final rounds ---
	app.Post("/contests/:id/finalize", auth.AuthMiddleware(), func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}

		contestID := c.Params("id")
		if strings.TrimSpace(contestID) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "contest ID is required"})
		}

		// Get contest details and verify it has ended
		contest, err := db.GetContest(c.Context(), contestID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "contest not found", "error": err.Error()})
		}

		endTime := contest.StartTime.Add(time.Duration(contest.DurationMinutes) * time.Minute)
		if time.Now().Before(endTime) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "contest has not ended yet",
				"end_time": endTime.Format(time.RFC3339),
			})
		}

		// Only the contest host (creator) can trigger finalization
		requesterID := ""
		if uid, ok := c.Locals("user_id").(string); ok {
			requesterID = uid
		}
		if contest.CreatedBy == "" || requesterID != contest.CreatedBy {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"message": "only the contest host can trigger finalization",
			})
		}

		// Parse optional config from request body
		var body struct {
			BotCount     int `json:"bot_count"`
			DurationSecs int `json:"duration_seconds"`
			RequestCount int `json:"request_count"`
		}
		_ = c.BodyParser(&body)
		if body.BotCount <= 0 {
			body.BotCount = 32
		}
		if body.DurationSecs <= 0 {
			body.DurationSecs = 10
		}

		// Set contest phase to finalizing
		if err := db.UpdateContestPhase(c.Context(), contestID, "finalizing"); err != nil {
			log.Printf("WARNING: failed to set contest phase to finalizing: %v", err)
		}

		// Get all registered teams
		teams, err := db.GetContestRegistrations(c.Context(), contestID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to get registrations", "error": err.Error()})
		}

		// Fallback: If no official registrations, find anyone who submitted code for this contest
		if len(teams) == 0 {
			fallbackTeams, err := db.GetUnregisteredParticipants(c.Context(), contestID)
			if err == nil {
				teams = fallbackTeams
			}
		}

		if len(teams) == 0 {
			// Instead of failing, just complete the contest immediately if absolutely no one submitted
			_ = db.UpdateContestPhase(c.Context(), contestID, "completed")
			return c.JSON(fiber.Map{"message": "No teams submitted. Contest finalized."})
		}

		// NOTE: In a real deployment, each team's sandbox would need to be running.
		// For now, we accept target_url as a parameter per team or use a convention.
		// This is a simplified version that demonstrates the finalization flow.
		log.Printf("[Finalize] Starting finalization for contest %s with %d teams", contestID, len(teams))

		// Build FinalRounds array from chosen strategies per problem
		problems, _ := db.GetContestProblems(c.Context(), contestID)
		var finalRounds []botfleet.FinalRoundDef
		for _, p := range problems {
			probStrategies := p.HiddenStrategies
			if len(probStrategies) == 0 {
				probStrategies = contest.FinalStrategies
			}
			if len(probStrategies) == 0 {
				probStrategies = []string{"bbo_heavy", "flash_crash", "high_cancel", "iceberg", "momentum_burst"}
			}

			for i, strat := range probStrategies {
				seed := int64(0xF10A0000) + int64(i+1)
				finalRounds = append(finalRounds, botfleet.FinalRoundDef{
					Strategy: botfleet.Strategy(strat),
					Seed:     seed,
					Label:    fmt.Sprintf("%s - %s", p.Code, strat),
				})
			}
		}

		// Run finalization in background so the HTTP request doesn't timeout
		go runFinalization(context.Background(), contestID, contest, teams, db, hub, redisClient)

		return c.JSON(fiber.Map{
			"message":    "Finalization started",
			"contest_id": contestID,
			"teams":      len(teams),
			"rounds":     len(finalRounds),
		})
	})

	// --- Contest-specific leaderboard (returns live or final scores) ---
	app.Get("/contests/:id/leaderboard", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}

		contestID := c.Params("id")
		limit, _ := strconv.Atoi(c.Query("limit", "50"))
		if limit <= 0 || limit > 200 {
			limit = 50
		}

		phase := ""
		if db != nil {
			if contest, err := db.GetContest(c.Context(), contestID); err == nil {
				phase = contest.Phase
			}
		}

		finalScores, liveScores, err := db.GetContestLeaderboard(c.Context(), contestID, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "leaderboard query failed", "error": err.Error()})
		}

		if finalScores != nil {
			return c.JSON(fiber.Map{
				"type":        "final",
				"phase":       phase,
				"contest_id":  contestID,
				"leaderboard": finalScores,
			})
		}
		return c.JSON(fiber.Map{
			"type":        "live",
			"phase":       phase,
			"contest_id":  contestID,
			"leaderboard": liveScores,
		})
	})

	// --- Post-contest final results ---
	app.Get("/contests/:id/final-results", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"message": "database not available"})
		}

		contestID := c.Params("id")
		limit, _ := strconv.Atoi(c.Query("limit", "50"))
		if limit <= 0 || limit > 200 {
			limit = 50
		}

		results, err := db.GetContestFinalScores(c.Context(), contestID, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "final results query failed", "error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"contest_id": contestID,
			"results":    results,
		})
	})

	log.Println("Platform API running on port 3001")
	log.Fatal(app.Listen(":3001"))
}

func extractPortFromSourceCode(content []byte) int {
	s := string(content)

	// 1. #define PORT 8080 or #define port 8080
	reDefine := regexp.MustCompile(`(?i)#define\s+ports?\s+(\d{2,5})\b`)
	if m := reDefine.FindStringSubmatch(s); len(m) > 1 {
		if p, err := strconv.Atoi(m[1]); err == nil && p >= 1 && p <= 65535 {
			return p
		}
	}

	// 2. Assignment like port = 8080, port := 8080, let port = 8080, const int port = 8080, PORT = 8080
	reAssign := regexp.MustCompile(`(?i)\bports?\b\s*(?::\s*[a-z0-9_]+)?\s*(::?=|=)\s*(\d{2,5})\b`)
	if m := reAssign.FindStringSubmatch(s); len(m) > 2 {
		if p, err := strconv.Atoi(m[2]); err == nil && p >= 1 && p <= 65535 {
			return p
		}
	}

	// 3. htons(8080)
	reHtons := regexp.MustCompile(`(?i)htons\(\s*(\d{2,5})\s*\)`)
	if m := reHtons.FindStringSubmatch(s); len(m) > 1 {
		if p, err := strconv.Atoi(m[1]); err == nil && p >= 1 && p <= 65535 {
			return p
		}
	}

	// 4. Bind string literals like ":9090" or "0.0.0.0:8080"
	reAddr := regexp.MustCompile(`["'](?:[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})?:(\d{2,5})["']`)
	if m := reAddr.FindStringSubmatch(s); len(m) > 1 {
		if p, err := strconv.Atoi(m[1]); err == nil && p >= 1 && p <= 65535 {
			return p
		}
	}

	return 0
}

func detectProtocolFromSourceCode(content []byte) string {
	s := string(content)
	sLower := strings.ToLower(s)

	// 1. FIX check
	if strings.Contains(sLower, "quickfix") || strings.Contains(sLower, "8=fix") || strings.Contains(sLower, "fix.4.") || strings.Contains(sLower, "35=") {
		return "fix"
	}

	// 2. HTTP check
	if strings.Contains(s, "HTTP/1.1") || strings.Contains(s, "HTTP/1.0") || 
		strings.Contains(sLower, "content-length:") || 
		strings.Contains(sLower, "net/http") || 
		strings.Contains(sLower, "gin-gonic") || 
		strings.Contains(sLower, "gofiber") || 
		strings.Contains(sLower, "crow.h") || 
		strings.Contains(sLower, "httplib.h") || 
		strings.Contains(sLower, "crow_all.h") {
		return "http"
	}

	// 3. TCP check
	if strings.Contains(sLower, "tcp") || 
		strings.Contains(s, "SOCK_STREAM") || 
		strings.Contains(sLower, "net.listen") || 
		strings.Contains(sLower, "tcplistener") || 
		strings.Contains(sLower, "socket(") {
		return "tcp"
	}

	return ""
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

