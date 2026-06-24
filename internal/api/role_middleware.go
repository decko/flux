package api

import (
	"net/http"

	"github.com/decko/flux/pkg/authctx"
)

// RequireRole returns middleware that checks the user has one of the allowed
// roles. It reads the role from context (set by AuthMiddleware). If the role
// is missing or not in the allowed list, a 403 Forbidden JSON response is
// returned.
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := authctx.Role(r.Context())
			for _, a := range allowed {
				if role == a {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeJSONError(w, http.StatusForbidden, "Forbidden", "")
		})
	}
}
