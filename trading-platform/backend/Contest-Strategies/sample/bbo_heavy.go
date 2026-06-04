package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Order struct {
	BotID         int       `json:"bot_id"`
	Sequence      int       `json:"sequence"`
	Strategy      string    `json:"strategy"`
	Action        string    `json:"action"`
	Side          string    `json:"side"`
	Price         float64   `json:"price"`
	Quantity      int       `json:"quantity"`
	Spread        float64   `json:"spread"`
	Cancel        bool      `json:"cancel"`
	TotalQuantity int       `json:"total_quantity,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type TelemetryRecord struct {
	Timestamp string  `json:"timestamp"`
	LatencyMs float64 `json:"latency_ms"`
	Success   bool    `json:"success"`
	Side      string  `json:"side"`
}

func orderToFIX(o Order) string {
	msgType := "D"
	if o.Cancel {
		msgType = "F"
	}
	side := "1"
	if o.Side == "SELL" {
		side = "2"
	}
	return fmt.Sprintf("35=%s|49=BOT%%d|11=%%d|54=%%s|55=SYM|44=%%.2f|38=%%d|40=2|10=000|",
		msgType, o.BotID, o.Sequence, side, o.Price, o.Quantity)
}

func generateOrder(botID int, sequence int, rng *rand.Rand) Order {
	baseMid := 100.0
	side := "BUY"
	if rng.Intn(2) == 0 {
		side = "SELL"
	}
	price := baseMid + rng.Float64()*0.8 - 0.4
	qty := 25 + rng.Intn(150)
	spread := 0.15 + rng.Float64()*0.35
	return Order{
		BotID:     botID,
		Sequence:  sequence,
		Strategy:  "bbo_heavy",
		Side:      side,
		Price:     price,
		Quantity:  qty,
		Spread:    spread,
		CreatedAt: time.Now().UTC(),
	}
}

func main() {
	targetFlag := flag.String("target", "", "Target engine address (IP:Port or URL)")
	protoFlag := flag.String("protocol", "http", "Protocol: http, tcp, fix")
	botsFlag := flag.Int("bots", 16, "Number of concurrent bots")
	durationFlag := flag.Duration("duration", 10*time.Second, "Test duration")
	requestsFlag := flag.Int("requests", 0, "Limit total requests")
	flag.Parse()

	target := *targetFlag
	if target == "" {
		target = os.Getenv("TARGET_URL")
	}
	if target == "" {
		log.Fatal("target address is required (pass --target or set TARGET_URL env var)")
	}

	protocol := strings.ToLower(*protoFlag)
	if envProto := os.Getenv("PROTOCOL"); envProto != "" {
		protocol = strings.ToLower(envProto)
	}

	bots := *botsFlag
	duration := *durationFlag
	requests := *requestsFlag

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var sentCount int64
	var wg sync.WaitGroup

	for botID := 0; botID < bots; botID++ {
		wg.Add(1)
		go func(bid int) {
			defer wg.Done()
			runWorker(ctx, target, protocol, bid, 0xdeadbeef, requests, 2*time.Second, &sentCount)
		}(botID)
	}

	wg.Wait()
}

func runWorker(ctx context.Context, target string, protocol string, botID int, seed int64, requestsLimit int, timeout time.Duration, sentCount *int64) {
	rng := rand.New(rand.NewSource(seed + int64(botID)*7919))
	seq := 0

	var conn net.Conn
	var reader *bufio.Reader
	var writer *bufio.Writer
	var err error

	if protocol == "tcp" || protocol == "fix" {
		dialer := net.Dialer{Timeout: timeout}
		cleanTarget := strings.TrimPrefix(target, "http://")
		cleanTarget = strings.TrimPrefix(cleanTarget, "https://")
		conn, err = dialer.DialContext(ctx, "tcp", cleanTarget)
		if err != nil {
			log.Printf("worker %%d: dial failed: %%v", botID, err)
			return
		}
		defer conn.Close()
		reader = bufio.NewReader(conn)
		writer = bufio.NewWriter(conn)
	}

	httpClient := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	defer httpClient.CloseIdleConnections()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if requestsLimit > 0 {
			current := atomic.AddInt64(sentCount, 1)
			if current > int64(requestsLimit) {
				return
			}
		}

		seq++
		order := generateOrder(botID, seq, rng)
		
		started := time.Now()
		var success bool

		if protocol == "tcp" {
			payload, _ := json.Marshal(order)
			_ = conn.SetWriteDeadline(time.Now().Add(timeout))
			_, err = writer.Write(append(payload, '\n'))
			if err == nil {
				err = writer.Flush()
			}
			if err == nil {
				_ = conn.SetReadDeadline(time.Now().Add(timeout))
				_, err = reader.ReadBytes('\n')
			}
			success = (err == nil)
		} else if protocol == "fix" {
			fixMsg := orderToFIX(order)
			_ = conn.SetWriteDeadline(time.Now().Add(timeout))
			_, err = writer.WriteString(fixMsg + "\n")
			if err == nil {
				err = writer.Flush()
			}
			if err == nil {
				_ = conn.SetReadDeadline(time.Now().Add(timeout))
				_, err = reader.ReadBytes('\n')
			}
			success = (err == nil)
		} else { // http
			payload, _ := json.Marshal(order)
			urlTarget := target
			if !strings.HasPrefix(urlTarget, "http://") && !strings.HasPrefix(urlTarget, "https://") {
				urlTarget = "http://" + urlTarget
			}
			req, err := http.NewRequestWithContext(ctx, "POST", urlTarget, bytes.NewReader(payload))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				resp, err := httpClient.Do(req)
				if err == nil {
					success = (resp.StatusCode >= 200 && resp.StatusCode < 300)
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()
				}
			}
		}

		latency := time.Since(started)

		rec := TelemetryRecord{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			LatencyMs: float64(latency.Nanoseconds()) / 1e6,
			Success:   success,
			Side:      order.Side,
		}
		telemetryPayload, _ := json.Marshal(rec)
		fmt.Println(string(telemetryPayload))

		time.Sleep(10 * time.Millisecond)
	}
}
