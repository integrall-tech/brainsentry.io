package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/integraltech/brainsentry/internal/models"
)

func TestFetchSnapshot_DecodesPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"snapshot": []models.ResolveResult{
				{Tier: models.TierUtility, Model: "u-model", Source: "config-tier"},
			},
		})
	}))
	defer srv.Close()
	got, err := fetchSnapshot(context.Background(), srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(got) != 1 || got[0].Model != "u-model" {
		t.Errorf("snapshot mis-decoded: %+v", got)
	}
}

func TestFetchModelsDoctor_DecodesPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(models.DoctorReport{
			OK: false,
			Results: []models.ProbeResult{{
				Tier: models.TierUtility, Model: "phantom",
				Failure: models.FailureModelNotFound, Detail: "HTTP 404",
			}},
		})
	}))
	defer srv.Close()
	rep, err := fetchModelsDoctor(context.Background(), srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if rep.OK {
		t.Errorf("expected ok=false")
	}
	if rep.Results[0].Failure != models.FailureModelNotFound {
		t.Errorf("expected model_not_found; got %s", rep.Results[0].Failure)
	}
}

func TestFetchSnapshot_NonOKErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := fetchSnapshot(context.Background(), srv.URL, srv.Client())
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 error; got %v", err)
	}
}

func TestRenderSnapshot_TextHasHeaders(t *testing.T) {
	var buf bytes.Buffer
	renderSnapshot(&buf, []models.ResolveResult{
		{Tier: models.TierUtility, Model: "u", Source: "config-tier"},
		{Tier: models.TierDeep, Model: "", Source: "(unresolved)"},
	}, false)
	out := buf.String()
	for _, want := range []string{"TIER", "SOURCE", "utility", "(unresolved)"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output; got %q", want, out)
		}
	}
}

func TestRenderModelsDoctor_TextSurfacesFailureFields(t *testing.T) {
	var buf bytes.Buffer
	renderModelsDoctor(&buf, models.DoctorReport{
		OK: false,
		Results: []models.ProbeResult{{
			Tier: models.TierUtility, Model: "u", Failure: models.FailureAuth,
			Detail: "401 unauthorized", Hint: "rotate the key",
		}},
	}, false)
	out := buf.String()
	for _, want := range []string{"FAIL", "auth", "rotate the key", "401 unauthorized"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output; got %q", want, out)
		}
	}
}

func TestRenderSnapshot_JSONIsStable(t *testing.T) {
	var buf bytes.Buffer
	renderSnapshot(&buf, []models.ResolveResult{
		{Tier: models.TierUtility, Model: "u", Source: "config-tier"},
	}, true)
	var got struct {
		Snapshot []models.ResolveResult `json:"snapshot"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Snapshot) != 1 {
		t.Errorf("expected 1 entry in snapshot; got %d", len(got.Snapshot))
	}
}
