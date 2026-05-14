package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/eval"
)

func TestBuildReplayQueryFn_HitsSearchEndpointAndReturnsIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/memories/search" || r.Method != http.MethodPost {
			t.Errorf("wrong endpoint %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": "m1"}, {"id": "m2"}, {"id": "m3"},
			},
		})
	}))
	defer srv.Close()

	fn := buildReplayQueryFn(srv.URL, srv.Client())
	ids, dur, err := fn(context.Background(), "what about q?", 3)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(ids) != 3 || ids[0] != "m1" {
		t.Errorf("expected 3 ids starting m1; got %v", ids)
	}
	if dur <= 0 {
		t.Errorf("expected non-zero latency; got %v", dur)
	}
}

func TestBuildReplayQueryFn_PropagatesNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	fn := buildReplayQueryFn(srv.URL, srv.Client())
	_, _, err := fn(context.Background(), "q", 5)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 error; got %v", err)
	}
}

func TestBuildReplayQueryFn_RoundTripsThroughEvalRun(t *testing.T) {
	// End-to-end: a fake server returns ids; we feed the QueryFn into
	// eval.Run and assert the summary lines up with what gbrain's loop
	// expects.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"id": "a"}, {"id": "b"}},
		})
	}))
	defer srv.Close()

	baseline := []eval.Candidate{
		{SchemaVersion: 1, Query: "q1", K: 2, TopIDs: []string{"a", "b"}, LatencyMs: 100},
	}
	fn := buildReplayQueryFn(srv.URL, srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sum := eval.Run(ctx, baseline, fn)
	if !sum.Pass(0.85) {
		t.Errorf("expected pass; got summary %+v", sum)
	}
}

func TestJsonStringLit_EscapesQuotesAndNewline(t *testing.T) {
	cases := map[string]string{
		`hello`:        `"hello"`,
		`with "quote"`: `"with \"quote\""`,
		"with\\back":   `"with\\back"`,
		"line1\nline2": `"line1\nline2"`,
	}
	for in, want := range cases {
		if got := jsonStringLit(in); got != want {
			t.Errorf("jsonStringLit(%q) = %q; want %q", in, got, want)
		}
	}
}
