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
// appropriate side, simulating order matching if the spread is crossed.
// side must be "BUY" or "SELL".
func (ob *Orderbook) AddOrder(side string, price float64, qty int) {
	qtyRemaining := qty
	if side == "BUY" {
		// Match against asks: lowest ask price first (ob.Asks is sorted ascending)
		for len(ob.Asks) > 0 && qtyRemaining > 0 {
			bestAsk := &ob.Asks[0]
			if price < bestAsk.Price {
				break // Spread not crossed
			}
			matchQty := qtyRemaining
			if bestAsk.TotalQty < matchQty {
				matchQty = bestAsk.TotalQty
			}
			qtyRemaining -= matchQty
			bestAsk.TotalQty -= matchQty
			if bestAsk.TotalQty <= 0 {
				ob.Asks = ob.Asks[1:]
			}
		}
		// Insert remaining quantity if any
		if qtyRemaining > 0 {
			ob.Bids = upsertLevel(ob.Bids, price, qtyRemaining)
			sort.Slice(ob.Bids, func(i, j int) bool { return ob.Bids[i].Price > ob.Bids[j].Price })
		}
	} else {
		// Match against bids: highest bid price first (ob.Bids is sorted descending)
		for len(ob.Bids) > 0 && qtyRemaining > 0 {
			bestBid := &ob.Bids[0]
			if price > bestBid.Price {
				break // Spread not crossed
			}
			matchQty := qtyRemaining
			if bestBid.TotalQty < matchQty {
				matchQty = bestBid.TotalQty
			}
			qtyRemaining -= matchQty
			bestBid.TotalQty -= matchQty
			if bestBid.TotalQty <= 0 {
				ob.Bids = ob.Bids[1:]
			}
		}
		// Insert remaining quantity if any
		if qtyRemaining > 0 {
			ob.Asks = upsertLevel(ob.Asks, price, qtyRemaining)
			sort.Slice(ob.Asks, func(i, j int) bool { return ob.Asks[i].Price < ob.Asks[j].Price })
		}
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

// BBO returns the current best-bid and best-ask in one call.
func (ob *Orderbook) BBO() (bestBid float64, hasBid bool, bestAsk float64, hasAsk bool) {
	bestBid, hasBid = ob.BestBid()
	bestAsk, hasAsk = ob.BestAsk()
	return
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
