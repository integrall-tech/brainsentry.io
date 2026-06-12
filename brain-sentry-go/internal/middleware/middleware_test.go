package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/integraltech/brainsentry/internal/service"
	"github.com/integraltech/brainsentry/pkg/tenant"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// okHandler is a simple handler that always returns 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})

// newLogger returns a discard logger suitable for tests.
func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newJWTService returns a JWTService with a fixed test secret.
func newJWTService() *service.JWTService {
	return service.NewJWTService("test-secret-key-middleware", 1*time.Hour)
}

// validToken generates a valid JWT for the given tenant and roles.
func validToken(t *testing.T, jwtSvc *service.JWTService, tenantID string, roles []string) string {
	t.Helper()
	tok, err := jwtSvc.GenerateToken("user-1", "user@example.com", tenantID, roles)
	if err != nil {
		t.Fatalf("failed to generate test token: %v", err)
	}
	return tok
}

// claimsInContext returns a new request whose context contains the given claims.
func claimsInContext(r *http.Request, claims *AuthClaims) *http.Request {
	ctx := context.WithValue(r.Context(), authContextKey{}, claims)
	return r.WithContext(ctx)
}

// -----------------------------------------------------------------------
// CORS middleware
// -----------------------------------------------------------------------

func TestCORS_AllowedOriginSetsHeader(t *testing.T) {
	handler := CORS(
		[]string{"https://example.com", "https://app.example.com"},
		[]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin 'https://example.com', got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCORS_DisallowedOriginDoesNotSetHeader(t *testing.T) {
	handler := CORS(
		[]string{"https://example.com"},
		[]string{"GET", "POST"},
	)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header for unknown origin, got %q", got)
	}
}

func TestCORS_MethodsHeaderAlwaysSet(t *testing.T) {
	methods := []string{"GET", "POST", "OPTIONS"}
	handler := CORS([]string{"https://example.com"}, methods)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("Access-Control-Allow-Methods")
	expected := strings.Join(methods, ", ")
	if got != expected {
		t.Errorf("expected Access-Control-Allow-Methods %q, got %q", expected, got)
	}
}

func TestCORS_AllowHeadersAlwaysSet(t *testing.T) {
	handler := CORS([]string{"https://example.com"}, []string{"GET"})(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("Access-Control-Allow-Headers")
	if got == "" {
		t.Error("expected Access-Control-Allow-Headers to be set")
	}
	if !strings.Contains(got, "Authorization") {
		t.Errorf("expected Authorization in Access-Control-Allow-Headers, got %q", got)
	}
	if !strings.Contains(got, "X-Tenant-ID") {
		t.Errorf("expected X-Tenant-ID in Access-Control-Allow-Headers, got %q", got)
	}
}

func TestCORS_CredentialsHeaderSet(t *testing.T) {
	handler := CORS([]string{"https://example.com"}, []string{"GET"})(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials 'true', got %q", got)
	}
}

func TestCORS_MaxAgeHeaderSet(t *testing.T) {
	handler := CORS([]string{"https://example.com"}, []string{"GET"})(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Max-Age"); got != "3600" {
		t.Errorf("expected Access-Control-Max-Age '3600', got %q", got)
	}
}

func TestCORS_PreflightReturns204(t *testing.T) {
	handler := CORS(
		[]string{"https://example.com"},
		[]string{"GET", "POST", "OPTIONS"},
	)(okHandler)

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS preflight, got %d", rec.Code)
	}
}

func TestCORS_PreflightDoesNotCallNextHandler(t *testing.T) {
	called := false
	sentinel := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := CORS([]string{"https://example.com"}, []string{"GET", "OPTIONS"})(sentinel)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Error("next handler should not be called for OPTIONS preflight")
	}
}

func TestCORS_NoOriginHeaderPassesThrough(t *testing.T) {
	handler := CORS([]string{"https://example.com"}, []string{"GET"})(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Deliberately no Origin header
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// Rate limiting middleware
// -----------------------------------------------------------------------

func TestRateLimit_RequestsUnderLimitPassThrough(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{RequestsPerMinute: 10, BurstSize: 5})
	handler := RateLimit(rl)(okHandler)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Tenant-ID", "tenant-a")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimit_RequestsOverLimitGet429(t *testing.T) {
	// BurstSize of 2 means only 2 requests pass before limiting kicks in.
	rl := NewRateLimiter(RateLimiterConfig{RequestsPerMinute: 1, BurstSize: 2})
	handler := RateLimit(rl)(okHandler)

	const tenantID = "tenant-overflow"

	// First two requests should succeed.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Tenant-ID", tenantID)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// Third request should be rate-limited.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after burst exhausted, got %d", rec.Code)
	}
}

