// backend/cmd/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

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
}

func submissionNameForLanguage(language string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "cpp", "c++", "cc", "cxx":
		return "main.cpp", nil
	case "go":
		return "main.go", nil
	case "rust":
		return "main.rs", nil
	case "python", "py":
		return "main.py", nil
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
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
// when running locally it retrieves the minikube IP or falls back to 127.0.0.1.
func buildTargetURL(result sandbox.ExecutionResult, protocol string, containerPort int) string {
	proto := strings.ToLower(strings.TrimSpace(protocol))

	if sandbox.InCluster() {
		host := fmt.Sprintf("%s.trading-sandbox.svc.cluster.local", result.ServiceName)
		if proto == "tcp" {
			return fmt.Sprintf("%s:%d", host, containerPort)
		}
		return fmt.Sprintf("http://%s:%d", host, containerPort)
	}

	// Running locally on macOS with minikube (Docker driver) + minikube tunnel.
	// LoadBalancer services are mapped to 127.0.0.1:<containerPort> by the tunnel.
	// The sandbox runner auto-cleans old services so there's only one active
	// LoadBalancer per containerPort, avoiding port collisions.
	hostIP := "127.0.0.1"

	if proto == "tcp" {
		return fmt.Sprintf("%s:%d", hostIP, containerPort)
	}
	return fmt.Sprintf("http://%s:%d", hostIP, containerPort)
}

func main() {
	app := fiber.New()

	// Enable CORS for frontend communication
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173, http://localhost:5174, http://localhost:3000",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Content-Type,Authorization",
	}))

	// Ensure our temporary workspace exists
	os.MkdirAll("./workspace", os.ModePerm)

	// --- Redis (optional — telemetry is best-effort) ---
	var redisClient *redis.Client
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	rc, err := telemetry.NewRedisClient(redisAddr)
	if err != nil {
		log.Printf("WARNING: Redis not available (%s) — telemetry disabled: %v", redisAddr, err)
	} else {
		redisClient = rc.Underlying()
		log.Printf("Redis connected at %s", redisAddr)
	}

	// --- Postgres (optional — scoring persistence is best-effort) ---
	var db *store.Store
	pgURL := os.Getenv("POSTGRES_URL")
	if pgURL == "" {
		pgURL = "postgres://user:password@localhost:5432/postgres?sslmode=disable"
	}
	dbConn, err := store.NewStore(context.Background(), pgURL)
	if err != nil {
		log.Printf("WARNING: PostgreSQL not available — leaderboard disabled: %v", err)
	} else {
		db = dbConn
		log.Println("PostgreSQL connected and migrated")
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
	app.Post("/submit", func(c *fiber.Ctx) error {
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

		submissionName, err := submissionNameForLanguage(language)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
		}

		if !extensionAllowedForLanguage(file.Filename, language) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": fmt.Sprintf("file extension %s does not match selected language %s", filepath.Ext(file.Filename), language)})
		}

		filePath := filepath.Join("./workspace", submissionName)
		if err := c.SaveFile(file, filePath); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to save uploaded file"})
		}

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
			result, err := sandbox.ExecuteCode(ctx, filePath, language, port)
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

	app.Post("/stress-test", func(c *fiber.Ctx) error {
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

		// --- Helper: run one stress-test round and score it ---
		type roundResult struct {
			SubmissionID string          `json:"submission_id"`
			Strategy     string          `json:"strategy"`
			Metrics      botfleet.Summary `json:"metrics"`
			Score        *scorer.Score   `json:"score,omitempty"`
		}
		runRound := func(strategy botfleet.Strategy, idSuffix string) (roundResult, error) {
			submissionID := fmt.Sprintf("stress-%d%s", time.Now().UnixNano(), idSuffix)

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
			})
			if err != nil {
				return roundResult{}, err
			}

			rr := roundResult{
				SubmissionID: submissionID,
				Strategy:     string(strategy),
				Metrics:      metrics,
			}

			// Scoring pipeline (best-effort)
			if redisClient != nil {
				time.Sleep(200 * time.Millisecond)
				scoreCtx, scoreCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer scoreCancel()
				events, consumeErr := telemetry.ConsumeAllForSubmission(scoreCtx, redisClient, submissionID)
				if consumeErr == nil && len(events) > 0 {
					perfMetrics := scorer.ComputeMetrics(submissionID, events)
					valResult := validator.RunValidatorFromEvents(submissionID, events)
					sc := scorer.ComputeScore(perfMetrics, valResult)
					rr.Score = &sc

					if db != nil {
						sr := store.NewSubmissionResult(submissionID, systemName, string(strategy), "", sc, perfMetrics, valResult)
						if storeErr := db.CreateSubmissionResult(scoreCtx, sr); storeErr != nil {
							log.Printf("WARNING: failed to persist scoring result for %s: %v", strategy, storeErr)
						}
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

	log.Println("Platform API running on port 3000")
	log.Fatal(app.Listen(":3000"))
}
