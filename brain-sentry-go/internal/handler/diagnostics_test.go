package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/diagnostics"
)

type stubChecker struct {
	name string
	res  diagnostics.CheckResult
}

func (s *stubChecker) Name() string { return s.name }
func (s *stubChecker) Check(_ context.Context) diagnostics.CheckResult {
	r := s.res
	r.Name = s.name
	return r
}

func TestDiagnosticsHandler_OKReport(t *testing.T) {
	doc := diagnostics.New([]diagnostics.Checker{
		&stubChecker{name: "redis", res: diagnostics.CheckResult{Status: diagnostics.StatusOK, Message: "alive"}},
		&stubChecker{name: "pg", res: diagnostics.CheckResult{Status: diagnostics.StatusOK, Message: "alive"}},
	}, time.Second)
	h := NewDiagnosticsHandler(doc)

	req := httptest.NewRequest(http.MethodGet, "/v1/diagnostics", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", rr.Code)
	}
	var rep diagnostics.Report
	if err := json.NewDecoder(rr.Body).Decode(&rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rep.Status != diagnostics.StatusOK {
		t.Errorf("expected aggregate ok; got %s", rep.Status)
	}
	if len(rep.Checks) != 2 {
		t.Errorf("expected 2 checks; got %d", len(rep.Checks))
	}
}

func TestDiagnosticsHandler_FailReportStill200(t *testing.T) {
	doc := diagnostics.New([]diagnostics.Checker{
		&stubChecker{name: "openrouter", res: diagnostics.CheckResult{Status: diagnostics.StatusFail, Message: "down"}},
	}, time.Second)
	h := NewDiagnosticsHandler(doc)

	req := httptest.NewRequest(http.MethodGet, "/v1/diagnostics", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("HTTP code stays 200 even on aggregate fail; got %d", rr.Code)
	}
	var rep diagnostics.Report
	if err := json.NewDecoder(rr.Body).Decode(&rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rep.Status != diagnostics.StatusFail {
		t.Errorf("expected aggregate fail in body; got %s", rep.Status)
	}
}

func TestDiagnosticsHandler_NilDoctorReturns503(t *testing.T) {
	h := NewDiagnosticsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/diagnostics", nil)
	rr := httptest.NewRecorder()
	h.Get(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when doctor missing; got %d", rr.Code)
	}
}
