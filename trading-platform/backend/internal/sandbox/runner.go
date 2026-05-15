// backend/internal/sandbox/runner.go
package sandbox

import (
	"context"
	"fmt"
	"path/filepath" // <--- Add this import

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func ExecuteCode(filePath string, language string) error {
	ctx := context.Background()
	
	// 1. Convert to Absolute Path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to connect to Docker: %v", err)
	}

	containerConfig := &container.Config{
		Image:           "gcc:latest",
		NetworkDisabled: true,
		Cmd:             []string{"sh", "-c", "g++ /app/code.cpp -o /app/run && /app/run"},
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			// Use absPath instead of filePath
			fmt.Sprintf("%s:/app/code.cpp", absPath), 
		},
		Resources: container.Resources{
			Memory:   512 * 1024 * 1024,
			NanoCPUs: 1000000000,
		},
	}

    // ... rest of your code (Create and Start)
    resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create sandbox: %v", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start sandbox: %v", err)
	}

	fmt.Printf("Sandbox started successfully with ID: %s\n", resp.ID)
	return nil
}