func TestRateLimit_429ResponseHasRetryAfterHeader(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{RequestsPerMinute: 1, BurstSize: 1})
	handler := RateLimit(rl)(okHandler)

	tenantID := "tenant-retry"
	// Exhaust the single token.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", tenantID)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	// Next request should be limited.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-Tenant-ID", tenantID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req2)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Error("expected Retry-After header on 429 response")
	}
}

func TestRateLimit_DifferentTenantsHaveIndependentBuckets(t *testing.T) {
	// BurstSize of 1 per tenant.
	rl := NewRateLimiter(RateLimiterConfig{RequestsPerMinute: 1, BurstSize: 1})
	handler := RateLimit(rl)(okHandler)

	// Exhaust tenant-alpha's bucket.
	reqA := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA.Header.Set("X-Tenant-ID", "tenant-alpha")
	handler.ServeHTTP(httptest.NewRecorder(), reqA)

	// tenant-beta should still have its own full bucket.
	reqB := httptest.NewRequest(http.MethodGet, "/", nil)
	reqB.Header.Set("X-Tenant-ID", "tenant-beta")
	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)

	if recB.Code != http.StatusOK {
		t.Errorf("tenant-beta should not be affected by tenant-alpha's rate limit, got %d", recB.Code)
	}

	// Verify tenant-alpha is now rate-limited.
	reqA2 := httptest.NewRequest(http.MethodGet, "/", nil)
	reqA2.Header.Set("X-Tenant-ID", "tenant-alpha")
	recA2 := httptest.NewRecorder()
	handler.ServeHTTP(recA2, reqA2)

	if recA2.Code != http.StatusTooManyRequests {
		t.Errorf("tenant-alpha should be rate-limited, got %d", recA2.Code)
	}
}

func TestRateLimit_TenantFromQueryParam(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{RequestsPerMinute: 60, BurstSize: 3})
	handler := RateLimit(rl)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/?tenantId=query-tenant", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when tenant from query param, got %d", rec.Code)
	}
}

func TestRateLimit_DefaultTenantWhenNoHeader(t *testing.T) {
	rl := NewRateLimiter(RateLimiterConfig{RequestsPerMinute: 60, BurstSize: 5})
	handler := RateLimit(rl)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No X-Tenant-ID header, no query param → falls back to "default"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for default tenant, got %d", rec.Code)
	}
}

func TestRateLimit_HeaderTakesPrecedenceOverQueryParam(t *testing.T) {
	// Use a very tight limiter; each bucket only holds 1 token.
	// Exhaust only the header tenant, then verify query tenant is unaffected.
	rl := NewRateLimiter(RateLimiterConfig{RequestsPerMinute: 1, BurstSize: 1})
	handler := RateLimit(rl)(okHandler)

	// Exhaust "header-tenant" via the header.
	reqH := httptest.NewRequest(http.MethodGet, "/?tenantId=query-tenant", nil)
	reqH.Header.Set("X-Tenant-ID", "header-tenant")
	handler.ServeHTTP(httptest.NewRecorder(), reqH)

	// "query-tenant" bucket should still be full because the header took precedence.
	reqQ := httptest.NewRequest(http.MethodGet, "/?tenantId=query-tenant", nil)
	recQ := httptest.NewRecorder()
	handler.ServeHTTP(recQ, reqQ)

	if recQ.Code != http.StatusOK {
		t.Errorf("query-tenant should be unaffected when header sets a different tenant, got %d", recQ.Code)
	}
}

// -----------------------------------------------------------------------
// Token bucket internals
// -----------------------------------------------------------------------

func TestTokenBucket_Allow(t *testing.T) {
	b := newTokenBucket(3, 1) // 3 tokens, refill at 1/sec

	// First three should succeed.
	for i := 0; i < 3; i++ {
		if !b.allow() {
			t.Errorf("token %d: expected allow=true", i+1)
		}
	}
	// Fourth should fail.
	if b.allow() {
		t.Error("expected allow=false when bucket empty")
	}
}

