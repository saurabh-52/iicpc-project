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
	"strconv"
	"strings"
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
		if proto == "tcp" {
			return fmt.Sprintf("%s:%d", host, containerPort)
		}
		return fmt.Sprintf("http://%s:%d", host, containerPort)
	}

	// Running locally outside cluster: try to get minikube IP, fallback to 127.0.0.1.
	// Use NodePort if available to support parallel executions, fallback to containerPort.
	hostIP := "127.0.0.1"
	if minikubeIP := os.Getenv("MINIKUBE_IP"); minikubeIP != "" {
		hostIP = minikubeIP
	}

	portToUse := containerPort
	// If minikube IP is explicitly provided, use NodePort since that's how we reach it.
	// Otherwise, assume local development via minikube tunnel, which exposes the LoadBalancer on 127.0.0.1:containerPort
	if result.NodePort > 0 && hostIP != "127.0.0.1" {
		portToUse = int(result.NodePort)
	}

	if proto == "tcp" {
		return fmt.Sprintf("%s:%d", hostIP, portToUse)
	}
	return fmt.Sprintf("http://%s:%d", hostIP, portToUse)
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

	app := fiber.New()

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
		results, err := db.GetUserHistory(c.Context(), userID, limit)
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

		fmt.Println("Attempting to start sandbox for:", filePath)

		// Execute sandbox with timeout context — increased to 75s because
		// waitForPodReady can take up to 60s for compilation + startup.
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
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
			SubmissionID string          `json:"submission_id"`
			Strategy     string          `json:"strategy"`
			JudgingMode  string          `json:"judging_mode"`
			SeedUsed     int64           `json:"seed_used"`
			Metrics      botfleet.Summary `json:"metrics"`
			Score        *scorer.Score   `json:"score,omitempty"`
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
				hasScored := false

				if consumeErr == nil && len(events) > 0 {
					perfMetrics = scorer.ComputeMetrics(submissionID, events)
					valResult = validator.RunValidatorFromEvents(submissionID, events)
					sc = scorer.ComputeScore(perfMetrics, valResult)
					rr.Score = &sc
					hasScored = true
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
					hasScored = true
				}

				if hasScored && db != nil {
					// Extract user_id from the request context (set by auth middleware)
					submitUserID := ""
					if uid, ok := c.Locals("user_id").(string); ok {
						submitUserID = uid
					}
					sr := store.NewSubmissionResultWithMode(
						submissionID, systemName, string(strategy), "", submitUserID,
						sc, perfMetrics, valResult,
						string(judgingMode), req.ContestID, nil, seed,
					)
					if storeErr := db.CreateSubmissionResult(scoreCtx, sr); storeErr != nil {
						log.Printf("WARNING: failed to persist scoring result for %s: %v", strategy, storeErr)
					}
				}
			}

			return rr, nil
		}

		// --- Run the user's selected strategy ---
		log.Printf("Running stress test: strategy=%s system_name=%q", userStrategy, systemName)
		userRound, err := runRound(userStrategy, "")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "stress test failed", "error": err.Error()})
		}

		rounds := []roundResult{userRound}

		// --- If user did NOT pick bbo_heavy, also run the default baseline ---
		if userStrategy != botfleet.StrategyBBOHeavy {
			log.Printf("Running baseline bbo_heavy stress test for %q", systemName)
			bboRound, err := runRound(botfleet.StrategyBBOHeavy, "-bbo")
			if err != nil {
				log.Printf("WARNING: baseline bbo_heavy run failed: %v", err)
			} else {
				rounds = append(rounds, bboRound)
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

	// Contest registration route
	app.Post("/contests/:id/register", func(c *fiber.Ctx) error {
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

		err := db.RegisterSystemForContest(c.Context(), contestID, payload.SystemName, payload.Code)
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

	// Contest Draft saving route
	app.Post("/contests/draft", func(c *fiber.Ctx) error {
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

	// Contest Publishing route
	app.Post("/contests/publish", func(c *fiber.Ctx) error {
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

		err = db.PublishContest(c.Context(), contestID,
			payload.Details.Name, payload.Details.Description, payload.Details.Visibility, payload.Details.Code,
			startTime, payload.Details.DurationMinutes, regDeadline, payload.Details.Strategy, payload.Details.FinalStrategies, problemsData,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to publish contest", "error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"message":    "Contest published successfully",
			"contest_id": contestID,
		})
	})

	// --- Contest finalization: admin triggers post-contest final rounds ---
	app.Post("/contests/:id/finalize", func(c *fiber.Ctx) error {
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

		if len(teams) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "no teams registered for this contest"})
		}

		// NOTE: In a real deployment, each team's sandbox would need to be running.
		// For now, we accept target_url as a parameter per team or use a convention.
		// This is a simplified version that demonstrates the finalization flow.
		log.Printf("[Finalize] Starting finalization for contest %s with %d teams", contestID, len(teams))

		// Build FinalRounds array from contest.FinalStrategies
		var finalRounds []botfleet.FinalRoundDef
		if len(contest.FinalStrategies) > 0 {
			for i, strat := range contest.FinalStrategies {
				// Deterministic but strategy-specific seed
				seed := int64(0xF10A0000) + int64(i+1)
				finalRounds = append(finalRounds, botfleet.FinalRoundDef{
					Strategy: botfleet.Strategy(strat),
					Seed:     seed,
					Label:    strat,
				})
			}
		} else {
			finalRounds = botfleet.DefaultFinalRounds
		}

		// Run finalization in background so the HTTP request doesn't timeout
		go func() {
			ctx := context.Background()
			frc := botfleet.FinalRoundConfig{
				ContestID:    contestID,
				Rounds:       finalRounds,
				BotCount:     body.BotCount,
				RequestCount: body.RequestCount,
				Duration:     time.Duration(body.DurationSecs) * time.Second,
				Timeout:      2 * time.Second,
				RedisClient:  redisClient,
			}

			// For each team, we would need their running sandbox target URL.
			// In practice this comes from the sandbox service registry.
			// For now we log the intent — the actual execution requires sandbox orchestration.
			for _, teamName := range teams {
				log.Printf("[Finalize] Would evaluate team %s for contest %s (requires sandbox target)", teamName, contestID)
				// When sandbox URLs are available:
				// result, err := botfleet.RunFinalRounds(ctx, frc, botfleet.FinalSubmission{...})
				// Then persist: db.SaveFinalScore(...)
			}

			// Mark as completed
			if err := db.UpdateContestPhase(ctx, contestID, "completed"); err != nil {
				log.Printf("WARNING: failed to set contest phase to completed: %v", err)
			}

			// Broadcast updated leaderboard
			if finalScores, err := db.GetContestFinalScores(ctx, contestID, 50); err == nil {
				hub.Broadcast(ws.LeaderboardUpdate{
					Type:    "contest_finalized",
					Payload: finalScores,
				})
			}

			_ = frc // suppress unused warning until sandbox URLs are wired
			log.Printf("[Finalize] Finalization complete for contest %s", contestID)
		}()

		return c.JSON(fiber.Map{
			"message":    "Finalization started",
			"contest_id": contestID,
			"teams":      len(teams),
			"rounds":     len(botfleet.DefaultFinalRounds),
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

		finalScores, liveScores, err := db.GetContestLeaderboard(c.Context(), contestID, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "leaderboard query failed", "error": err.Error()})
		}

		if finalScores != nil {
			return c.JSON(fiber.Map{
				"type":        "final",
				"contest_id":  contestID,
				"leaderboard": finalScores,
			})
		}
		return c.JSON(fiber.Map{
			"type":        "live",
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
