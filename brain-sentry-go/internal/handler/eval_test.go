package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/integraltech/brainsentry/internal/eval"
)

func TestEvalHandler_StatsEmpty(t *testing.T) {
	store := eval.NewStore(0)
	h := NewEvalHandler(store)
	rr := httptest.NewRecorder()
	h.Stats(rr, httptest.NewRequest(http.MethodGet, "/v1/eval/candidates/stats", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// JSON numbers come out as float64
	if got["count"].(float64) != 0 {
		t.Errorf("expected count=0; got %v", got["count"])
	}
}

func TestEvalHandler_ExportRoundTrip(t *testing.T) {
	store := eval.NewStore(0)
	store.Add(eval.Candidate{Query: "q1", K: 2, TopIDs: []string{"a", "b"}, LatencyMs: 10})
	store.Add(eval.Candidate{Query: "q2", K: 1, TopIDs: []string{"c"}, LatencyMs: 20})
	h := NewEvalHandler(store)

	rr := httptest.NewRecorder()
	h.Export(rr, httptest.NewRequest(http.MethodGet, "/v1/eval/candidates.ndjson", nil))

	if ct := rr.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("wrong content-type %q", ct)
	}
	loaded, err := eval.LoadCandidates(bytes.NewReader(rr.Body.Bytes()))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 candidates loaded; got %d", len(loaded))
	}
	if !strings.Contains(rr.Body.String(), "\"top_ids\":[\"a\",\"b\"]") {
		t.Errorf("expected ndjson body to carry top_ids; got %s", rr.Body.String())
	}
}

func TestEvalHandler_Reset(t *testing.T) {
	store := eval.NewStore(0)
	store.Add(eval.Candidate{Query: "q", K: 1, TopIDs: []string{"a"}})
	h := NewEvalHandler(store)
	rr := httptest.NewRecorder()
	h.Reset(rr, httptest.NewRequest(http.MethodPost, "/v1/eval/candidates/reset", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", rr.Code)
	}
	if store.Len() != 0 {
		t.Errorf("expected store empty after reset; got %d", store.Len())
	}
}
