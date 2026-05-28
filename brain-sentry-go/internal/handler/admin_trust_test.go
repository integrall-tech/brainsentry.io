package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/integraltech/brainsentry/internal/middleware"
	"github.com/integraltech/brainsentry/pkg/trust"
)

// TestWipeEmbeddingCache_NoRedisReturns503 ensures the operator gets a
// clear "not configured" response when Redis is missing — same shape used
// across the codebase.
func TestWipeEmbeddingCache_NoRedisReturns503(t *testing.T) {
	h := NewAdminTrustHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/wipe-embedding-cache", nil).
		WithContext(trust.WithLocal(context.Background()))
	rr := httptest.NewRecorder()
	h.WipeEmbeddingCache(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 with nil cache; got %d", rr.Code)
	}
}

// TestWipeEmbeddingCache_RefusedFromRemote ensures the trust gate runs when
// the route is wrapped in RequireLocalTrust + TrustRemote middleware.
func TestWipeEmbeddingCache_RefusedFromRemote(t *testing.T) {
	h := NewAdminTrustHandler(nil)
	wrapped := middleware.TrustRemote(middleware.RequireLocalTrust(h.WipeEmbeddingCache))
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/wipe-embedding-cache", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 from remote; got %d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error_code"] != "trust_required_local" {
		t.Errorf("expected error_code trust_required_local; got %+v", body)
	}
}
