package eval

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// --- ScrubQuery ---

func TestScrubQuery_RedactsKnownPII(t *testing.T) {
	cases := []struct {
		in    string
		want  string
	}{
		{"contact john@example.com please", "contact [email] please"},
		{"call 555-123-4567 now", "call [phone] now"},
		{"ssn 123-45-6789 here", "ssn [ssn] here"},
		{"jwt eyJabc.eyJdef.ghi here", "jwt [jwt] here"},
		{"ip 192.168.1.1 banned", "ip [ip] banned"},
		{"api_key=verylongsecretvalue9999", "[apikey]"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			out := ScrubQuery(tc.in)
			if !strings.Contains(out, tc.want) {
				t.Errorf("expected %q in scrubbed output; got %q", tc.want, out)
			}
		})
	}
}

func TestScrubQuery_BenignStaysIntact(t *testing.T) {
	in := "what was the database decision last sprint"
	if out := ScrubQuery(in); out != in {
		t.Errorf("benign query mutated: in=%q out=%q", in, out)
	}
}

// --- Store ---

func TestStore_AddScrubsAndStamps(t *testing.T) {
	s := NewStore(0)
	s.Add(Candidate{Query: "email me at j@x.com", K: 5, TopIDs: []string{"a"}})
	got := s.Snapshot()
	if len(got) != 1 {
		t.Fatalf("expected 1; got %d", len(got))
	}
	if !strings.Contains(got[0].Query, "[email]") {
		t.Errorf("expected query scrubbed; got %q", got[0].Query)
	}
	if got[0].SchemaVersion != SchemaVersion {
		t.Errorf("schema_version not stamped; got %d", got[0].SchemaVersion)
	}
	if got[0].CapturedAt.IsZero() {
		t.Errorf("captured_at not stamped")
	}
}

func TestStore_RingBufferDropsOldest(t *testing.T) {
	s := NewStore(2)
	s.Add(Candidate{Query: "q1", K: 1, TopIDs: []string{"a"}})
	s.Add(Candidate{Query: "q2", K: 1, TopIDs: []string{"b"}})
	s.Add(Candidate{Query: "q3", K: 1, TopIDs: []string{"c"}})
	got := s.Snapshot()
	if len(got) != 2 {
		t.Fatalf("expected 2 after ring drop; got %d", len(got))
	}
	if got[0].Query != "q2" || got[1].Query != "q3" {
		t.Errorf("expected oldest dropped; got %v", []string{got[0].Query, got[1].Query})
	}
}

func TestStore_ResetEmpties(t *testing.T) {
	s := NewStore(0)
	s.Add(Candidate{Query: "q", K: 1, TopIDs: []string{"a"}})
	s.Reset()
	if s.Len() != 0 {
		t.Errorf("expected empty after reset; got %d", s.Len())
	}
}

// --- Export / LoadCandidates round-trip ---

func TestExport_Roundtrip(t *testing.T) {
	s := NewStore(0)
	s.Add(Candidate{Query: "q1", K: 5, TopIDs: []string{"a", "b"}, LatencyMs: 10})
	s.Add(Candidate{Query: "q2", K: 3, TopIDs: []string{"c"}, LatencyMs: 20, Strategy: "vector"})

	var buf bytes.Buffer
	n, err := s.Export(&buf)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 written; got %d", n)
	}

	loaded, err := LoadCandidates(&buf)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded; got %d", len(loaded))
	}
	if loaded[0].Query != "q1" || loaded[1].Strategy != "vector" {
		t.Errorf("round-trip lost data: %+v", loaded)
	}
}

func TestLoadCandidates_RejectsUnknownSchemaVersion(t *testing.T) {
	bad := []byte(`{"schema_version":99,"query":"q","k":1,"top_ids":["x"]}`)
	_, err := LoadCandidates(bytes.NewReader(bad))
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Errorf("expected schema_version error; got %v", err)
	}
}

func TestLoadCandidates_RejectsMissingFields(t *testing.T) {
	cases := [][]byte{
		[]byte(`{"schema_version":1,"k":1,"top_ids":["a"]}`),    // no query
		[]byte(`{"schema_version":1,"query":"q","top_ids":[]}`), // k=0
	}
	for _, b := range cases {
		_, err := LoadCandidates(bytes.NewReader(b))
		if err == nil {
			t.Errorf("expected validation error for %s", b)
		}
	}
}

// --- CaptureEnabled env flag ---

func TestCaptureEnabled_OffByDefault(t *testing.T) {
	t.Setenv(EnvFlag, "")
	if CaptureEnabled() {
		t.Errorf("expected disabled by default")
	}
}

func TestCaptureEnabled_AcceptsTruthyValues(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "YES"} {
		t.Setenv(EnvFlag, v)
		if !CaptureEnabled() {
			t.Errorf("expected enabled for %q", v)
		}
	}
}

func TestCaptureEnabled_RejectsRandomNoise(t *testing.T) {
	for _, v := range []string{"0", "false", "no", "abc", " "} {
		t.Setenv(EnvFlag, v)
		if CaptureEnabled() {
			t.Errorf("expected disabled for %q", v)
		}
	}
}