func TestTokenBucket_RefillOverTime(t *testing.T) {
	b := newTokenBucket(1, 100) // 100 tokens/sec refill
	b.allow()                   // drain the single token

	// Move lastRefill back to simulate elapsed time (add enough for ~2 tokens)
	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-50 * time.Millisecond) // 0.05s * 100/s = 5 tokens
	b.mu.Unlock()

	if !b.allow() {
		t.Error("expected token to be available after simulated refill time")
	}
}

func TestTokenBucket_DoesNotExceedMax(t *testing.T) {
	b := newTokenBucket(5, 1000) // refill very fast

	// Simulate a long time elapsed.
	b.mu.Lock()
	b.lastRefill = b.lastRefill.Add(-10 * time.Second)
	b.mu.Unlock()

	// Should not exceed maxTokens=5.
	count := 0
	for b.allow() {
		count++
		if count > 10 {
			t.Fatal("bucket did not cap at maxTokens")
		}
	}
	if count != 5 {
		t.Errorf("expected exactly 5 tokens, got %d", count)
	}
}

// -----------------------------------------------------------------------
// RBAC middleware
// -----------------------------------------------------------------------

func TestRequireRole_AdminAllowedForAdminRole(t *testing.T) {
	handler := RequireRole(RoleAdmin)(okHandler)

	claims := &AuthClaims{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Roles:    []string{RoleAdmin},
	}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/admin", nil), claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for ADMIN role, got %d", rec.Code)
	}
}

func TestRequireRole_UserDeniedForAdminEndpoint(t *testing.T) {
	handler := RequireRole(RoleAdmin)(okHandler)

	claims := &AuthClaims{
		UserID:   "user-2",
		TenantID: "tenant-1",
		Roles:    []string{RoleUser},
	}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/admin", nil), claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for USER accessing admin endpoint, got %d", rec.Code)
	}
}

func TestRequireRole_ReadonlyDeniedForAdminEndpoint(t *testing.T) {
	handler := RequireRole(RoleAdmin)(okHandler)

	claims := &AuthClaims{
		UserID:   "user-3",
		TenantID: "tenant-1",
		Roles:    []string{RoleReadonly},
	}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/admin", nil), claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for READONLY accessing admin endpoint, got %d", rec.Code)
	}
}

func TestRequireRole_MissingClaimsDenied(t *testing.T) {
	handler := RequireRole(RoleAdmin)(okHandler)

	// No claims in context
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when no claims present, got %d", rec.Code)
	}
}

func TestRequireRole_MultipleAllowedRoles(t *testing.T) {
	handler := RequireRole(RoleAdmin, RoleUser)(okHandler)

	for _, role := range []string{RoleAdmin, RoleUser} {
		claims := &AuthClaims{
			UserID:   "user-x",
			TenantID: "tenant-1",
			Roles:    []string{role},
		}
		req := claimsInContext(httptest.NewRequest(http.MethodGet, "/resource", nil), claims)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for role %q, got %d", role, rec.Code)
		}
	}
}

func TestRequireRole_UserWithMultipleRolesAllowed(t *testing.T) {
	handler := RequireRole(RoleAdmin)(okHandler)

	// User has both USER and ADMIN roles
	claims := &AuthClaims{
		UserID:   "user-multi",
		TenantID: "tenant-1",
		Roles:    []string{RoleUser, RoleAdmin},
	}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/admin", nil), claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when user has ADMIN among multiple roles, got %d", rec.Code)
	}
}

func TestRequireRole_EmptyRolesInClaims(t *testing.T) {
	handler := RequireRole(RoleAdmin)(okHandler)

	claims := &AuthClaims{
		UserID:   "user-empty",
		TenantID: "tenant-1",
		Roles:    []string{},
	}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/admin", nil), claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for empty roles, got %d", rec.Code)
	}
}

func TestRequireRole_ForbiddenBodyContainsMessage(t *testing.T) {
	handler := RequireRole(RoleAdmin)(okHandler)

	claims := &AuthClaims{Roles: []string{RoleUser}}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/admin", nil), claims)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "forbidden") && !strings.Contains(body, "403") {
		t.Errorf("expected forbidden message in body, got %q", body)
	}
}

// -----------------------------------------------------------------------
// JWT auth middleware
// -----------------------------------------------------------------------

