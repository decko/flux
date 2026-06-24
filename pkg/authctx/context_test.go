package authctx

import (
	"context"
	"testing"
)

func TestWithAndGetUserID(t *testing.T) {
	ctx := WithUserID(context.Background(), "user-42")
	if got := UserID(ctx); got != "user-42" {
		t.Errorf("UserID(ctx) = %q, want %q", got, "user-42")
	}
}

func TestUserID_Empty(t *testing.T) {
	if got := UserID(context.Background()); got != "" {
		t.Errorf("UserID(ctx) = %q, want empty string", got)
	}
}

func TestWithAndGetRole(t *testing.T) {
	ctx := WithRole(context.Background(), "admin")
	if got := Role(ctx); got != "admin" {
		t.Errorf("Role(ctx) = %q, want %q", got, "admin")
	}
}

func TestRole_Empty(t *testing.T) {
	if got := Role(context.Background()); got != "" {
		t.Errorf("Role(ctx) = %q, want empty string", got)
	}
}
