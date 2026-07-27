package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/integraltech/brainsentry/pkg/tenant"
)

// TenantExtractor returns a middleware that resolves the tenant ID for the
// request and injects it into the context.
//
// For authenticated requests the JWT claim is the source of truth. The
// X-Tenant-ID header (or `tenant` query param) may request a different
// tenant, but only users with the ADMIN role are allowed to act on a tenant
// other than their own — everyone else gets 403. Header/query must never
// outrank the claim: that would let any authenticated user read another
// tenant's data by spoofing a header.
//
// Unauthenticated requests only reach this middleware on public paths
// (JWTAuth runs first); for those, header > query > default applies.
//
// Service API keys are the exception to all of the above: they are pinned to
// the tenant stored with the key and NOTHING a caller sends can move them —
// not the header, not the query, not an admin role. See below.
func TenantExtractor(defaultTenantID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requested := r.Header.Get("X-Tenant-ID")
			if requested == "" {
				requested = r.URL.Query().Get("tenant")
			}

			// A service key is scoped to exactly one tenant, by construction
			// (RFC-014 §8.1). The user path lets an ADMIN act on another
			// tenant via the header; a service key must NOT inherit that,
			// because "a bug writes to the wrong customer" is the worst
			// defect this integration can have.
			//
			// A divergent header is refused rather than ignored: silently
			// serving tenant A to a caller who asked for B is how a wrong
			// assumption survives to production unnoticed.
			if principal := ServicePrincipalFromContext(r.Context()); principal != nil {
				if requested != "" && requested != principal.TenantID {
					writeTenantError(w, http.StatusForbidden,
						"forbidden: api key is scoped to a single tenant")
					return
				}
				// No default fallback here either: the key always carries its
				// tenant, so an empty one means something is broken and must
				// fail closed instead of landing on the default tenant
				// (pkg/tenant.FromContext returns it, which is fail-open).
				if principal.TenantID == "" {
					writeTenantError(w, http.StatusForbidden,
						"forbidden: api key has no tenant")
					return
				}
				if err := tenant.ValidateTenantID(principal.TenantID); err != nil {
					writeTenantError(w, http.StatusBadRequest, err.Error())
					return
				}
				ctx := tenant.WithTenant(r.Context(), principal.TenantID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			var tenantID string
			if claims := ClaimsFromContext(r.Context()); claims != nil && claims.TenantID != "" {
				tenantID = claims.TenantID
				if requested != "" && requested != claims.TenantID {
					if !claimsHaveRole(claims, RoleAdmin) {
						writeTenantError(w, http.StatusForbidden, "forbidden: cannot act on another tenant")
						return
					}
					tenantID = requested
				}
			} else {
				tenantID = requested
			}

			if tenantID == "" {
				tenantID = defaultTenantID
			}

			// Validate tenant ID format
			if err := tenant.ValidateTenantID(tenantID); err != nil {
				writeTenantError(w, http.StatusBadRequest, err.Error())
				return
			}

			ctx := tenant.WithTenant(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func claimsHaveRole(claims *AuthClaims, role string) bool {
	for _, r := range claims.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func writeTenantError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
