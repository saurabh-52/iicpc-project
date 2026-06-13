package validator

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ParsedBBO holds the best-bid/best-ask extracted from an engine response.
type ParsedBBO struct {
	BestBid float64
	BestAsk float64
	HasBid  bool
	HasAsk  bool
	Parsed  bool // true if at least one of bid/ask was found
}

// BBOTolerance is the maximum allowed difference (in price units) between the
// user-reported BBO and the reference BBO.  Set to 0.01 (1 cent) to account
// for floating-point rounding differences across languages.
const BBOTolerance = 0.01

// ParseEngineOutput attempts to extract best_bid and best_ask from the raw
// engine response string.  It tries JSON first, then falls back to a simple
// key=value pipe-delimited format (FIX-style).
//
// Supported formats:
//
//	JSON:  {"status":"accepted","best_bid":99.95,"best_ask":100.05}
//	KV:    accepted|best_bid=99.95|best_ask=100.05
func ParseEngineOutput(raw string) ParsedBBO {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedBBO{}
	}

	// --- JSON attempt ---
	if p := parseJSON(raw); p.Parsed {
		return p
	}

	// --- Pipe-delimited key=value fallback ---
	if p := parseKV(raw); p.Parsed {
		return p
	}

	return ParsedBBO{}
}

// BBOMatches returns true if the user-reported BBO is within tolerance of the
// reference BBO.  Only the fields present in the parsed result are compared.
func BBOMatches(parsed ParsedBBO, refBid float64, hasBid bool, refAsk float64, hasAsk bool) bool {
	if parsed.HasBid && hasBid {
		if math.Abs(parsed.BestBid-refBid) > BBOTolerance {
			return false
		}
	}
	if parsed.HasAsk && hasAsk {
		if math.Abs(parsed.BestAsk-refAsk) > BBOTolerance {
			return false
		}
	}
	return true
}

// IsCrossed returns true when the parsed BBO indicates a crossed book
// (best bid >= best ask).
func IsCrossed(p ParsedBBO) bool {
	if !p.HasBid || !p.HasAsk {
		return false // can't determine cross without both sides
	}
	if p.BestBid <= 0 || p.BestAsk <= 0 {
		return false // one or both sides are empty/invalid, cannot be crossed
	}
	return p.BestBid >= p.BestAsk
}

// ---- internal parsers ----

func parseJSON(raw string) ParsedBBO {
	// Quick check — must look like JSON.
	if len(raw) == 0 || raw[0] != '{' {
		return ParsedBBO{}
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ParsedBBO{}
	}

	p := ParsedBBO{}
	if v, ok := m["best_bid"]; ok {
		if f, err := strconv.ParseFloat(strings.Trim(string(v), `"`), 64); err == nil {
			p.BestBid = f
			p.HasBid = true
			p.Parsed = true
		}
	}
	if v, ok := m["best_ask"]; ok {
		if f, err := strconv.ParseFloat(strings.Trim(string(v), `"`), 64); err == nil {
			p.BestAsk = f
			p.HasAsk = true
			p.Parsed = true
		}
	}
	return p
}

var kvPattern = regexp.MustCompile(`(?i)(best_bid|best_ask)\s*=\s*([0-9]+\.?[0-9]*)`)

func parseKV(raw string) ParsedBBO {
	matches := kvPattern.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return ParsedBBO{}
	}

	p := ParsedBBO{}
	for _, m := range matches {
		key := strings.ToLower(m[1])
		val, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		switch key {
		case "best_bid":
			p.BestBid = val
			p.HasBid = true
			p.Parsed = true
		case "best_ask":
			p.BestAsk = val
			p.HasAsk = true
			p.Parsed = true
		}
	}
	return p
}