func TestJWTAuth_ValidTokenSetsClaimsInContext(t *testing.T) {
	jwtSvc := newJWTService()
	token := validToken(t, jwtSvc, "tenant-jwt", []string{RoleUser})

	var capturedClaims *AuthClaims
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClaims = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := JWTAuth(jwtSvc, nil)(captureHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid token, got %d", rec.Code)
	}
	if capturedClaims == nil {
		t.Fatal("expected claims to be set in context")
	}
	if capturedClaims.TenantID != "tenant-jwt" {
		t.Errorf("expected TenantID 'tenant-jwt', got %q", capturedClaims.TenantID)
	}
}

func TestJWTAuth_MissingTokenReturns401(t *testing.T) {
	jwtSvc := newJWTService()
	handler := JWTAuth(jwtSvc, nil)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
	// No Authorization header
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when no Authorization header, got %d", rec.Code)
	}
}

func TestJWTAuth_MalformedBearerReturns401(t *testing.T) {
	jwtSvc := newJWTService()
	handler := JWTAuth(jwtSvc, nil)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // Basic, not Bearer
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for non-Bearer auth scheme, got %d", rec.Code)
	}
}

func TestJWTAuth_InvalidTokenReturns401(t *testing.T) {
	jwtSvc := newJWTService()
	handler := JWTAuth(jwtSvc, nil)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
	req.Header.Set("Authorization", "Bearer this.is.not.a.valid.jwt")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", rec.Code)
	}
}

func TestJWTAuth_ExpiredTokenReturns401(t *testing.T) {
	expiredSvc := service.NewJWTService("test-secret-key-middleware", -1*time.Hour)
	validatingSvc := newJWTService() // same secret, same key but validates expiry

	token, err := expiredSvc.GenerateToken("u1", "e@e.com", "t1", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := JWTAuth(validatingSvc, nil)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", rec.Code)
	}
}

func TestJWTAuth_PublicPathBypassesAuth(t *testing.T) {
	jwtSvc := newJWTService()
	publicPaths := []string{"/health", "/public"}
	handler := JWTAuth(jwtSvc, publicPaths)(okHandler)

	for _, path := range []string{"/health", "/public/page", "/public"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		// No Authorization header
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("path %q: expected 200 for public path without token, got %d", path, rec.Code)
		}
	}
}

func TestJWTAuth_NonPublicPathRequiresToken(t *testing.T) {
	jwtSvc := newJWTService()
	handler := JWTAuth(jwtSvc, []string{"/public"})(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/private", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for non-public path without token, got %d", rec.Code)
	}
}

func TestJWTAuth_WrongSecretReturns401(t *testing.T) {
	signingService := service.NewJWTService("signing-secret", 1*time.Hour)
	validatingService := service.NewJWTService("different-secret", 1*time.Hour)

	token := validToken(t, signingService, "tenant-1", []string{RoleUser})

	handler := JWTAuth(validatingService, nil)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when token signed with different secret, got %d", rec.Code)
	}
}

func TestJWTAuth_NilPublicPathsStillWorks(t *testing.T) {
	jwtSvc := newJWTService()
	handler := JWTAuth(jwtSvc, nil)(okHandler)

	token := validToken(t, jwtSvc, "t1", []string{RoleUser})
	req := httptest.NewRequest(http.MethodGet, "/api/resource", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestClaimsFromContext_NilWhenNotSet(t *testing.T) {
	ctx := context.Background()
	if claims := ClaimsFromContext(ctx); claims != nil {
		t.Errorf("expected nil claims from empty context, got %+v", claims)
	}
}

func TestClaimsFromContext_ReturnsSetClaims(t *testing.T) {
	original := &AuthClaims{UserID: "u1", TenantID: "t1"}
	ctx := context.WithValue(context.Background(), authContextKey{}, original)
	got := ClaimsFromContext(ctx)
	if got == nil {
		t.Fatal("expected claims, got nil")
	}
	if got.UserID != "u1" {
		t.Errorf("expected UserID 'u1', got %q", got.UserID)
	}
}

// -----------------------------------------------------------------------
// Recovery (panic) middleware
// -----------------------------------------------------------------------

func TestRecovery_NoPanicPassesThrough(t *testing.T) {
	logger := newLogger()
	handler := Recovery(logger)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for normal request, got %d", rec.Code)
	}
}

func TestRecovery_PanicReturns500(t *testing.T) {
	logger := newLogger()
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went terribly wrong")
	})

	handler := Recovery(logger)(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic, got %d", rec.Code)
	}
}

