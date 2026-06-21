package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// mockUserRepo is an in-memory UserRepository for testing.
type mockUserRepo struct {
	mu    sync.Mutex
	store map[string]model.User
}

func newMockUserRepoAuth() *mockUserRepo {
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

// setupAuthServer creates a Server with AuthService for testing.
func setupAuthServer(t *testing.T) *Server {
	t.Helper()

	userRepo := newMockUserRepoAuth()
	authSvc := domain.NewAuthService(userRepo, []byte("test-secret"))
	return NewServer(WithAuthService(authSvc))
}

// mustDecodeAuth decodes JSON response body into the given target.
func mustDecodeAuth(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
}

func TestHandleRegister(t *testing.T) {
	srv := setupAuthServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("successful registration returns 201", func(t *testing.T) {
		body := `{"email":"newuser@example.com","password":"securePass123!"}`
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/auth/register: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusCreated)
		}

		var result map[string]interface{}
		mustDecodeAuth(t, resp, &result)

		if result["id"] == "" {
			t.Error("expected non-empty user ID")
		}
		if result["email"] != "newuser@example.com" {
			t.Errorf("got email %q, want %q", result["email"], "newuser@example.com")
		}
		if result["role"] != "user" {
			t.Errorf("got role %q, want %q", result["role"], "user")
		}
		if _, ok := result["password_hash"]; ok {
			t.Error("response should not include password_hash")
		}
	})

	t.Run("missing email returns 400", func(t *testing.T) {
		body := `{"password":"pass123"}`
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/auth/register: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		var errResp map[string]string
		mustDecodeAuth(t, resp, &errResp)
		if !strings.Contains(errResp["error"], "email") {
			t.Errorf("error message %q does not mention email", errResp["error"])
		}
	})

	t.Run("missing password returns 400", func(t *testing.T) {
		body := `{"email":"user@example.com"}`
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/auth/register: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		var errResp map[string]string
		mustDecodeAuth(t, resp, &errResp)
		if !strings.Contains(errResp["error"], "password") {
			t.Errorf("error message %q does not mention password", errResp["error"])
		}
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		body := `{bad json}`
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/auth/register: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("duplicate email returns 409", func(t *testing.T) {
		body := `{"email":"dup@example.com","password":"pass123"}`
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("first register: %v", err)
		}
		_ = resp.Body.Close()

		// Second registration with same email.
		req, _ = http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("second register: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusConflict {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusConflict)
		}
	})
}

func TestHandleLogin(t *testing.T) {
	srv := setupAuthServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// First register a user.
	registerBody := `{"email":"login@example.com","password":"myPassword1"}`
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/register", strings.NewReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	_ = resp.Body.Close()

	t.Run("successful login returns 200 with token", func(t *testing.T) {
		body := `{"email":"login@example.com","password":"myPassword1"}`
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/auth/login: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var result map[string]string
		mustDecodeAuth(t, resp, &result)
		if result["token"] == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("wrong password returns 401", func(t *testing.T) {
		body := `{"email":"login@example.com","password":"wrongPassword"}`
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/auth/login: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("nonexistent email returns 401", func(t *testing.T) {
		body := `{"email":"nonexistent@example.com","password":"pass123"}`
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/auth/login: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})
}

func TestHandleRefresh(t *testing.T) {
	srv := setupAuthServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Register and login to get a token.
	registerBody := `{"email":"refresh@example.com","password":"myPassword1"}`
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/register", strings.NewReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	_ = resp.Body.Close()

	loginBody := `{"email":"refresh@example.com","password":"myPassword1"}`
	req, _ = http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/login", strings.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	var loginResp map[string]string
	mustDecodeAuth(t, resp, &loginResp)
	_ = resp.Body.Close()

	token := loginResp["token"]

	t.Run("valid refresh returns 200 with new token", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/auth/refresh: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var result map[string]string
		mustDecodeAuth(t, resp, &result)
		if result["token"] == "" {
			t.Error("expected non-empty new token")
		}
	})

	t.Run("missing authorization header returns 401", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/refresh", nil)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/auth/refresh: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/api/v1/auth/refresh", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/auth/refresh: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})
}
