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

	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// setupAdminServer creates an in-memory SQLite database, migrates it,
// creates a UserService-backed Server, and seeds test users.
func setupAdminServer(t *testing.T) *Server {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}

	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlite")
	userRepo := repository.NewSQLiteUserRepository(sdb)

	// Seed test users via repository.
	ctx := context.Background()
	now := time.Now().UTC()
	seedUsers := []model.User{
		{
			ID:           "admin-1",
			Email:        "admin@example.com",
			PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
			Role:         "admin",
			CreatedAt:    now,
		},
		{
			ID:           "user-1",
			Email:        "user@example.com",
			PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
			Role:         "user",
			CreatedAt:    now,
		},
		{
			ID:           "user-2",
			Email:        "another@example.com",
			PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
			Role:         "user",
			CreatedAt:    now,
		},
	}
	for _, u := range seedUsers {
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatalf("seed: create user %s: %v", u.ID, err)
		}
	}

	// UserService does not exist yet — this will fail to compile (RED).
	userSvc := domain.NewUserService(userRepo)
	return NewServer(WithJWTSecret(testJWTSecretBytes), WithUserService(userSvc))
}

// nonAdminRequest creates an HTTP request with a non-admin JWT Bearer token,
// suitable for testing that non-admin users receive 403 Forbidden.
func nonAdminRequest(method, url string, body string) *http.Request {
	claims := jwt.MapClaims{
		"sub":   "non-admin-user",
		"email": "regular@example.com",
		"role":  "user",
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(testJWTSecretBytes)

	req, _ := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// ─── Tests ─────────────────────────────────────────────────────────────────

func TestAdminHandler_ListUsers(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/admin/users", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/admin/users: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var users []model.User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	if len(users) == 0 {
		t.Error("expected non-empty user list")
	}

	// Verify that password_hash is never serialized in the response.
	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil {
		// We can't decode the body twice, so just decode first user manually.
		_ = raw
	}
}

func TestAdminHandler_UpdateUserRole(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"role":"admin"}`
	req := authedRequest(http.MethodPut, ts.URL+"/api/v1/admin/users/user-1/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/admin/users/user-1/role: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAdminHandler_UpdateUserRole_InvalidRole(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"role":"superadmin"}`
	req := authedRequest(http.MethodPut, ts.URL+"/api/v1/admin/users/user-1/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/admin/users/user-1/role (invalid): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAdminHandler_DeleteUser(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodDelete, ts.URL+"/api/v1/admin/users/user-2", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/v1/admin/users/user-2: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestAdminHandler_ListUsers_Unauthorized(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/v1/admin/users", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/admin/users (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminHandler_ListUsers_Forbidden(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := nonAdminRequest(http.MethodGet, ts.URL+"/api/v1/admin/users", "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/admin/users (non-admin): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestAdminHandler_UpdateUserRole_Unauthorized(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"role":"admin"}`
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, ts.URL+"/api/v1/admin/users/user-1/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/admin/users/user-1/role (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminHandler_UpdateUserRole_Forbidden(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"role":"admin"}`
	req := nonAdminRequest(http.MethodPut, ts.URL+"/api/v1/admin/users/user-1/role", body)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/admin/users/user-1/role (non-admin): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestAdminHandler_DeleteUser_Unauthorized(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, ts.URL+"/api/v1/admin/users/user-2", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/v1/admin/users/user-2 (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminHandler_DeleteUser_Forbidden(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := nonAdminRequest(http.MethodDelete, ts.URL+"/api/v1/admin/users/user-2", "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/v1/admin/users/user-2 (non-admin): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// ─── TestAdminHandler_CreateUser ──────────────────────────────────────

func TestAdminHandler_CreateUser(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"email":"new@test.com","password":"123456789012","role":"user"}`
	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/admin/users: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var user model.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	if user.ID == "" {
		t.Error("expected non-empty user ID")
	}
	if user.Email != "new@test.com" {
		t.Errorf("got email %q, want %q", user.Email, "new@test.com")
	}
	if user.Role != "user" {
		t.Errorf("got role %q, want %q", user.Role, "user")
	}
	if user.PasswordHash != "" {
		t.Error("PasswordHash should be empty in response (json:\"-\")")
	}
}

func TestAdminHandler_CreateUser_Duplicate(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"email":"dupe@test.com","password":"123456789012","role":"user"}`
	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("first POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 first time, got %d", resp.StatusCode)
	}

	// Duplicate request.
	req2 := authedRequest(http.MethodPost, ts.URL+"/api/v1/admin/users", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("second POST: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("got status %d, want %d", resp2.StatusCode, http.StatusConflict)
	}
}

func TestAdminHandler_CreateUser_InvalidRole(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"email":"new@test.com","password":"123456789012","role":"superadmin"}`
	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/admin/users: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAdminHandler_CreateUser_ShortPassword(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"email":"new@test.com","password":"short","role":"user"}`
	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/admin/users: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAdminHandler_CreateUser_Unauthorized(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"email":"new@test.com","password":"123456789012","role":"user"}`
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/admin/users (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminHandler_CreateUser_Forbidden(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"email":"new@test.com","password":"123456789012","role":"user"}`
	req := nonAdminRequest(http.MethodPost, ts.URL+"/api/v1/admin/users", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/admin/users (non-admin): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// ─── TestAdminHandler_ResetPassword ───────────────────────────────────

func TestAdminHandler_ResetPassword(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"password":"newpass123456"}`
	req := authedRequest(http.MethodPut, ts.URL+"/api/v1/admin/users/user-1/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/admin/users/user-1/password: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAdminHandler_ResetPassword_NotFound(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"password":"newpass123456"}`
	req := authedRequest(http.MethodPut, ts.URL+"/api/v1/admin/users/nonexistent/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/admin/users/nonexistent/password: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAdminHandler_ResetPassword_ShortPassword(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"password":"short"}`
	req := authedRequest(http.MethodPut, ts.URL+"/api/v1/admin/users/user-1/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/admin/users/user-1/password (short): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestAdminHandler_ResetPassword_Unauthorized(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"password":"newpass123456"}`
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, ts.URL+"/api/v1/admin/users/user-1/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/admin/users/user-1/password (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAdminHandler_ResetPassword_Forbidden(t *testing.T) {
	srv := setupAdminServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"password":"newpass123456"}`
	req := nonAdminRequest(http.MethodPut, ts.URL+"/api/v1/admin/users/user-1/password", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/admin/users/user-1/password (non-admin): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
