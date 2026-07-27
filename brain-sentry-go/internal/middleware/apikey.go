package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/service"
)

type serviceKeyContextKey struct{}

// ServicePrincipal identifies a request authenticated by a service API key
// rather than by a user token.
//
// It exists as its own context value — instead of being folded into
// AuthClaims — because the difference matters at exactly one place
// (TenantExtractor) and folding it in would make "is this a service?" a
// question about the shape of the claims, which is easy to get wrong later.
type ServicePrincipal struct {
	KeyID    string
	TenantID string
	Name     string
}

// ServicePrincipalFromContext returns the service principal, or nil when the
// request was not authenticated by an API key.
func ServicePrincipalFromContext(ctx context.Context) *ServicePrincipal {
	p, _ := ctx.Value(serviceKeyContextKey{}).(*ServicePrincipal)
	return p
}

// APIKeyAuthenticator is the subset of APIKeyService this middleware needs.
type APIKeyAuthenticator interface {
	Authenticate(ctx context.Context, secret string) (*domain.APIKey, error)
}

// APIKeyAuth authenticates service keys, leaving everything else untouched.
//
// It runs BEFORE JWTAuth and is deliberately permissive about what it
// ignores: a request with no credential, or with a credential that is not
// shaped like a service key, passes straight through to JWT handling. Adding
// this middleware therefore cannot change how any existing request is
// interpreted — the two credentials are told apart by the "bs_" prefix, not
// by trying one and falling back on failure.
//
// On success it populates BOTH the service principal and AuthClaims. The
// claims exist so RequireRole and the handlers keep working unchanged; the
// principal is what TenantExtractor uses to refuse cross-tenant access.
func APIKeyAuth(keys APIKeyAuthenticator, publicPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, p := range publicPaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}

			secret := extractAPIKeySecret(r)
			if secret == "" {
				next.ServeHTTP(w, r) // not a key: JWTAuth decides
				return
			}

			key, err := keys.Authenticate(r.Context(), secret)
			if err != nil {
				// The credential claimed to be a service key and was not a
				// valid one. Falling through to JWT here would only produce a
				// confusing "invalid token" for what is really a bad key.
				writeTenantError(w, http.StatusUnauthorized, "invalid or revoked api key")
				return
			}

			ctx := context.WithValue(r.Context(), serviceKeyContextKey{}, &ServicePrincipal{
				KeyID:    key.ID,
				TenantID: key.TenantID,
				Name:     key.Name,
			})
			// Roles: a service key carries RoleUser and never RoleAdmin.
			// Admin is the role that can act on another tenant, and a
			// credential whose entire purpose is being confined to one tenant
			// must not be able to hold it.
			ctx = context.WithValue(ctx, authContextKey{}, &AuthClaims{
				UserID:   "service:" + key.ID,
				Email:    "",
				TenantID: key.TenantID,
				Roles:    []string{RoleUser},
			})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractAPIKeySecret pulls a service key from either accepted header.
// X-API-Key is checked first because a request carrying it clearly means it;
// Authorization: Bearer is shared with JWT and only counts when the value
// carries the service-key prefix.
func extractAPIKeySecret(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-API-Key")); v != "" {
		return v
	}
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if !domain.LooksLikeAPIKey(token) {
		return ""
	}
	return token
}

// RejectServicePrincipal blocks service keys from a route.
//
// Used on the API-key management surface: a key that can mint keys is a key
// that can mint a key for another tenant, which would hand back exactly the
// cross-tenant reach the credential design removes.
func RejectServicePrincipal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ServicePrincipalFromContext(r.Context()) != nil {
			writeTenantError(w, http.StatusForbidden,
				"forbidden: service keys cannot manage api keys")
			return
		}
		next.ServeHTTP(w, r)
	})
}

var _ APIKeyAuthenticator = (*service.APIKeyService)(nil)
