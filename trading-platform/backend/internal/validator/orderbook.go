package validator

import (
	"fmt"
	"sort"
)

// Level represents one price level in the orderbook.
type Level struct {
	Price    float64
	TotalQty int
}

// Orderbook maintains a sorted, in-memory view of bids (desc) and asks (asc).
type Orderbook struct {
	Bids []Level // sorted descending (highest bid first)
	Asks []Level // sorted ascending (lowest ask first)
}

// AddOrder inserts or increases quantity at the given price level on the
// appropriate side.  side must be "BUY" or "SELL".
func (ob *Orderbook) AddOrder(side string, price float64, qty int) {
	if side == "BUY" {
		ob.Bids = upsertLevel(ob.Bids, price, qty)
		sort.Slice(ob.Bids, func(i, j int) bool { return ob.Bids[i].Price > ob.Bids[j].Price })
	} else {
		ob.Asks = upsertLevel(ob.Asks, price, qty)
		sort.Slice(ob.Asks, func(i, j int) bool { return ob.Asks[i].Price < ob.Asks[j].Price })
	}
}

// CancelOrder reduces quantity at the given price level, removing the level
// when quantity reaches zero.
func (ob *Orderbook) CancelOrder(side string, price float64, qty int) {
	if side == "BUY" {
		ob.Bids = removeQty(ob.Bids, price, qty)
	} else {
		ob.Asks = removeQty(ob.Asks, price, qty)
	}
}

// BestBid returns the highest bid price and whether any bids exist.
func (ob *Orderbook) BestBid() (float64, bool) {
	if len(ob.Bids) == 0 {
		return 0, false
	}
	return ob.Bids[0].Price, true
}

// BestAsk returns the lowest ask price and whether any asks exist.
func (ob *Orderbook) BestAsk() (float64, bool) {
	if len(ob.Asks) == 0 {
		return 0, false
	}
	return ob.Asks[0].Price, true
}

// ValidateCross returns an error if the book is crossed (best bid >= best ask),
// which indicates incorrect price-time priority implementation.
func (ob *Orderbook) ValidateCross() error {
	bid, hasBid := ob.BestBid()
	ask, hasAsk := ob.BestAsk()
	if hasBid && hasAsk && bid >= ask {
		return fmt.Errorf("crossed book: best bid %.6f >= best ask %.6f", bid, ask)
	}
	return nil
}

// --- helpers ---

func upsertLevel(levels []Level, price float64, qty int) []Level {
	for i := range levels {
		if levels[i].Price == price {
			levels[i].TotalQty += qty
			return levels
		}
	}
	return append(levels, Level{Price: price, TotalQty: qty})
}

func removeQty(levels []Level, price float64, qty int) []Level {
	for i := range levels {
		if levels[i].Price == price {
			levels[i].TotalQty -= qty
			if levels[i].TotalQty <= 0 {
				return append(levels[:i], levels[i+1:]...)
			}
			return levels
		}
	}
	return levels
}
