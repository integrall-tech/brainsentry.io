package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/integraltech/brainsentry/pkg/tenant"
)

// tenantCapturingHandler records the tenant ID resolved into the context.
func tenantCapturingHandler(captured *string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*captured = tenant.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

func TestTenantExtractor_ClaimIsSourceOfTruth(t *testing.T) {
	var got string
	handler := TenantExtractor("default-tenant")(tenantCapturingHandler(&got))

	claims := &AuthClaims{UserID: "u1", TenantID: "tenant-a", Roles: []string{RoleUser}}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/v1/memories", nil), claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got != "tenant-a" {
		t.Errorf("expected tenant from JWT claim (tenant-a), got %q", got)
	}
}

func TestTenantExtractor_HeaderSpoofingDeniedForNonAdmin(t *testing.T) {
	var got string
	handler := TenantExtractor("default-tenant")(tenantCapturingHandler(&got))

	claims := &AuthClaims{UserID: "u1", TenantID: "tenant-a", Roles: []string{RoleUser}}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/v1/memories", nil), claims)
	req.Header.Set("X-Tenant-ID", "tenant-b")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-tenant header from non-admin, got %d", rec.Code)
	}
	if got != "" {
		t.Errorf("handler should not run on denied request, but resolved tenant %q", got)
	}
}

func TestTenantExtractor_QuerySpoofingDeniedForNonAdmin(t *testing.T) {
	var got string
	handler := TenantExtractor("default-tenant")(tenantCapturingHandler(&got))

	claims := &AuthClaims{UserID: "u1", TenantID: "tenant-a", Roles: []string{RoleReadonly}}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/v1/memories?tenant=tenant-b", nil), claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-tenant query param from non-admin, got %d", rec.Code)
	}
}

func TestTenantExtractor_HeaderMatchingClaimAllowed(t *testing.T) {
	var got string
	handler := TenantExtractor("default-tenant")(tenantCapturingHandler(&got))

	claims := &AuthClaims{UserID: "u1", TenantID: "tenant-a", Roles: []string{RoleUser}}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/v1/memories", nil), claims)
	req.Header.Set("X-Tenant-ID", "tenant-a")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when header matches claim, got %d", rec.Code)
	}
	if got != "tenant-a" {
		t.Errorf("expected tenant-a, got %q", got)
	}
}

func TestTenantExtractor_AdminMayActOnOtherTenant(t *testing.T) {
	var got string
	handler := TenantExtractor("default-tenant")(tenantCapturingHandler(&got))

	claims := &AuthClaims{UserID: "admin-1", TenantID: "tenant-a", Roles: []string{RoleAdmin}}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/v1/memories", nil), claims)
	req.Header.Set("X-Tenant-ID", "tenant-b")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for admin cross-tenant override, got %d", rec.Code)
	}
	if got != "tenant-b" {
		t.Errorf("expected admin override to tenant-b, got %q", got)
	}
}

func TestTenantExtractor_UnauthenticatedUsesHeaderThenDefault(t *testing.T) {
	var got string
	handler := TenantExtractor("default-tenant")(tenantCapturingHandler(&got))

	// Public path (no claims): header is honored.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Tenant-ID", "tenant-pub")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || got != "tenant-pub" {
		t.Errorf("expected header tenant on public path, got code=%d tenant=%q", rec.Code, got)
	}

	// No claims, no header: default.
	got = ""
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || got != "default-tenant" {
		t.Errorf("expected default tenant, got code=%d tenant=%q", rec.Code, got)
	}
}

func TestTenantExtractor_InvalidTenantIDRejected(t *testing.T) {
	var got string
	handler := TenantExtractor("default-tenant")(tenantCapturingHandler(&got))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Tenant-ID", "bad tenant id!!")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid tenant ID format, got %d", rec.Code)
	}
}
