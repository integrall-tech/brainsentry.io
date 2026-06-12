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
func TenantExtractor(defaultTenantID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requested := r.Header.Get("X-Tenant-ID")
			if requested == "" {
				requested = r.URL.Query().Get("tenant")
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
