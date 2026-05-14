package middleware

import (
	"net/http"

	"github.com/integraltech/brainsentry/pkg/trust"
)

// TrustRemote wraps every HTTP handler to tag the request context with
// trust.Remote. This is intentional belt-and-braces: pkg/trust already
// returns Remote when nothing is set, but tagging explicitly here means
// every audit log line carries the trust level even if a caller forgets to
// look it up.
//
// Why a single Remote tag everywhere instead of per-route trust?
// HTTP requests cross the network. Even an authenticated admin's request
// can have been replayed or man-in-the-middle'd in theory; the operator
// CLI is the only path we treat as fully trusted. If a future flow needs
// elevated trust (e.g. a localhost-only admin endpoint), it should mark
// itself with trust.WithLocal *after* this middleware in the chain so the
// elevation is auditable in source.
func TrustRemote(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := trust.WithRemote(r.Context())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireLocalTrust is a route-level guard that refuses any request whose
// context is below trust.Local. Use sparingly — only on operations whose
// blast radius is so wide that even an authenticated admin should not
// trigger them over the network (mass deletion, full cache wipe, schema
// rebuild). The CLI is expected to elevate with trust.WithLocal before
// dispatching to in-process equivalents.
//
// Returns HTTP 403 with a structured error body so the admin UI can
// surface a clear "this action is CLI-only" message.
func RequireLocalTrust(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := trust.Require(r.Context(), trust.Local); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"Forbidden","error_code":"trust_required_local","message":"this operation is restricted to the operator CLI (use ` + "`brainsentry`" + ` instead of the network)"}`))
			return
		}
		next(w, r)
	}
}
