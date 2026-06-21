package model

import (
	"testing"
	"time"
)

func TestUser_Validate(t *testing.T) {
	tests := []struct {
		name    string
		user    User
		wantErr bool
	}{
		{
			name: "valid user",
			user: User{
				ID:           "user-1",
				Email:        "user@example.com",
				PasswordHash: "$2a$10$hash",
				Role:         "admin",
				CreatedAt:    time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing email",
			user: User{
				ID:           "user-1",
				PasswordHash: "$2a$10$hash",
				Role:         "admin",
			},
			wantErr: true,
		},
		{
			name: "missing password hash",
			user: User{
				ID:    "user-1",
				Email: "user@example.com",
				Role:  "admin",
			},
			wantErr: true,
		},
		{
			name: "missing role",
			user: User{
				ID:           "user-1",
				Email:        "user@example.com",
				PasswordHash: "$2a$10$hash",
			},
			wantErr: true,
		},
		{
			name:    "all fields empty",
			user:    User{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.user.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
