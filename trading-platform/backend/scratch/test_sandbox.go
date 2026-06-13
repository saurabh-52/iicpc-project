package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Saurabh-52/trading-platform/internal/sandbox"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	fmt.Println("Executing sandbox for tcp_server.go...")
	res, err := sandbox.ExecuteCode(ctx, "d:/IICPC Project/tcp_server.go", "go", 9090, "test")
	if err != nil {
		log.Fatalf("ExecuteCode failed: %v", err)
	}

	fmt.Printf("Execution Result:\n")
	fmt.Printf("PodID: %s\n", res.PodID)
	fmt.Printf("ServiceName: %s\n", res.ServiceName)
	fmt.Printf("Phase: %s\n", res.Phase)
	fmt.Printf("NodePort: %d\n", res.NodePort)
	fmt.Printf("Output: %s\n", res.Output)
}
