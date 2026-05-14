package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/integraltech/brainsentry/internal/models"
)

type stubProber struct {
	respond func(model string) error
}

func (s *stubProber) Probe(_ context.Context, model string) error {
	if s.respond == nil {
		return nil
	}
	return s.respond(model)
}

func TestModelsHandler_ListReturnsSnapshot(t *testing.T) {
	cfg := models.Config{Default: "default-m", Tier: map[models.Tier]string{
		models.TierUtility: "u",
	}}
	h := NewModelsHandler(cfg, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", rr.Code)
	}
	var resp SnapshotResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Snapshot) != len(models.AllTiers()) {
		t.Errorf("expected %d entries; got %d", len(models.AllTiers()), len(resp.Snapshot))
	}
	var sawUtility bool
	for _, r := range resp.Snapshot {
		if r.Tier == models.TierUtility && r.Model == "u" && r.Source == "config-tier" {
			sawUtility = true
		}
	}
	if !sawUtility {
		t.Errorf("expected utility tier to be picked up from config")
	}
}

func TestModelsHandler_DoctorAllPass(t *testing.T) {
	h := NewModelsHandler(models.Config{Default: "x"}, &stubProber{})
	req := httptest.NewRequest(http.MethodGet, "/v1/models/doctor", nil)
	rr := httptest.NewRecorder()
	h.Doctor(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200; got %d", rr.Code)
	}
	var rep models.DoctorReport
	if err := json.NewDecoder(rr.Body).Decode(&rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !rep.OK {
		t.Errorf("expected ok aggregate; got %+v", rep)
	}
}

func TestModelsHandler_DoctorReportsFailure(t *testing.T) {
	h := NewModelsHandler(models.Config{Default: "phantom"}, &stubProber{
		respond: func(_ string) error { return errors.New("HTTP 404 not found") },
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/models/doctor", nil)
	rr := httptest.NewRecorder()
	h.Doctor(rr, req)
	var rep models.DoctorReport
	if err := json.NewDecoder(rr.Body).Decode(&rep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rep.OK {
		t.Errorf("expected ok=false")
	}
	for _, r := range rep.Results {
		if r.Failure != models.FailureModelNotFound {
			t.Errorf("expected model_not_found per tier; got %s", r.Failure)
		}
	}
}
