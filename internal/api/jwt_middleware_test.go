package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddleware(t *testing.T) {
	secret := []byte("test-secret")
	middleware := AuthMiddleware(secret)

	t.Run("valid token passes", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(middleware)
		r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
			userID := UserIDFromContext(r.Context())
			role := UserRoleFromContext(r.Context())
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"user_id": userID,
				"role":    role,
			})
		})

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":   "user-1",
			"email": "user@example.com",
			"role":  "admin",
		})
		tokenStr, err := token.SignedString(secret)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		ts := httptest.NewServer(r)
		defer ts.Close()

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /protected: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["user_id"] != "user-1" {
			t.Errorf("got user_id %q, want %q", body["user_id"], "user-1")
		}
		if body["role"] != "admin" {
			t.Errorf("got role %q, want %q", body["role"], "admin")
		}
	})

	t.Run("missing authorization header returns 401", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(middleware)
		r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		ts := httptest.NewServer(r)
		defer ts.Close()

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/protected", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /protected: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(middleware)
		r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		ts := httptest.NewServer(r)
		defer ts.Close()

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /protected: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("expired token returns 401", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(middleware)
		r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Use a specific past timestamp.
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":   "user-1",
			"email": "user@example.com",
			"role":  "admin",
			"exp":   float64(1516239022), // January 17, 2018 — definitely expired
		})
		tokenStr, err := token.SignedString(secret)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		ts := httptest.NewServer(r)
		defer ts.Close()

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /protected: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("wrong signing key returns 401", func(t *testing.T) {
		r := chi.NewRouter()
		r.Use(AuthMiddleware([]byte("different-secret")))
		r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub":   "user-1",
			"email": "user@example.com",
			"role":  "admin",
		})
		tokenStr, err := token.SignedString(secret) // signed with test-secret
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		ts := httptest.NewServer(r)
		defer ts.Close()

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /protected: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})
}

func TestUserIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	if id := UserIDFromContext(ctx); id != "" {
		t.Errorf("got %q, want empty string", id)
	}
}

func TestUserRoleFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	if role := UserRoleFromContext(ctx); role != "" {
		t.Errorf("got %q, want empty string", role)
	}
}
