package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const (
	contextKeyUserID contextKey = "user_id"
	contextKeyRole   contextKey = "role"
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
			claims, err := validateJWT(tokenStr, jwtSecret)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token", "")
				return
			}

			userID, _ := claims.GetSubject()
			role, _ := claims["role"].(string)

			ctx := context.WithValue(r.Context(), contextKeyUserID, userID)
			ctx = context.WithValue(ctx, contextKeyRole, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the user ID from the request context.
// Returns an empty string if the context does not contain user ID data.
func UserIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKeyUserID).(string)
	return id
}

// UserRoleFromContext extracts the user role from the request context.
// Returns an empty string if the context does not contain role data.
func UserRoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(contextKeyRole).(string)
	return role
}

// validateJWT parses and validates a JWT token string using the given secret.
func validateJWT(tokenString string, jwtSecret []byte) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
