package telemetry

import (
	"strconv"
	"time"
)

// TelemetryEvent captures one interaction between a bot and the contestant's engine.
type TelemetryEvent struct {
	SubmissionID string    `json:"submission_id"`
	BotID        int       `json:"bot_id"`
	Sequence     int       `json:"sequence"`
	Action       string    `json:"action"`       // NEW | CANCEL | LOG
	Side         string    `json:"side"`         // BUY | SELL | ""
	Price        float64   `json:"price"`
	Quantity     int       `json:"quantity"`
	StatusCode   int       `json:"status_code"`  // HTTP status; 0 for TCP/FIX
	LatencyMs    float64   `json:"latency_ms"`
	Timestamp    time.Time `json:"timestamp"`
	EngineOutput string    `json:"engine_output"` // truncated to 512 bytes
}

// ToMap converts the event to a flat string map for XADD.
func (e TelemetryEvent) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"submission_id": e.SubmissionID,
		"bot_id":        strconv.Itoa(e.BotID),
		"sequence":      strconv.Itoa(e.Sequence),
		"action":        e.Action,
		"side":          e.Side,
		"price":         strconv.FormatFloat(e.Price, 'f', -1, 64),
		"quantity":      strconv.Itoa(e.Quantity),
		"status_code":   strconv.Itoa(e.StatusCode),
		"latency_ms":    strconv.FormatFloat(e.LatencyMs, 'f', -1, 64),
		"timestamp":     e.Timestamp.UTC().Format(time.RFC3339Nano),
		"engine_output": e.EngineOutput,
	}
}

// FromMap reconstructs a TelemetryEvent from a Redis Stream entry map.
func FromMap(m map[string]string) TelemetryEvent {
	botID, _ := strconv.Atoi(m["bot_id"])
	seq, _ := strconv.Atoi(m["sequence"])
	qty, _ := strconv.Atoi(m["quantity"])
	status, _ := strconv.Atoi(m["status_code"])
	price, _ := strconv.ParseFloat(m["price"], 64)
	latency, _ := strconv.ParseFloat(m["latency_ms"], 64)
	
	ts, err := time.Parse(time.RFC3339Nano, m["timestamp"])
	if err != nil {
		ts = time.Now().UTC() // Fallback to avoid Year 1 window corruption
	}

	return TelemetryEvent{
		SubmissionID: m["submission_id"],
		BotID:        botID,
		Sequence:     seq,
		Action:       m["action"],
		Side:         m["side"],
		Price:        price,
		Quantity:     qty,
		StatusCode:   status,
		LatencyMs:    latency,
		Timestamp:    ts,
		EngineOutput: m["engine_output"],
	}
}

// Truncate512 trims s to at most 512 bytes while strictly preserving UTF-8 boundaries.
func Truncate512(s string) string {
	if len(s) <= 512 {
		return s
	}
	// Iterate over runes to find the last valid boundary before 512 bytes
	var validLen int
	for i := range s {
		if i > 512 {
			break
		}
		validLen = i
	}
	return s[:validLen]
}
