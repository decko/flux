package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// TestAdminUserManagement_Smoke verifies the full admin user management flow:
// register → list users → change role → delete user → auth guards.
func TestAdminUserManagement_Smoke(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// 1. In-memory database with migrations.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlite")

	// 2. Create repos and services.
	userRepo := repository.NewSQLiteUserRepository(sdb)
	userSvc := domain.NewUserService(userRepo)

	// Seed an admin user.
	admin := model.User{
		ID:           "admin-1",
		Email:        "admin@flux.dev",
		PasswordHash: "$2a$10$dummy",
		Role:         "admin",
		CreatedAt:    time.Now().UTC(),
	}
	if err := userRepo.Create(ctx, admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	// Seed a regular user.
	user := model.User{
		ID:           "user-1",
		Email:        "user@flux.dev",
		PasswordHash: "$2a$10$dummy",
		Role:         "user",
		CreatedAt:    time.Now().UTC(),
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 3. Create server with UserService.
	srv := NewServer(
		WithJWTSecret(testJWTSecretBytes),
		WithUserService(userSvc),
	)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// 4. Admin lists users → 200, returns both users.
	adminReq := authedRequest(http.MethodGet, ts.URL+"/api/v1/admin/users", nil)
	resp, err := http.DefaultClient.Do(adminReq)
	if err != nil {
		t.Fatalf("GET admin users: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("list users: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var users []model.User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("got %d users, want 2", len(users))
	}

	// 5. Unauthenticated → 401.
	unauthReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/admin/users", nil)
	resp2, err := http.DefaultClient.Do(unauthReq)
	if err != nil {
		t.Fatalf("GET unauthenticated: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: got %d, want %d", resp2.StatusCode, http.StatusUnauthorized)
	}

	// 6. Non-admin → 403.
	nonAdminToken := generateNonAdminToken()
	nonAdminReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/admin/users", nil)
	nonAdminReq.Header.Set("Authorization", "Bearer "+nonAdminToken)
	resp3, err := http.DefaultClient.Do(nonAdminReq)
	if err != nil {
		t.Fatalf("GET non-admin: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()
	if resp3.StatusCode != http.StatusForbidden {
		t.Errorf("non-admin: got %d, want %d", resp3.StatusCode, http.StatusForbidden)
	}

	// 7. Admin changes user's role to admin.
	roleBody := `{"role":"admin"}`
	putReq := authedRequest(http.MethodPut, ts.URL+"/api/v1/admin/users/user-1/role", strings.NewReader(roleBody))
	putReq.Header.Set("Content-Type", "application/json")
	resp4, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("PUT role: %v", err)
	}
	defer func() { _ = resp4.Body.Close() }()
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("change role: got %d, want %d", resp4.StatusCode, http.StatusOK)
	}

	// Verify role was updated.
	updated, err := userRepo.GetByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if updated.Role != "admin" {
		t.Errorf("role = %q, want %q", updated.Role, "admin")
	}

	// 8. Admin deletes the user → 204.
	delReq := authedRequest(http.MethodDelete, ts.URL+"/api/v1/admin/users/user-1", nil)
	resp5, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE user: %v", err)
	}
	defer func() { _ = resp5.Body.Close() }()
	if resp5.StatusCode != http.StatusNoContent {
		t.Errorf("delete: got %d, want %d", resp5.StatusCode, http.StatusNoContent)
	}

	// Verify user is gone from list.
	resp6, err := http.DefaultClient.Do(adminReq)
	if err != nil {
		t.Fatalf("GET after delete: %v", err)
	}
	defer func() { _ = resp6.Body.Close() }()
	var remaining []model.User
	if err := json.NewDecoder(resp6.Body).Decode(&remaining); err != nil {
		t.Fatalf("decode remaining: %v", err)
	}
	if len(remaining) != 1 {
		t.Errorf("after delete: got %d users, want 1", len(remaining))
	}

	// Step 9: Admin creates a new user via POST /api/v1/admin/users → 201.
	createBody := `{"email":"new@flux.dev","password":"123456789012","role":"user"}`
	createReq := authedRequest(http.MethodPost, ts.URL+"/api/v1/admin/users", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	resp7, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("POST create user: %v", err)
	}
	defer func() { _ = resp7.Body.Close() }()
	if resp7.StatusCode != http.StatusCreated {
		t.Errorf("create user: got %d, want %d", resp7.StatusCode, http.StatusCreated)
	}
	var created model.User
	if err := json.NewDecoder(resp7.Body).Decode(&created); err != nil {
		t.Fatalf("decode created user: %v", err)
	}
	if created.Email != "new@flux.dev" {
		t.Errorf("created email = %q, want %q", created.Email, "new@flux.dev")
	}
	if created.ID == "" {
		t.Error("created user ID should not be empty")
	}
	if created.PasswordHash != "" {
		t.Error("created user PasswordHash should be empty in response")
	}

	// Step 10: Verify user count increased to 3.
	listReq := authedRequest(http.MethodGet, ts.URL+"/api/v1/admin/users", nil)
	resp8, err := http.DefaultClient.Do(listReq)
	if err != nil {
		t.Fatalf("GET users after create: %v", err)
	}
	defer func() { _ = resp8.Body.Close() }()
	var allUsers []model.User
	if err := json.NewDecoder(resp8.Body).Decode(&allUsers); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	if len(allUsers) != 2 {
		t.Errorf("after create: got %d users, want 2 (admin + new user)", len(allUsers))
	}

	// Step 11: Admin resets password for the new user.
	resetBody := `{"password":"newpassword1234"}`
	resetReq := authedRequest(http.MethodPut, ts.URL+"/api/v1/admin/users/"+created.ID+"/password", strings.NewReader(resetBody))
	resetReq.Header.Set("Content-Type", "application/json")
	resp9, err := http.DefaultClient.Do(resetReq)
	if err != nil {
		t.Fatalf("PUT reset password: %v", err)
	}
	defer func() { _ = resp9.Body.Close() }()
	if resp9.StatusCode != http.StatusOK {
		t.Errorf("reset password: got %d, want %d", resp9.StatusCode, http.StatusOK)
	}

	t.Log("admin user management smoke test passed")
}
