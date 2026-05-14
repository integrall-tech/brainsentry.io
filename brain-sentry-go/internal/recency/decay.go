// Package recency models the recency-decay axis of memory scoring as a
// **separate, orthogonal** dimension from salience (importance / mattering).
//
// Two ideas:
//
//  1. Per-prefix decay. A memory whose category prefix is `daily/` is
//     short-lived: weight should fall off in days. A memory under
//     `concepts/` (definitions, principles) is evergreen: it should not
//     decay at all. Operators tune the map per workspace.
//
//  2. Auto-detect from query. Phrases like "today", "this week", "now"
//     reveal the user actually wants fresh information. The detector
//     elevates the recency coefficient for the call without forcing the
//     caller to think about it.
//
// Composition with salience: the final score is
//
//	final = base_relevance * recency_factor * salience_factor
//
// recency_factor in [0, 1] (1 = fresh, 0 = fully decayed)
// salience_factor in [0, +inf)  (1 = neutral importance)
//
// Inspired by gbrain's src/core/search/recency-decay.ts +
// skills/conventions/salience-and-recency.md.
package recency

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// PrefixPolicy is the per-prefix decay rule.
type PrefixPolicy struct {
	// HalflifeDays — number of days at which the recency factor halves
	// (e.g. 14 means a 14-day-old memory is at 0.5). 0 disables decay
	// entirely (evergreen).
	HalflifeDays float64
	// Coefficient — multiplier on the decay exponent. Lets a prefix decay
	// faster (>1) or slower (<1) than its halflife alone would dictate.
	// 1.0 is the natural rate.
	Coefficient float64
}

// Config holds the operator-facing routing of prefix → policy.
type Config struct {
	Default  PrefixPolicy            // applied when no prefix matches
	Prefixes map[string]PrefixPolicy // keys are matched as longest-prefix-wins
}

// DefaultConfig is a sensible starting point. Operators are expected to
// override in config.yaml under `recency:`.
var DefaultConfig = Config{
	Default: PrefixPolicy{HalflifeDays: 90, Coefficient: 1.0},
	Prefixes: map[string]PrefixPolicy{
		"daily/":    {HalflifeDays: 14, Coefficient: 1.5},
		"weekly/":   {HalflifeDays: 30, Coefficient: 1.2},
		"concepts/": {HalflifeDays: 0, Coefficient: 0}, // evergreen
		"facts/":    {HalflifeDays: 0, Coefficient: 0}, // evergreen
		"events/":   {HalflifeDays: 7, Coefficient: 1.5},
	},
}

// PolicyForPath returns the longest-prefix-wins policy applicable to a
// memory path / category. Falls back to Default.
func (c Config) PolicyForPath(path string) PrefixPolicy {
	var best string
	for p := range c.Prefixes {
		if strings.HasPrefix(path, p) && len(p) > len(best) {
			best = p
		}
	}
	if best == "" {
		return c.Default
	}
	return c.Prefixes[best]
}

// Factor returns the recency factor in (0, 1] for a memory recorded at
// `recordedAt` given the policy and the current time `now`. A halflife of 0
// (or coefficient 0) yields 1 (no decay).
//
// Formula: factor = exp(-ln(2) * coefficient * age_days / halflife_days)
func Factor(p PrefixPolicy, recordedAt, now time.Time) float64 {
	if p.HalflifeDays <= 0 || p.Coefficient <= 0 {
		return 1
	}
	ageDays := now.Sub(recordedAt).Hours() / 24
	if ageDays <= 0 {
		return 1
	}
	exp := -math.Ln2 * p.Coefficient * ageDays / p.HalflifeDays
	return math.Exp(exp)
}

// --- Auto-detect: which queries want fresh results? ---

// freshnessHints — words that signal the user wants recent memories.
// Conservative on purpose (false-positives elevate recency on every query
// using the word "today" in some other sense, which is fine but worth
// keeping bounded).
var freshnessPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\btoday\b`),
	regexp.MustCompile(`(?i)\byesterday\b`),
	regexp.MustCompile(`(?i)\b(?:this|last)\s+(?:week|month|sprint)\b`),
	regexp.MustCompile(`(?i)\brecent(?:ly)?\b`),
	regexp.MustCompile(`(?i)\b(?:right\s+)?now\b`),
	regexp.MustCompile(`(?i)\blatest\b`),
	regexp.MustCompile(`(?i)\b(?:hoje|ontem|agora|recentes?|últim[oa]s?)\b`), // pt-BR
	regexp.MustCompile(`(?i)\besta\s+semana\b`),
}

// FreshnessHint reports whether the query carries a freshness signal.
// English + a small pt-BR vocabulary.
func FreshnessHint(query string) bool {
	for _, rx := range freshnessPatterns {
		if rx.MatchString(query) {
			return true
		}
	}
	return false
}

// ApplyAutoDetect amplifies a policy's coefficient when the query carries
// a freshness signal. Multiplier is 1.5 — modest enough not to drown out
// the operator-configured curve, big enough to actually change ranking on
// stale-vs-fresh memories.
func ApplyAutoDetect(query string, p PrefixPolicy) PrefixPolicy {
	if !FreshnessHint(query) {
		return p
	}
	out := p
	if out.HalflifeDays == 0 {
		// even evergreen prefixes get a soft recency tilt when the user
		// explicitly asks for "latest" / "today" — emergency override
		out.HalflifeDays = 14
	}
	out.Coefficient = math.Max(out.Coefficient, 1) * 1.5
	return out
}

// --- Composition with salience ---

// Compose multiplies a base relevance score by recency and salience
// factors. Pure helper — no side effects, easy to test.
func Compose(baseRelevance, recencyFactor, salienceFactor float64) float64 {
	return baseRelevance * recencyFactor * salienceFactor
}

// --- Snapshot for the admin UI ---

// PolicySnapshot returns the (sorted) per-prefix view used by the admin
// page to show the operator the current configuration.
func (c Config) PolicySnapshot() []PolicyRow {
	rows := make([]PolicyRow, 0, len(c.Prefixes)+1)
	rows = append(rows, PolicyRow{Prefix: "(default)", HalflifeDays: c.Default.HalflifeDays, Coefficient: c.Default.Coefficient})
	for p, pol := range c.Prefixes {
		rows = append(rows, PolicyRow{Prefix: p, HalflifeDays: pol.HalflifeDays, Coefficient: pol.Coefficient})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Prefix == "(default)" {
			return true
		}
		if rows[j].Prefix == "(default)" {
			return false
		}
		return rows[i].Prefix < rows[j].Prefix
	})
	return rows
}

// PolicyRow is the JSON-friendly shape for the snapshot.
type PolicyRow struct {
	Prefix       string  `json:"prefix"`
	HalflifeDays float64 `json:"halflife_days"`
	Coefficient  float64 `json:"coefficient"`
}
