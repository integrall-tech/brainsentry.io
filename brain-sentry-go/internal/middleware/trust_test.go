package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/integraltech/brainsentry/pkg/trust"
)

func TestTrustRemote_TagsRequestContext(t *testing.T) {
	var observed trust.Level
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed = trust.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	wrapped := TrustRemote(final)

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if observed != trust.Remote {
		t.Errorf("expected Remote tagged in handler ctx; got %s", observed)
	}
}

func TestTrustRemote_RouteCanElevateAfter(t *testing.T) {
	// Simulate a route that elevates to Local (e.g. a localhost-only admin
	// endpoint) after the middleware tagged Remote. The elevation must win.
	final := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := trust.WithLocal(r.Context())
		if trust.FromContext(ctx) != trust.Local {
			t.Errorf("elevation should win over earlier Remote tag")
		}
	})
	TrustRemote(final).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}

func TestRequireLocalTrust_RefusesRemote(t *testing.T) {
	called := false
	wrapped := RequireLocalTrust(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/danger", nil)
	rr := httptest.NewRecorder()
	TrustRemote(wrapped).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403; got %d", rr.Code)
	}
	if called {
		t.Errorf("inner handler must not run when trust is below Local")
	}
	body := rr.Body.String()
	if !contains(body, "trust_required_local") {
		t.Errorf("expected error_code in body; got %s", body)
	}
}

func TestRequireLocalTrust_AllowsLocal(t *testing.T) {
	called := false
	wrapped := RequireLocalTrust(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	// Skip TrustRemote middleware; tag the request as Local directly to
	// simulate a route that has already elevated trust.
	req := httptest.NewRequest(http.MethodPost, "/admin/danger", nil).WithContext(trust.WithLocal(httptest.NewRequest(http.MethodPost, "/admin/danger", nil).Context()))
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if !called {
		t.Errorf("inner handler must run with Local trust; status was %d", rr.Code)
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