func TestRecovery_PanicBodyContainsErrorMessage(t *testing.T) {
	logger := newLogger()
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("deliberate panic")
	})

	handler := Recovery(logger)(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "internal server error") && !strings.Contains(body, "500") {
		t.Errorf("expected internal server error message in body, got %q", body)
	}
}

func TestRecovery_PanicWithError(t *testing.T) {
	logger := newLogger()
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler) // panic with an error value
	})

	handler := Recovery(logger)(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Should not propagate the panic.
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after error panic, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// Tenant extractor middleware
// -----------------------------------------------------------------------

func TestTenantExtractor_FromXTenantIDHeader(t *testing.T) {
	const defaultTenant = "default-tenant"
	const headerTenant = "header-tenant-id"

	var capturedTenantID string
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID = tenant.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := TenantExtractor(defaultTenant)(captureHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", headerTenant)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if capturedTenantID != headerTenant {
		t.Errorf("expected tenant %q from header, got %q", headerTenant, capturedTenantID)
	}
}

func TestTenantExtractor_FromQueryParam(t *testing.T) {
	const defaultTenant = "default-tenant"
	const queryTenant = "query-tenant-id"

	var capturedTenantID string
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID = tenant.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := TenantExtractor(defaultTenant)(captureHandler)

	req := httptest.NewRequest(http.MethodGet, "/?tenant="+queryTenant, nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if capturedTenantID != queryTenant {
		t.Errorf("expected tenant %q from query param, got %q", queryTenant, capturedTenantID)
	}
}

func TestTenantExtractor_FromJWTClaim(t *testing.T) {
	const defaultTenant = "default-tenant"
	const claimTenant = "jwt-claim-tenant"

	var capturedTenantID string
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID = tenant.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := TenantExtractor(defaultTenant)(captureHandler)

	claims := &AuthClaims{UserID: "u1", TenantID: claimTenant}
	req := claimsInContext(httptest.NewRequest(http.MethodGet, "/", nil), claims)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if capturedTenantID != claimTenant {
		t.Errorf("expected tenant %q from JWT claim, got %q", claimTenant, capturedTenantID)
	}
}

func TestTenantExtractor_DefaultWhenNothingProvided(t *testing.T) {
	const defaultTenant = "fallback-default"

	var capturedTenantID string
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID = tenant.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := TenantExtractor(defaultTenant)(captureHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No header, no query, no claims
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if capturedTenantID != defaultTenant {
		t.Errorf("expected default tenant %q, got %q", defaultTenant, capturedTenantID)
	}
}

func TestTenantExtractor_HeaderTakesPrecedenceOverQuery(t *testing.T) {
	var capturedTenantID string
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTenantID = tenant.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := TenantExtractor("default")(captureHandler)

	req := httptest.NewRequest(http.MethodGet, "/?tenant=query-tenant", nil)
	req.Header.Set("X-Tenant-ID", "header-tenant")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if capturedTenantID != "header-tenant" {
		t.Errorf("expected header tenant to take precedence, got %q", capturedTenantID)
	}
}

// NOTE: the old "header/query takes precedence over JWT claim" tests were
// removed on purpose: that behavior was a cross-tenant access vulnerability.
// The claim-is-source-of-truth semantics are covered in tenant_test.go.

func TestTenantExtractor_TenantInContextAfterExtraction(t *testing.T) {
	var tenantFound bool
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantFound = tenant.HasTenant(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := TenantExtractor("some-default")(captureHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "test-tenant")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if !tenantFound {
		t.Error("expected tenant to be present in context after extraction")
	}
}

// -----------------------------------------------------------------------
// RequestLogger middleware (basic smoke test)
// -----------------------------------------------------------------------

func TestRequestLogger_PassesThroughResponse(t *testing.T) {
	logger := newLogger()
	handler := RequestLogger(logger)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequestLogger_LogsNon200StatusCodes(t *testing.T) {
	logger := newLogger()
	notFoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	handler := RequestLogger(logger)(notFoundHandler)

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 to pass through, got %d", rec.Code)
	}
}
