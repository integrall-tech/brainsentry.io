package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/integraltech/brainsentry/internal/service"
)

type authContextKey struct{}

// AuthClaims is stored in the request context after JWT validation.
type AuthClaims = service.JWTClaims

// ClaimsFromContext extracts JWT claims from the request context.
func ClaimsFromContext(ctx context.Context) *AuthClaims {
	claims, _ := ctx.Value(authContextKey{}).(*AuthClaims)
	return claims
}

// JWTAuth returns a middleware that validates JWT tokens.
// Requests to public paths bypass authentication.
func JWTAuth(jwtService *service.JWTService, publicPaths []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is public
			for _, p := range publicPaths {
				if strings.HasPrefix(r.URL.Path, p) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Already authenticated by a service API key (APIKeyAuth runs
			// first). Without this the request would be rejected here for
			// carrying no JWT — the key IS the credential.
			if ServicePrincipalFromContext(r.Context()) != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"missing or invalid authorization header","status":401}`, http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := jwtService.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token","status":401}`, http.StatusUnauthorized)
				return
			}

			// Store claims in context
			ctx := context.WithValue(r.Context(), authContextKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
