package service

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TimeWindow is a half-bounded or fully-bounded interval extracted from a
// natural-language query, used to filter memory recall by recorded_at.
type TimeWindow struct {
	From    time.Time
	To      time.Time
	Matched string // the phrase that produced the window (for telemetry)
}

// IsZero reports whether the window is empty (no temporal intent found).
func (w TimeWindow) IsZero() bool {
	return w.From.IsZero() && w.To.IsZero()
}

// Deterministic temporal patterns. The router (query_router.go) already
// classifies a query as TEMPORAL via regex; this is the companion that turns
// the phrase into an actual [from, to] window — no LLM call, ~microseconds,
// matching the router's "regex over LLM" philosophy. Bilingual (pt-BR/en)
// because the product ships both locales.
var (
	reLastNDays  = regexp.MustCompile(`(?i)(?:últimos?|ultimos?|last|past)\s+(\d+)\s+(?:dias?|days?)`)
	reLastNHours = regexp.MustCompile(`(?i)(?:últimas?|ultimas?|last|past)\s+(\d+)\s+(?:horas?|hours?|hrs?)`)
	reLastNWeeks = regexp.MustCompile(`(?i)(?:últimas?|ultimas?|last|past)\s+(\d+)\s+(?:semanas?|weeks?)`)
)

// ExtractTimeWindow parses a relative/absolute time window from a query,
// resolved against `now`. Returns ok=false when no temporal intent is found,
// so callers leave non-temporal recall untouched. `now` is a parameter
// (not time.Now()) so the parser is deterministic under test.
func ExtractTimeWindow(query string, now time.Time) (TimeWindow, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return TimeWindow{}, false
	}

	// Numeric ranges first — most specific.
	if m := reLastNHours.FindStringSubmatch(q); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return TimeWindow{From: now.Add(-time.Duration(n) * time.Hour), To: now, Matched: m[0]}, true
		}
	}
	if m := reLastNDays.FindStringSubmatch(q); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return TimeWindow{From: now.AddDate(0, 0, -n), To: now, Matched: m[0]}, true
		}
	}
	if m := reLastNWeeks.FindStringSubmatch(q); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			return TimeWindow{From: now.AddDate(0, 0, -7*n), To: now, Matched: m[0]}, true
		}
	}

	// Named relative windows. Order matters: check multi-word phrases before
	// their single-word substrings ("last week" before "week"/"today").
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch {
	case containsAny(q, "ontem", "yesterday"):
		y := startOfDay.AddDate(0, 0, -1)
		return TimeWindow{From: y, To: startOfDay, Matched: "yesterday"}, true
	case containsAny(q, "hoje", "today"):
		return TimeWindow{From: startOfDay, To: now, Matched: "today"}, true
	case containsAny(q, "última semana", "ultima semana", "semana passada", "last week", "past week"):
		return TimeWindow{From: now.AddDate(0, 0, -7), To: now, Matched: "last week"}, true
	case containsAny(q, "esta semana", "essa semana", "this week"):
		// Week-to-date: back to most recent Monday 00:00.
		weekday := int(startOfDay.Weekday()) // Sunday=0
		daysSinceMonday := (weekday + 6) % 7
		monday := startOfDay.AddDate(0, 0, -daysSinceMonday)
		return TimeWindow{From: monday, To: now, Matched: "this week"}, true
	case containsAny(q, "último mês", "ultimo mes", "mês passado", "mes passado", "last month", "past month"):
		return TimeWindow{From: now.AddDate(0, -1, 0), To: now, Matched: "last month"}, true
	case containsAny(q, "este mês", "esse mes", "this month"):
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return TimeWindow{From: startOfMonth, To: now, Matched: "this month"}, true
	}

	return TimeWindow{}, false
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
