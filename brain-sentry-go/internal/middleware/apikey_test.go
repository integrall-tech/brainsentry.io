package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// fakeAuthenticator resolves secrets to keys from a map, and records whether
// it was consulted at all — some tests care that a credential never even
// reached the key lookup.
type fakeAuthenticator struct {
	keys   map[string]*domain.APIKey
	called int
}

func (f *fakeAuthenticator) Authenticate(_ context.Context, secret string) (*domain.APIKey, error) {
	f.called++
	if k, ok := f.keys[secret]; ok {
		return k, nil
	}
	return nil, errors.New("invalid api key")
}

// chain wires the middlewares in the same order as the server, so these tests
// exercise the real interaction rather than each piece in isolation.
func chain(auth APIKeyAuthenticator, defaultTenant string, h http.Handler) http.Handler {
	return APIKeyAuth(auth, nil)(TenantExtractor(defaultTenant)(h))
}

// tenantEcho reports the tenant the request finally resolved to.
func tenantEcho() (http.Handler, *string) {
	seen := new(string)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = tenant.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}), seen
}

const (
	secretA = "bs_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	secretB = "bs_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func authWithTenantA() *fakeAuthenticator {
	return &fakeAuthenticator{keys: map[string]*domain.APIKey{
		secretA: {ID: "key-a", TenantID: "tenant-a", Name: "core-a"},
		secretB: {ID: "key-b", TenantID: "tenant-b", Name: "core-b"},
	}}
}

// ---------------------------------------------------------------------------
// The guard the RFC calls the most important test of the whole design:
// a key of tenant A must not reach tenant B, by any route.
// ---------------------------------------------------------------------------

func TestAPIKey_CannotReachAnotherTenantViaHeader(t *testing.T) {
	h, seen := tenantEcho()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories", nil)
	req.Header.Set("Authorization", "Bearer "+secretA)
	req.Header.Set("X-Tenant-ID", "tenant-b")

	chain(authWithTenantA(), "tenant-default", h).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-tenant header, got %d", rr.Code)
	}
	if *seen != "" {
		t.Errorf("handler must not run at all; it saw tenant %q", *seen)
	}
}

func TestAPIKey_CannotReachAnotherTenantViaQueryParam(t *testing.T) {
	h, seen := tenantEcho()
	rr := httptest.NewRecorder()
	// The query param is the other door into TenantExtractor; closing only
	// the header would leave this one open.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories?tenant=tenant-b", nil)
	req.Header.Set("X-API-Key", secretA)

	chain(authWithTenantA(), "tenant-default", h).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-tenant query param, got %d", rr.Code)
	}
	if *seen != "" {
		t.Errorf("handler must not run; it saw tenant %q", *seen)
	}
}

// An ADMIN user may act on another tenant; a service key may not — this is
// the exact inheritance the RFC says must be broken.
//
// Deliberately exercises TenantExtractor DIRECTLY instead of going through
// APIKeyAuth. Routed through the full chain, APIKeyAuth overwrites the claims
// with RoleUser, so the request would be refused by the pre-existing admin
// check and the test would pass without the service-principal guard existing
// at all. A mutation run proved that: with the guard disabled, the
// full-chain version stayed green.
//
// What this pins down is the property that must survive a future change —
// "even WITH admin claims, a service principal cannot cross tenants" — rather
// than today's implementation detail of which role the key is given.
func TestTenantExtractor_ServicePrincipalIgnoresAdminRole(t *testing.T) {
	for _, tc := range []struct{ name, target, header string }{
		{"header", "/api/v1/memories", "tenant-b"},
		{"query param", "/api/v1/memories?tenant=tenant-b", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, seen := tenantEcho()
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			if tc.header != "" {
				req.Header.Set("X-Tenant-ID", tc.header)
			}

			ctx := context.WithValue(req.Context(), serviceKeyContextKey{},
				&ServicePrincipal{KeyID: "key-a", TenantID: "tenant-a"})
			ctx = context.WithValue(ctx, authContextKey{}, &AuthClaims{
				TenantID: "tenant-a",
				Roles:    []string{RoleAdmin}, // the escalation being denied
			})

			TenantExtractor("tenant-default")(h).ServeHTTP(rr, req.WithContext(ctx))

			if rr.Code != http.StatusForbidden {
				t.Fatalf("admin claims must not let a service principal cross tenants; got %d", rr.Code)
			}
			if *seen != "" {
				t.Errorf("handler must not run; it saw tenant %q", *seen)
			}
		})
	}
}

