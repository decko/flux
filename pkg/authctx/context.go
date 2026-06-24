// Package authctx provides context helpers for storing and retrieving
// authentication-related values (user ID, role) in Go contexts.
package authctx

import "context"

// unexported type for context keys to avoid collisions.
type ctxKey string

const (
	keyUserID ctxKey = "user_id"
	keyRole   ctxKey = "role"
)

// WithUserID stores the given user ID in ctx and returns the new context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, keyUserID, userID)
}

// UserID extracts the user ID from ctx. Returns an empty string if not set.
func UserID(ctx context.Context) string {
	id, _ := ctx.Value(keyUserID).(string)
	return id
}

// WithRole stores the given role in ctx and returns the new context.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, keyRole, role)
}

// Role extracts the role from ctx. Returns an empty string if not set.
func Role(ctx context.Context) string {
	r, _ := ctx.Value(keyRole).(string)
	return r
}