// --- Jaccard ---

func TestJaccard_IdenticalIs1(t *testing.T) {
	if got := Jaccard([]string{"a", "b"}, []string{"a", "b"}); got != 1 {
		t.Errorf("expected 1; got %v", got)
	}
}

func TestJaccard_DisjointIs0(t *testing.T) {
	if got := Jaccard([]string{"a"}, []string{"b"}); got != 0 {
		t.Errorf("expected 0; got %v", got)
	}
}

func TestJaccard_PartialOverlap(t *testing.T) {
	// {a,b} ∩ {b,c} = {b}; ∪ = {a,b,c}; jaccard = 1/3
	got := Jaccard([]string{"a", "b"}, []string{"b", "c"})
	if got < 0.33 || got > 0.34 {
		t.Errorf("expected ~0.333; got %v", got)
	}
}

func TestJaccard_BothEmptyIs1(t *testing.T) {
	if got := Jaccard(nil, nil); got != 1 {
		t.Errorf("expected 1 for both-empty; got %v", got)
	}
}

func TestJaccard_OneEmptyIs0(t *testing.T) {
	if got := Jaccard(nil, []string{"a"}); got != 0 {
		t.Errorf("expected 0; got %v", got)
	}
}

// --- Replay Run ---

func TestRun_PerfectMatchIs1(t *testing.T) {
	baseline := []Candidate{
		{SchemaVersion: 1, Query: "q1", K: 2, TopIDs: []string{"a", "b"}, LatencyMs: 10},
		{SchemaVersion: 1, Query: "q2", K: 2, TopIDs: []string{"c", "d"}, LatencyMs: 20},
	}
	fn := func(_ context.Context, q string, _ int) ([]string, time.Duration, error) {
		switch q {
		case "q1":
			return []string{"a", "b"}, 11 * time.Millisecond, nil
		case "q2":
			return []string{"c", "d"}, 19 * time.Millisecond, nil
		}
		return nil, 0, nil
	}
	sum := Run(context.Background(), baseline, fn)
	if sum.Failed != 0 || sum.MeanJaccard < 0.999 {
		t.Errorf("expected ok run; got %+v", sum)
	}
	if !sum.Pass(0.85) {
		t.Errorf("expected pass at threshold 0.85")
	}
}

func TestRun_RegressionFlagged(t *testing.T) {
	baseline := []Candidate{
		{SchemaVersion: 1, Query: "q", K: 3, TopIDs: []string{"a", "b", "c"}, LatencyMs: 10},
	}
	fn := func(_ context.Context, _ string, _ int) ([]string, time.Duration, error) {
		return []string{"x", "y", "z"}, 12 * time.Millisecond, nil
	}
	sum := Run(context.Background(), baseline, fn)
	if sum.MeanJaccard != 0 {
		t.Errorf("expected jaccard=0; got %v", sum.MeanJaccard)
	}
	if sum.Pass(0.85) {
		t.Errorf("did not expect pass at 0.85 when jaccard=0")
	}
}

func TestRun_QueryErrorsCounted(t *testing.T) {
	baseline := []Candidate{
		{SchemaVersion: 1, Query: "q", K: 2, TopIDs: []string{"a"}},
	}
	fn := func(_ context.Context, _ string, _ int) ([]string, time.Duration, error) {
		return nil, 0, errors.New("backend down")
	}
	sum := Run(context.Background(), baseline, fn)
	if sum.Failed != 1 {
		t.Errorf("expected failed=1; got %d", sum.Failed)
	}
	if sum.Pass(0.0) {
		t.Errorf("any failure must fail Pass()")
	}
}

func TestRun_LatencyDeltaMedian(t *testing.T) {
	baseline := []Candidate{
		{SchemaVersion: 1, Query: "a", K: 1, TopIDs: []string{"x"}, LatencyMs: 10},
		{SchemaVersion: 1, Query: "b", K: 1, TopIDs: []string{"x"}, LatencyMs: 20},
		{SchemaVersion: 1, Query: "c", K: 1, TopIDs: []string{"x"}, LatencyMs: 30},
	}
	fn := func(_ context.Context, q string, _ int) ([]string, time.Duration, error) {
		// every replay is 100ms slower -> deltas 90,80,70 -> median 80
		var ms time.Duration
		switch q {
		case "a":
			ms = 100 * time.Millisecond
		case "b":
			ms = 100 * time.Millisecond
		case "c":
			ms = 100 * time.Millisecond
		}
		return []string{"x"}, ms, nil
	}
	sum := Run(context.Background(), baseline, fn)
	if sum.MedianLatencyDelta != 80 {
		t.Errorf("expected median delta 80ms; got %d", sum.MedianLatencyDelta)
	}
}

func TestSummary_FormatText(t *testing.T) {
	sum := ReplaySummary{Total: 5, Failed: 1, MeanJaccard: 0.72, Top1Stability: 0.6, MedianLatencyDelta: -5, Duration: 200 * time.Millisecond}
	out := sum.FormatText()
	for _, want := range []string{"FAIL", "0.720", "60.0%", "-5", "200ms"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in formatted text; got %q", want, out)
		}
	}
}
