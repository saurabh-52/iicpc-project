// backend/cmd/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Saurabh-52/trading-platform/internal/botfleet"
	"github.com/Saurabh-52/trading-platform/internal/sandbox"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
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
	Bots          int    `json:"bots"`
	Requests      int    `json:"requests"`
	DurationSecs  int    `json:"duration_seconds"`
	TimeoutMillis int    `json:"timeout_ms"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	ExpectReply   bool   `json:"expect_reply"`
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
	if result.NodePort <= 0 {
		return ""
	}

	proto := strings.ToLower(strings.TrimSpace(protocol))

	if sandbox.InCluster() {
		host := fmt.Sprintf("%s.trading-sandbox.svc.cluster.local", result.ServiceName)
		if proto == "tcp" {
			return fmt.Sprintf("%s:%d", host, containerPort)
		}
		return fmt.Sprintf("http://%s:%d", host, containerPort)
	}

	// Running locally — try to get minikube IP, fallback to 127.0.0.1
	hostIP := "127.0.0.1"
	if out, err := exec.Command("minikube", "ip").Output(); err == nil {
		ip := strings.TrimSpace(string(out))
		if ip != "" {
			hostIP = ip
		}
	}

	if proto == "tcp" {
		return fmt.Sprintf("%s:%d", hostIP, result.NodePort)
	}
	return fmt.Sprintf("http://%s:%d", hostIP, result.NodePort)
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

	// Health check endpoint for Kubernetes probes
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
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
			result, err := sandbox.ExecuteCode(filePath, language, port)
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
		}

		if executionErr != nil {
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

		// Use a standalone context so that the stress test is not cancelled
		// when the browser navigates away or the HTTP client times out.
		stressCtx, stressCancel := context.WithTimeout(context.Background(), duration+timeout+5*time.Second)
		defer stressCancel()

		metrics, err := botfleet.Run(stressCtx, botfleet.Config{
			Target:      req.Target,
			Protocol:    botfleet.NormalizeProtocol(req.Protocol),
			Strategy:    botfleet.NormalizeStrategy(req.Strategy),
			Bots:        req.Bots,
			Requests:    req.Requests,
			Duration:    duration,
			Timeout:     timeout,
			Method:      req.Method,
			Path:        req.Path,
			ExpectReply: req.ExpectReply,
		})
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "stress test failed", "error": err.Error()})
		}

		return c.JSON(fiber.Map{
			"message": "stress test complete",
			"metrics": metrics,
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
