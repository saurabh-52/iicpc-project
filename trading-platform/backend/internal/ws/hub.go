package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

// Hub manages a set of active WebSocket connections and broadcasts messages
// to all of them simultaneously.
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]struct{}),
	}
}

// Register adds a connection to the hub.
func (h *Hub) Register(c *websocket.Conn) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	log.Printf("WS: client connected (%d total)", h.Count())
}

// Unregister removes a connection from the hub.
func (h *Hub) Unregister(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	log.Printf("WS: client disconnected (%d remaining)", h.Count())
}

// Count returns the number of connected clients.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast sends a JSON message to all connected clients.
// Connections that fail to write are automatically unregistered.
func (h *Hub) Broadcast(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("WS: marshal error: %v", err)
		return
	}

	h.mu.RLock()
	targets := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	var dead []*websocket.Conn
	for _, c := range targets {
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			dead = append(dead, c)
		}
	}

	if len(dead) > 0 {
		h.mu.Lock()
		for _, c := range dead {
			delete(h.clients, c)
			_ = c.Close()
		}
		h.mu.Unlock()
	}
}

// LeaderboardUpdate is the JSON payload broadcast to clients.
type LeaderboardUpdate struct {
	Type    string      `json:"type"`    // "leaderboard_update"
	Payload interface{} `json:"payload"` // []store.SubmissionResult
}

// FinalizationProgressUpdate is the JSON payload broadcast to clients for contest finalization.
type FinalizationProgressUpdate struct {
	Type     string `json:"type"`       // "finalization_progress"
	Contest  string `json:"contest_id"` // Contest ID
	Progress int    `json:"progress"`   // Progress percentage 0-100
}

// HandleConnection runs the WebSocket read loop for a single connection.
// It keeps the connection alive until the client disconnects.
func HandleConnection(hub *Hub, c *websocket.Conn) {
	hub.Register(c)
	defer hub.Unregister(c)

	// Read loop — we only need to detect disconnects.
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			break
		}
	}
}