// The mirror of the above: an ADMIN *user* (no service principal) keeps the
// cross-tenant ability it has today. The guard must be narrow.
func TestTenantExtractor_AdminUserStillCrossesTenants(t *testing.T) {
	h, seen := tenantEcho()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories", nil)
	req.Header.Set("X-Tenant-ID", "tenant-b")

	ctx := context.WithValue(req.Context(), authContextKey{}, &AuthClaims{
		TenantID: "tenant-a",
		Roles:    []string{RoleAdmin},
	})
	TenantExtractor("tenant-default")(h).ServeHTTP(rr, req.WithContext(ctx))

	if rr.Code != http.StatusOK || *seen != "tenant-b" {
		t.Errorf("admin user must keep acting on another tenant (code=%d, tenant=%q)", rr.Code, *seen)
	}
}

func TestAPIKey_ResolvesItsOwnTenant(t *testing.T) {
	h, seen := tenantEcho()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories", nil)
	req.Header.Set("Authorization", "Bearer "+secretB)

	chain(authWithTenantA(), "tenant-default", h).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if *seen != "tenant-b" {
		t.Errorf("expected tenant-b from the key, got %q", *seen)
	}
}

// Matching header is allowed — it is redundant, not hostile.
func TestAPIKey_MatchingHeaderIsAccepted(t *testing.T) {
	h, seen := tenantEcho()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories", nil)
	req.Header.Set("X-API-Key", secretA)
	req.Header.Set("X-Tenant-ID", "tenant-a")

	chain(authWithTenantA(), "tenant-default", h).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK || *seen != "tenant-a" {
		t.Errorf("expected 200 on tenant-a, got %d / %q", rr.Code, *seen)
	}
}

// A service key must never land on the default tenant: that fallback is the
// fail-open pkg/tenant.FromContext still has, and the credential exists
// precisely so the tenant is never guessed.
func TestAPIKey_EmptyTenantFailsClosedInsteadOfDefaulting(t *testing.T) {
	auth := &fakeAuthenticator{keys: map[string]*domain.APIKey{
		secretA: {ID: "broken", TenantID: ""},
	}}

	h, seen := tenantEcho()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories", nil)
	req.Header.Set("X-API-Key", secretA)

	chain(auth, "tenant-default", h).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if *seen == "tenant-default" {
		t.Error("a key with no tenant must not fall back to the default tenant")
	}
}

// ---------------------------------------------------------------------------
// Credential handling
// ---------------------------------------------------------------------------

func TestAPIKey_InvalidSecretIs401(t *testing.T) {
	h, _ := tenantEcho()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories", nil)
	req.Header.Set("X-API-Key", "bs_this-key-does-not-exist")

	chain(authWithTenantA(), "tenant-default", h).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unknown key, got %d", rr.Code)
	}
}

// A JWT must pass through untouched — adding key auth cannot change how an
// existing credential is interpreted.
func TestAPIKey_JWTBearerIsNotClaimedAsAKey(t *testing.T) {
	auth := authWithTenantA()

	nextCalled := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if ServicePrincipalFromContext(r.Context()) != nil {
			t.Error("a JWT must not produce a service principal")
		}
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.fake.jwt")

	APIKeyAuth(auth, nil)(h).ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("JWT request must fall through to the next middleware")
	}
	if auth.called != 0 {
		t.Errorf("a JWT must not reach the key lookup; it was called %d times", auth.called)
	}
}

func TestAPIKey_NoCredentialFallsThrough(t *testing.T) {
	auth := authWithTenantA()
	nextCalled := false
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories", nil)
	APIKeyAuth(auth, nil)(h).ServeHTTP(rr, req)

	if !nextCalled || auth.called != 0 {
		t.Errorf("no credential must fall through untouched (next=%v, lookups=%d)", nextCalled, auth.called)
	}
}

// ---------------------------------------------------------------------------
// Privilege escalation
// ---------------------------------------------------------------------------

func TestRejectServicePrincipal_BlocksKeyFromMintingKeys(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not be reached by a service key")
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/tenant-b/api-keys", nil)
	req.Header.Set("X-API-Key", secretA)

	APIKeyAuth(authWithTenantA(), nil)(RejectServicePrincipal(h)).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestRejectServicePrincipal_AllowsUserJWT(t *testing.T) {
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants/t/api-keys", nil)
	RejectServicePrincipal(h).ServeHTTP(rr, req)

	if !called || rr.Code != http.StatusOK {
		t.Errorf("a non-service request must pass (called=%v, code=%d)", called, rr.Code)
	}
}

// A service key carries USER, never ADMIN: admin is the role that unlocks
// cross-tenant action on the user path.
func TestAPIKey_NeverCarriesAdminRole(t *testing.T) {
	var claims *AuthClaims
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories", nil)
	req.Header.Set("X-API-Key", secretA)
	APIKeyAuth(authWithTenantA(), nil)(h).ServeHTTP(rr, req)

	if claims == nil {
		t.Fatal("expected claims to be populated for a service key")
	}
	for _, role := range claims.Roles {
		if role == RoleAdmin {
			t.Fatal("a service key must never carry the ADMIN role")
		}
	}
}
