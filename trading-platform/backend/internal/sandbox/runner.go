// backend/internal/sandbox/runner.go
package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	
	
	// 5. Wait for the container to finish executing
    statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for container: %v", err)
		}
	case <-statusCh:
	}

	// 6. Retrieve the logs (stdout and stderr)
	out, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return fmt.Errorf("failed to get logs: %v", err)
	}
	defer out.Close()

	// 7. Stream logs to your Go console for now
	fmt.Println("--- Contestant Output ---")
	io.Copy(os.Stdout, out)
	fmt.Println("-------------------------")

	// 8. Cleanup: Remove the container after test
	cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})

	return nil


}