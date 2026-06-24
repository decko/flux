// Package authctx provides context key helpers for extracting authentication
// information (user ID, role) from request contexts. Services use these
// helpers instead of importing internal API packages.
package authctx

import "context"

// contextKey is an unexported type for context value keys to avoid collisions.
type contextKey string

const (
	keyUserID contextKey = "user_id"
	keyRole   contextKey = "role"
)

// WithUserID returns a new context with the given user ID stored in it.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, keyUserID, userID)
}

// UserID extracts the user ID from the context. Returns empty string if no
// user ID is set.
func UserID(ctx context.Context) string {
	v, _ := ctx.Value(keyUserID).(string)
	return v
}

// WithRole returns a new context with the given role stored in it.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, keyRole, role)
}

// Role extracts the role from the context. Returns empty string if no role
// is set.
func Role(ctx context.Context) string {
	v, _ := ctx.Value(keyRole).(string)
	return v
}
