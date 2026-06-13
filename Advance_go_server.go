package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
)

// --- 1. Data Structures ---

type Order struct {
	ID        string  `json:"id"`
	Symbol    string  `json:"sym"`
	Side      string  `json:"side"` // "buy" or "sell"
	Price     float64 `json:"price"`
	Qty       int     `json:"qty"`
	Timestamp int64   `json:"-"` // Internal sequencer timestamp
}

// UnmarshalJSON custom parsing to normalise client payloads (supporting both standard client formats and botfleet payloads)
func (o *Order) UnmarshalJSON(data []byte) error {
	type Alias Order
	aux := &struct {
		ID       interface{} `json:"id"`
		Sequence interface{} `json:"sequence"`
		Qty      interface{} `json:"qty"`
		Quantity interface{} `json:"quantity"`
		*Alias
	}{
		Alias: (*Alias)(o),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Resolve ID / Sequence
	if aux.ID != nil {
		o.ID = fmt.Sprintf("%v", aux.ID)
	} else if aux.Sequence != nil {
		o.ID = fmt.Sprintf("%v", aux.Sequence)
	}

	// Resolve Qty / Quantity
	if aux.Qty != nil {
		switch v := aux.Qty.(type) {
		case float64:
			o.Qty = int(v)
		case int:
			o.Qty = v
		}
	} else if aux.Quantity != nil {
		switch v := aux.Quantity.(type) {
		case float64:
			o.Qty = int(v)
		case int:
			o.Qty = v
		}
	}

	// Normalise side to lowercase
	o.Side = strings.ToLower(strings.TrimSpace(o.Side))

	// Set default symbol if empty
	if o.Symbol == "" {
		o.Symbol = "AAPL"
	}

	return nil
}

type OrderBook struct {
	mu           sync.Mutex
	Symbol       string
	Bids         []Order // Buyers (Sorted: Highest Price first, then Oldest Time)
	Asks         []Order // Sellers (Sorted: Lowest Price first, then Oldest Time)
	nextSequence uint64
}

// --- 2. The Matching Engine ---

// ProcessOrder handles the Price-Time FIFO matching logic
func (ob *OrderBook) ProcessOrder(incoming Order) (float64, float64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	fmt.Printf("\n[ENGINE] Processing: %s %d @ $%.2f\n", incoming.Side, incoming.Qty, incoming.Price)
	ob.nextSequence++
	incoming.Timestamp = int64(ob.nextSequence)
	qtyRemaining := incoming.Qty

	if incoming.Side == "buy" {
		// Match against Asks
		for len(ob.Asks) > 0 && qtyRemaining > 0 {
			bestAsk := &ob.Asks[0]
			if incoming.Price < bestAsk.Price {
				break // Spread not crossed
			}

			tradeQty := min(qtyRemaining, bestAsk.Qty)
			fmt.Printf("   -> [TRADE] %d shares of %s @ $%.2f\n", tradeQty, ob.Symbol, bestAsk.Price)

			qtyRemaining -= tradeQty
			bestAsk.Qty -= tradeQty

			// If the resting ask is fully filled, remove it
			if bestAsk.Qty == 0 {
				ob.Asks = ob.Asks[1:]
			}
		}
		// If the buy order still has shares left, add it to the book
		if qtyRemaining > 0 {
			incoming.Qty = qtyRemaining
			ob.Bids = append(ob.Bids, incoming)
			// Sort Bids: Price DESC, Time ASC
			sort.SliceStable(ob.Bids, func(i, j int) bool {
				if ob.Bids[i].Price == ob.Bids[j].Price {
					return ob.Bids[i].Timestamp < ob.Bids[j].Timestamp // Time Priority
				}
				return ob.Bids[i].Price > ob.Bids[j].Price // Price Priority
			})
		}

	} else if incoming.Side == "sell" {
		// Match against Bids
		for len(ob.Bids) > 0 && qtyRemaining > 0 {
			bestBid := &ob.Bids[0]
			if incoming.Price > bestBid.Price {
				break // Spread not crossed
			}

			tradeQty := min(qtyRemaining, bestBid.Qty)
			fmt.Printf("   -> [TRADE] %d shares of %s @ $%.2f\n", tradeQty, ob.Symbol, bestBid.Price)

			qtyRemaining -= tradeQty
			bestBid.Qty -= tradeQty

			// If resting bid is fully filled, remove it
			if bestBid.Qty == 0 {
				ob.Bids = ob.Bids[1:]
			}
		}
		// If the sell order still has shares left, add it to the book
		if qtyRemaining > 0 {
			incoming.Qty = qtyRemaining
			ob.Asks = append(ob.Asks, incoming)
			// Sort Asks: Price ASC, Time ASC
			sort.SliceStable(ob.Asks, func(i, j int) bool {
				if ob.Asks[i].Price == ob.Asks[j].Price {
					return ob.Asks[i].Timestamp < ob.Asks[j].Timestamp // Time Priority
				}
				return ob.Asks[i].Price < ob.Asks[j].Price // Price Priority
			})
		}
	}

	var bestBid, bestAsk float64
	if len(ob.Bids) > 0 {
		bestBid = ob.Bids[0].Price
	}
	if len(ob.Asks) > 0 {
		bestAsk = ob.Asks[0].Price
	}
	return bestBid, bestAsk
}

// BBO returns the current best bid and best ask prices
func (ob *OrderBook) BBO() (float64, float64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	var bestBid, bestAsk float64
	if len(ob.Bids) > 0 {
		bestBid = ob.Bids[0].Price
	}
	if len(ob.Asks) > 0 {
		bestAsk = ob.Asks[0].Price
	}
	return bestBid, bestAsk
}

// --- 3. Networking & Setup ---

func main() {
	// Create a simple single-symbol orderbook for testing
	book := &OrderBook{Symbol: "AAPL", Bids: []Order{}, Asks: []Order{}}

	// Start the TCP Server on port 9090
	ln, err := net.Listen("tcp", ":9090")
	if err != nil {
		fmt.Println("listen error:", err)
		return
	}
	fmt.Println("Matching Engine running on TCP :9090")

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		// Handle each client concurrently
		go handleClient(conn, book)
	}
}

func handleClient(c net.Conn, book *OrderBook) {
	defer c.Close()
	scanner := bufio.NewScanner(c)

	for scanner.Scan() {
		line := scanner.Bytes()
		var incomingOrder Order

		err := json.Unmarshal(line, &incomingOrder)
		if err != nil {
			c.Write([]byte(`{"status":"error", "msg":"invalid json"}` + "\n"))
			continue
		}

		// Process the order synchronously and safely under a single lock
		bestBid, bestAsk := book.ProcessOrder(incomingOrder)

		// Send acceptance notification with correctness fields
		resp := fmt.Sprintf(`{"status":"accepted","id":"%s","best_bid":%.6f,"best_ask":%.6f}`+"\n",
			incomingOrder.ID, bestBid, bestAsk)
		c.Write([]byte(resp))
	}
}

// Helper math function for Go
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
