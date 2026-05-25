// backend/cmd/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"github.com/Saurabh-52/trading-platform/internal/sandbox"
	"github.com/gofiber/fiber/v2"
)

func main() {
	app := fiber.New()

	// Ensure our temporary workspace exists
	os.MkdirAll("./workspace", os.ModePerm)

	// The endpoint where contestants submit their code
	app.Post("/submit", func(c *fiber.Ctx) error {
    file, _ := c.FormFile("source_code")
    filePath := fmt.Sprintf("./workspace/%s", file.Filename)
    c.SaveFile(file, filePath)

    // Log what's happening
    fmt.Println("Attempting to start sandbox for:", filePath)

    err := sandbox.ExecuteCode(filePath, "cpp")
    if err != nil {
        fmt.Println("SANDBOX ERROR:", err) // This will show in your terminal
        return c.Status(500).SendString(err.Error())
    }

    return c.JSON(fiber.Map{"message": "Container created and running!"})
})

	log.Println("Platform API running on port 3000")
	log.Fatal(app.Listen(":3000"))
}