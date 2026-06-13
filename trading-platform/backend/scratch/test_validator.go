package main

import (
	"fmt"

	"github.com/Saurabh-52/trading-platform/internal/telemetry"
	"github.com/Saurabh-52/trading-platform/internal/validator"
)

func main() {
	events := []telemetry.TelemetryEvent{
		{Sequence: 370, Action: "NEW", Side: "BUY", Price: 99.6654, Quantity: 117},
		{Sequence: 379, Action: "NEW", Side: "SELL", Price: 99.9786, Quantity: 159},
		{Sequence: 374, Action: "NEW", Side: "SELL", Price: 100.1424, Quantity: 94},
		{Sequence: 371, Action: "NEW", Side: "BUY", Price: 100.0117, Quantity: 72},
		{Sequence: 378, Action: "NEW", Side: "SELL", Price: 100.1416, Quantity: 153},
		{Sequence: 430, Action: "NEW", Side: "BUY", Price: 100.2977, Quantity: 165},
		{Sequence: 424, Action: "NEW", Side: "BUY", Price: 100.1848, Quantity: 32},
		{Sequence: 402, Action: "NEW", Side: "SELL", Price: 99.6584, Quantity: 54},
		{Sequence: 403, Action: "NEW", Side: "SELL", Price: 99.6296, Quantity: 131},
		{Sequence: 414, Action: "NEW", Side: "BUY", Price: 100.1031, Quantity: 30},
	}

	ob := &validator.Orderbook{}

	for i, e := range events {
		fmt.Printf("\n--- Step %d: %s %s %.4f Qty %d ---\n", i+1, e.Action, e.Side, e.Price, e.Quantity)
		if e.Action == "NEW" {
			ob.AddOrder(e.Side, e.Price, e.Quantity)
		} else if e.Action == "CANCEL" {
			ob.CancelOrder(e.Side, e.Price, e.Quantity)
		}

		fmt.Printf("Book state:\n  Bids: %v\n  Asks: %v\n", ob.Bids, ob.Asks)
		if err := ob.ValidateCross(); err != nil {
			fmt.Printf("CROSS ERROR: %v\n", err)
		} else {
			fmt.Printf("ValidateCross: OK\n")
		}
	}
}
