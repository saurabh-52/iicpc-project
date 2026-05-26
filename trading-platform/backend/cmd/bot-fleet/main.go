package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/Saurabh-52/trading-platform/internal/botfleet"
)

func main() {
	target := flag.String("target", "http://127.0.0.1:3000/health", "Target URL or TCP address")
	protocol := flag.String("protocol", "http", "Protocol: http or tcp")
	strategy := flag.String("strategy", "bbo_heavy", "Strategy: bbo_heavy, flash_crash, high_cancel, wide_spread")
	bots := flag.Int("bots", 32, "Number of concurrent workers")
	requests := flag.Int("requests", 0, "Total requests to send; 0 uses duration")
	duration := flag.Duration("duration", 10*time.Second, "Load test duration")
	timeout := flag.Duration("timeout", 2*time.Second, "Per-request timeout")
	method := flag.String("method", "POST", "HTTP method for HTTP mode")
	path := flag.String("path", "", "Optional HTTP path override")
	expectReply := flag.Bool("expect-reply", false, "Wait for newline replies in TCP mode")
	flag.Parse()

	ctx := context.Background()
	if *duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duration+5*time.Second)
		defer cancel()
	}

	metrics, err := botfleet.Run(ctx, botfleet.Config{
		Target:      *target,
		Protocol:    botfleet.NormalizeProtocol(*protocol),
		Strategy:    botfleet.NormalizeStrategy(*strategy),
		Bots:        *bots,
		Requests:    *requests,
		Duration:    *duration,
		Timeout:     *timeout,
		Method:      *method,
		Path:        *path,
		ExpectReply: *expectReply,
	})
	if err != nil {
		log.Fatal(err)
	}

	payload, _ := json.MarshalIndent(metrics, "", "  ")
	fmt.Println(string(payload))
}
