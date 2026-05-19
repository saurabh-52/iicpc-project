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

	"github.com/Saurabh-52/trading-platform/internal/sandbox"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

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

func main() {
	app := fiber.New()

	// Enable CORS for frontend communication
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173, http://localhost:3000",
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

		// Execute sandbox with timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
		defer cancel()

		executionResult := sandbox.ExecutionResult{}
		executionErr := error(nil)

		// Run in separate goroutine so we can timeout
		done := make(chan bool, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("PANIC in sandbox execution:", r)
					executionErr = fmt.Errorf("sandbox panic: %v", r)
				}
				done <- true
			}()
			var err error
			executionResult, err = sandbox.ExecuteCode(filePath, language, port)
			if err != nil {
				fmt.Println("SANDBOX ERROR:", err)
				executionErr = err
			}
		}()

		// Wait for completion or timeout
		select {
		case <-done:
			// Execution completed
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

		fmt.Println("✓ Submission processed successfully, returning JSON response")
		response := fiber.Map{
			"message": "Submission processed",
			"form_data": fiber.Map{
				"language":             language,
				"port":                 port,
				"original_filename":    file.Filename,
				"stored_filename":      submissionName,
				"stored_relative_path": filePath,
			},
			"execution_result": fiber.Map{
				"pod_id":       executionResult.PodID,
				"service_name": executionResult.ServiceName,
				"phase":        executionResult.Phase,
				"output":       executionResult.Output,
			},
		}

		if payload, marshalErr := json.Marshal(response); marshalErr == nil {
			fmt.Println("RESPONSE:", string(payload))
		}

		return c.JSON(response)
	})

	log.Println("Platform API running on port 3000")
	log.Fatal(app.Listen(":3000"))
}
