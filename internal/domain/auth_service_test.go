package domain_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Mock: UserRepository ──────────────────────────────────────────────

type mockUserRepo struct {
	mu    sync.Mutex
	store map[string]model.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{store: make(map[string]model.User)}
}

func (r *mockUserRepo) Create(_ context.Context, u model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.store {
		if existing.Email == u.Email {
			return repository.ErrDuplicateEmail
		}
	}
	r.store[u.ID] = u
	return nil
}

func (r *mockUserRepo) GetByEmail(_ context.Context, email string) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.store {
		if u.Email == email {
			return u, nil
		}
	}
	return model.User{}, repository.ErrNotFound
}

func (r *mockUserRepo) GetByID(_ context.Context, id string) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, exists := r.store[id]
	if !exists {
		return model.User{}, repository.ErrNotFound
	}
	return u, nil
}

func (r *mockUserRepo) Update(_ context.Context, u model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[u.ID]; !exists {
		return repository.ErrNotFound
	}
	r.store[u.ID] = u
	return nil
}

func (r *mockUserRepo) List(_ context.Context) ([]model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]model.User, 0, len(r.store))
	for _, u := range r.store {
		result = append(result, u)
	}
	return result, nil
}

func (r *mockUserRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[id]; !exists {
		return repository.ErrNotFound
	}
	delete(r.store, id)
	return nil
}

func (r *mockUserRepo) Count(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.store), nil
}

func (r *mockUserRepo) CountByRole(_ context.Context, role string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, u := range r.store {
		if u.Role == role {
			count++
		}
	}
	return count, nil
}

// ─── Tests ─────────────────────────────────────────────────────────────

func TestAuthService_Register(t *testing.T) {
	repo := newMockUserRepo()
	svc := domain.NewAuthService(repo, []byte("test-secret"))
	ctx := context.Background()

	user, err := svc.Register(ctx, "newuser@example.com", "securePass123!")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if user.ID == "" {
		t.Error("expected non-empty user ID")
	}
	if user.Email != "newuser@example.com" {
		t.Errorf("got email %q, want %q", user.Email, "newuser@example.com")
	}
	// First registered user gets admin role (first-user bootstrap).
	if user.Role != "admin" {
		t.Errorf("got role %q, want %q", user.Role, "admin")
	}
	if user.PasswordHash != "" {
		t.Error("PasswordHash should be empty in returned user (json:\"-\")")
	}

	// Verify password hash is stored correctly.
	stored, err := repo.GetByEmail(ctx, "newuser@example.com")
	if err != nil {
		t.Fatalf("GetByEmail returned error: %v", err)
	}
	if stored.PasswordHash == "" {
		t.Fatal("expected non-empty password hash in store")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("securePass123!")); err != nil {
		t.Error("stored password hash does not match original password")
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	repo := newMockUserRepo()
	svc := domain.NewAuthService(repo, []byte("test-secret"))
	ctx := context.Background()

	_, err := svc.Register(ctx, "dup@example.com", "password123456")
	if err != nil {
		t.Fatalf("first register returned error: %v", err)
	}

	_, err = svc.Register(ctx, "dup@example.com", "password456789")
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}
	if !errors.Is(err, repository.ErrDuplicateEmail) {
		t.Errorf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestAuthService_Register_ValidationError(t *testing.T) {
	repo := newMockUserRepo()
	svc := domain.NewAuthService(repo, []byte("test-secret"))
	ctx := context.Background()

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{name: "empty email", email: "", password: "pass123"},
		{name: "empty password", email: "user@example.com", password: ""},
		{name: "invalid email", email: "not-an-email", password: "pass123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Register(ctx, tt.email, tt.password)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	repo := newMockUserRepo()
	svc := domain.NewAuthService(repo, []byte("test-secret"))
	ctx := context.Background()

	_, err := svc.Register(ctx, "login@example.com", "myPassword12345")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	token, err := svc.Login(ctx, "login@example.com", "myPassword12345")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty JWT token")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	repo := newMockUserRepo()
	svc := domain.NewAuthService(repo, []byte("test-secret"))
	ctx := context.Background()

	_, err := svc.Register(ctx, "login@example.com", "correctPassword")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	_, err = svc.Login(ctx, "login@example.com", "wrongPassword")
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	repo := newMockUserRepo()
	svc := domain.NewAuthService(repo, []byte("test-secret"))
	ctx := context.Background()

	_, err := svc.Login(ctx, "nonexistent@example.com", "pass123")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAuthService_RefreshToken(t *testing.T) {
	repo := newMockUserRepo()
	svc := domain.NewAuthService(repo, []byte("test-secret"))
	ctx := context.Background()

	user, err := svc.Register(ctx, "refresh@example.com", "myPassword12345")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	token, err := svc.Login(ctx, "refresh@example.com", "myPassword12345")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	refreshed, err := svc.RefreshToken(ctx, token)
	if err != nil {
		t.Fatalf("RefreshToken returned error: %v", err)
	}
	if refreshed == "" {
		t.Fatal("expected non-empty refreshed token")
	}

	// Refreshed token should be valid for login equivalent.
	refreshedUser, err := svc.RefreshToken(ctx, refreshed)
	if err != nil {
		t.Fatal("refreshed token should itself be refreshable")
	}
	_ = refreshedUser
	_ = user
}

func TestAuthService_RefreshToken_InvalidToken(t *testing.T) {
	repo := newMockUserRepo()
	svc := domain.NewAuthService(repo, []byte("test-secret"))
	ctx := context.Background()

	_, err := svc.RefreshToken(ctx, "invalid-token-string")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
}

func TestAuthService_RefreshToken_ExpiredToken(t *testing.T) {
	repo := newMockUserRepo()
	svc := domain.NewAuthService(repo, []byte("test-secret"))
	ctx := context.Background()

	// Create an expired token by manipulating time isn't possible
	// without a clock interface. Instead, test that a tampered token fails.
	_, err := svc.RefreshToken(ctx, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZXhwIjoiMTUxNjIzOTAyMn0.tampered")
	if err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}

// ─── TestAuthService_Register_ShortPassword ───────────────────────────

func TestAuthService_Register_ShortPassword(t *testing.T) {
	repo := newMockUserRepo()
	svc := domain.NewAuthService(repo, []byte("test-secret"))
	ctx := context.Background()

	_, err := svc.Register(ctx, "user@example.com", "short")
	if err == nil {
		t.Fatal("expected error for short password, got nil")
	}
}

// TestUserPasswordNeverSerialized ensures PasswordHash has json:"-" tag.
func TestUserPasswordNeverSerialized(t *testing.T) {
	u := model.User{
		ID:           "u1",
		Email:        "test@example.com",
		PasswordHash: "secret-hash",
		Role:         "admin",
		CreatedAt:    time.Now(),
	}

	data, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}

	if _, ok := decoded["password_hash"]; ok {
		t.Error("PasswordHash should not appear in JSON output (json:\"-\")")
	}
	if _, ok := decoded["PasswordHash"]; ok {
		t.Error("PasswordHash should not appear in JSON output (json:\"-\")")
	}
}
