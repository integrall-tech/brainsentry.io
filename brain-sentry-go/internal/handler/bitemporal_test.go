package handler

import (
	"net/url"
	"testing"
	"time"
)

func mustValues(t *testing.T, raw string) url.Values {
	t.Helper()
	v, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("bad query %q: %v", raw, err)
	}
	return v
}

func TestParseTemporalQuery_Valid(t *testing.T) {
	q := mustValues(t, "since=2026-06-01T00:00:00Z&limit=25")
	ts, limit, err := parseTemporalQuery(q, "since")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ts.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("timestamp parsed wrong: %v", ts)
	}
	if limit != 25 {
		t.Errorf("limit: got %d want 25", limit)
	}
}

func TestParseTemporalQuery_MissingParam(t *testing.T) {
	_, _, err := parseTemporalQuery(mustValues(t, "limit=10"), "since")
	if err == nil {
		t.Fatal("expected error for missing timestamp param")
	}
	// Error must name the specific param so the 400 message is useful.
	if got := err.Error(); got == "" || !contains(got, "since") {
		t.Errorf("error should mention the param name 'since'; got %q", got)
	}
}

func TestParseTemporalQuery_InvalidTimestamp(t *testing.T) {
	_, _, err := parseTemporalQuery(mustValues(t, "at=not-a-date"), "at")
	if err == nil {
		t.Fatal("expected error for non-RFC3339 timestamp")
	}
}

func TestParseTemporalQuery_LimitDefaultsTo100(t *testing.T) {
	_, limit, err := parseTemporalQuery(mustValues(t, "since=2026-06-01T00:00:00Z"), "since")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 100 {
		t.Errorf("default limit: got %d want 100", limit)
	}
}

func TestParseTemporalQuery_NonPositiveLimitIgnored(t *testing.T) {
	for _, raw := range []string{
		"since=2026-06-01T00:00:00Z&limit=0",
		"since=2026-06-01T00:00:00Z&limit=-5",
		"since=2026-06-01T00:00:00Z&limit=abc",
	} {
		_, limit, err := parseTemporalQuery(mustValues(t, raw), "since")
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", raw, err)
		}
		if limit != 100 {
			t.Errorf("%q: bad/non-positive limit should fall back to 100; got %d", raw, limit)
		}
	}
}

func TestParseTemporalQuery_ParamNameIsHonored(t *testing.T) {
	// "since" present but we ask for "at" — should be treated as missing.
	if _, _, err := parseTemporalQuery(mustValues(t, "since=2026-06-01T00:00:00Z"), "at"); err == nil {
		t.Error("asking for 'at' when only 'since' is present should error")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
