package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/decko/flux/pkg/authctx"
	"github.com/decko/flux/pkg/jwtutil"
)

// AuthMiddleware returns middleware that validates JWT Bearer tokens from
// the Authorization header. On success, the user ID and role are stored in
// the request context. On failure, a 401 Unauthorized JSON response is
// returned.
func AuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing authorization header", "")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeJSONError(w, http.StatusUnauthorized, "invalid authorization header format", "")
				return
			}

			tokenStr := parts[1]
			claims, err := jwtutil.ValidateJWTToken(tokenStr, jwtSecret)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token", "")
				return
			}

			userID, _ := claims.GetSubject()
			role, _ := claims["role"].(string)

			ctx := authctx.WithUserID(r.Context(), userID)
			ctx = authctx.WithRole(ctx, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the user ID from the request context.
// Returns an empty string if the context does not contain user ID data.
func UserIDFromContext(ctx context.Context) string {
	return authctx.UserID(ctx)
}

// UserRoleFromContext extracts the user role from the request context.
// Returns an empty string if the context does not contain role data.
func UserRoleFromContext(ctx context.Context) string {
	return authctx.Role(ctx)
}
