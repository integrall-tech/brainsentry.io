package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/integraltech/brainsentry/internal/middleware"
)

// TestAdminTrust_WiringMatchesProductionStack mirrors the exact middleware
// order used in cmd/server/main.go (TrustRemote first, then the route is
// wrapped in RequireLocalTrust). Catches a regression where someone
// accidentally drops TrustRemote or forgets to wrap the route guard.
func TestAdminTrust_WiringMatchesProductionStack(t *testing.T) {
	h := NewAdminTrustHandler(nil)

	r := chi.NewRouter()
	r.Use(middleware.TrustRemote)
	r.Post("/v1/admin/wipe-embedding-cache", middleware.RequireLocalTrust(h.WipeEmbeddingCache))

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/v1/admin/wipe-embedding-cache", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 from network with no Local elevation; got %d", resp.StatusCode)
	}
}
