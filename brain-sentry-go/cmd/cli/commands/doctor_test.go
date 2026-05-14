package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/diagnostics"
)

func newDiagSrv(t *testing.T, body diagnostics.Report, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func TestFetchRemoteReport_DecodesOK(t *testing.T) {
	want := diagnostics.Report{
		Status: diagnostics.StatusOK,
		Checks: []diagnostics.CheckResult{
			{Name: "redis", Status: diagnostics.StatusOK, Message: "alive"},
		},
	}
	srv := newDiagSrv(t, want, http.StatusOK)
	defer srv.Close()

	got, err := fetchRemoteReport(context.Background(), srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got.Status != diagnostics.StatusOK {
		t.Errorf("expected ok; got %s", got.Status)
	}
	if len(got.Checks) != 1 || got.Checks[0].Name != "redis" {
		t.Errorf("expected redis check round-tripped; got %+v", got.Checks)
	}
}

func TestFetchRemoteReport_NonOKErrors(t *testing.T) {
	srv := newDiagSrv(t, diagnostics.Report{}, http.StatusInternalServerError)
	defer srv.Close()

	_, err := fetchRemoteReport(context.Background(), srv.URL, srv.Client())
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 error; got %v", err)
	}
}

func TestFetchRemoteReport_TimeoutHonored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := fetchRemoteReport(ctx, srv.URL, srv.Client())
	if err == nil {
		t.Errorf("expected timeout error")
	}
}

func TestRenderReport_TextHasMarkers(t *testing.T) {
	rep := diagnostics.Report{
		Status: diagnostics.StatusFail,
		Checks: []diagnostics.CheckResult{
			{Name: "openrouter", Status: diagnostics.StatusFail, Message: "401", Hint: "check key"},
		},
		Summary: diagnostics.Summary{Fail: 1},
	}
	var buf bytes.Buffer
	renderReport(&buf, rep, false)
	out := buf.String()
	for _, want := range []string{"[fail] openrouter", "401", "hint:"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in text output; got %q", want, out)
		}
	}
}

func TestRenderReport_JSONIsParseable(t *testing.T) {
	rep := diagnostics.Report{
		Status: diagnostics.StatusOK,
		Checks: []diagnostics.CheckResult{
			{Name: "redis", Status: diagnostics.StatusOK, Message: "alive"},
		},
	}
	var buf bytes.Buffer
	renderReport(&buf, rep, true)
	var got diagnostics.Report
	if err := json.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Status != diagnostics.StatusOK {
		t.Errorf("round-trip lost status; got %s", got.Status)
	}
}
