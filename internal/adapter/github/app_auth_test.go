package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// generateTestKey creates an RSA key pair and returns the PEM-encoded private
// key along with the parsed key for JWT verification in tests.
func generateTestKey(t *testing.T) (pemPrivateKey string, key *rsa.PrivateKey) {
	t.Helper()
	var err error
	key, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return string(pemBytes), key
}

// defaultTokenHandler responds 201 with a fixed test token JSON body.
func defaultTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(`{"token":"ghs_test_token","expires_at":"2026-06-24T12:00:00Z"}`))
}

// ---------------------------------------------------------------------------
// NewAppAuth
// ---------------------------------------------------------------------------

func TestNewAppAuth_ValidKey(t *testing.T) {
	pemKey, _ := generateTestKey(t)
	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth() returned error: %v", err)
	}
	if a == nil {
		t.Fatal("NewAppAuth() returned nil")
	}
	if a.appID != "12345" {
		t.Errorf("appID = %q, want %q", a.appID, "12345")
	}
}

func TestNewAppAuth_InvalidKey(t *testing.T) {
	_, err := NewAppAuth("12345", "this-is-not-a-valid-pem-block")
	if err == nil {
		t.Fatal("expected error for invalid PEM, got nil")
	}
}

func TestNewAppAuth_EmptyKey(t *testing.T) {
	_, err := NewAppAuth("12345", "")
	if err == nil {
		t.Fatal("expected error for empty private key, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetToken
// ---------------------------------------------------------------------------

func TestGetToken_Cached(t *testing.T) {
	pemKey, _ := generateTestKey(t)
	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}

	// Seed the cache with a still-valid token.
	a.cache["install_42"] = &cachedToken{
		token:     "ghs_cached_valid",
		expiresAt: time.Now().Add(10 * time.Minute),
	}

	got, err := a.GetToken(context.Background(), "install_42")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got != "ghs_cached_valid" {
		t.Errorf("got token %q, want %q", got, "ghs_cached_valid")
	}
}

func TestGetToken_Expired(t *testing.T) {
	pemKey, privKey := generateTestKey(t)

	var reqCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}

		// Verify the Authorization header carries a valid JWT.
		auth := r.Header.Get("Authorization")
		if !verifyAuthHasJWT(t, auth, privKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"token":"ghs_fresh_token","expires_at":"2026-06-24T12:00:00Z"}`))
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	// Inject an expired cached token.
	a.cache["install_42"] = &cachedToken{
		token:     "ghs_expired",
		expiresAt: time.Now().Add(-1 * time.Hour),
	}

	got, err := a.GetToken(context.Background(), "install_42")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got != "ghs_fresh_token" {
		t.Errorf("got token %q, want %q", got, "ghs_fresh_token")
	}
	if reqCount != 1 {
		t.Errorf("server handled %d requests, want 1", reqCount)
	}
}

func TestGetToken_NewInstallation(t *testing.T) {
	pemKey, _ := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(defaultTokenHandler))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	got, err := a.GetToken(context.Background(), "install_99")
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if got != "ghs_test_token" {
		t.Errorf("got token %q, want %q", got, "ghs_test_token")
	}

	// Verify the fetched token was cached.
	cached, ok := a.cache["install_99"]
	if !ok {
		t.Fatal("token was not cached after GetToken")
	}
	if cached.token != "ghs_test_token" {
		t.Errorf("cached token = %q, want %q", cached.token, "ghs_test_token")
	}
}

func TestGetToken_DifferentInstallations(t *testing.T) {
	pemKey, _ := generateTestKey(t)

	var reqCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		body := fmt.Sprintf(
			`{"token":"ghs_install_%d","expires_at":"2026-06-24T12:00:00Z"}`,
			reqCount,
		)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	t1, err := a.GetToken(context.Background(), "install_a")
	if err != nil {
		t.Fatalf("GetToken(install_a): %v", err)
	}
	t2, err := a.GetToken(context.Background(), "install_b")
	if err != nil {
		t.Fatalf("GetToken(install_b): %v", err)
	}

	if t1 == t2 {
		t.Error("tokens for different installations should be unique")
	}
	if len(a.cache) != 2 {
		t.Errorf("cache has %d entries, want 2", len(a.cache))
	}
	if reqCount != 2 {
		t.Errorf("server handled %d requests, want 2", reqCount)
	}
}

// ---------------------------------------------------------------------------
// generateJWT
// ---------------------------------------------------------------------------

func TestGenerateJWT(t *testing.T) {
	pemKey, privKey := generateTestKey(t)

	a, err := NewAppAuth("test-app-999", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}

	jwtStr, err := a.generateJWT()
	if err != nil {
		t.Fatalf("generateJWT: %v", err)
	}
	if jwtStr == "" {
		t.Fatal("generateJWT returned empty string")
	}

	// Parse and verify the JWT signature and claims.
	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(jwtStr, claims, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return &privKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("JWT parse/verify: %v", err)
	}
	if !parsed.Valid {
		t.Fatal("JWT is not valid")
	}

	// iss must match the app ID.
	if claims.Issuer != "test-app-999" {
		t.Errorf("iss = %q, want %q", claims.Issuer, "test-app-999")
	}

	// iat must be approximately now (±2 min window).
	now := time.Now()
	if claims.IssuedAt == nil {
		t.Fatal("iat claim is missing")
	}
	iat := claims.IssuedAt.Time
	if iat.Before(now.Add(-2*time.Minute)) || iat.After(now.Add(2*time.Minute)) {
		t.Errorf("iat = %v, want within 2 min of %v", iat, now)
	}

	// exp must be ~10 minutes after iat (9–11 min window).
	if claims.ExpiresAt == nil {
		t.Fatal("exp claim is missing")
	}
	expiry := claims.ExpiresAt.Sub(claims.IssuedAt.Time)
	if expiry < 9*time.Minute || expiry > 11*time.Minute {
		t.Errorf("JWT validity = %v, want ~10 minutes", expiry)
	}
}

// ---------------------------------------------------------------------------
// exchangeJWTForToken
// ---------------------------------------------------------------------------

func TestExchangeJWT_Error(t *testing.T) {
	pemKey, _ := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	_, _, err = a.exchangeJWTForToken(context.Background(), "some.invalid.jwt", "install_42")
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// ListInstallations
// ---------------------------------------------------------------------------

func TestListInstallations_SinglePage(t *testing.T) {
	pemKey, privKey := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/app/installations" {
			t.Errorf("path = %s, want /app/installations", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !verifyAuthHasJWT(t, auth, privKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Accept") != "application/vnd.github.v3+json" {
			t.Errorf("Accept header = %q, want %q", r.Header.Get("Accept"), "application/vnd.github.v3+json")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"id":1,"account->login":"user1","target_type":"User","html_url":"https://github.com/apps/myapp/installations/1"},
			{"id":2,"account->login":"org1","target_type":"Organization","html_url":"https://github.com/apps/myapp/installations/2"}
		]`))
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	installations, err := a.ListInstallations(context.Background())
	if err != nil {
		t.Fatalf("ListInstallations: %v", err)
	}
	if len(installations) != 2 {
		t.Fatalf("got %d installations, want 2", len(installations))
	}
	if installations[0].ID != 1 {
		t.Errorf("installations[0].ID = %d, want 1", installations[0].ID)
	}
	if installations[0].AccountLogin != "user1" {
		t.Errorf("installations[0].AccountLogin = %q, want %q", installations[0].AccountLogin, "user1")
	}
	if installations[1].TargetType != "Organization" {
		t.Errorf("installations[1].TargetType = %q, want %q", installations[1].TargetType, "Organization")
	}
}

func TestListInstallations_MultiplePages(t *testing.T) {
	pemKey, privKey := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/app/installations" {
			t.Errorf("path = %s, want /app/installations", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !verifyAuthHasJWT(t, auth, privKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")

		nextBase := fmt.Sprintf("http://%s", r.Host)
		switch page {
		case "", "1":
			w.Header().Set("Link", fmt.Sprintf(`<%s/app/installations?page=2>; rel="next"`, nextBase))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"id":1,"account->login":"user1","target_type":"User","html_url":"https://github.com/apps/myapp/installations/1"},
				{"id":2,"account->login":"org1","target_type":"Organization","html_url":"https://github.com/apps/myapp/installations/2"}
			]`))
		case "2":
			w.Header().Set("Link", fmt.Sprintf(`<%s/app/installations?page=3>; rel="next"`, nextBase))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"id":3,"account->login":"user2","target_type":"User","html_url":"https://github.com/apps/myapp/installations/3"},
				{"id":4,"account->login":"org2","target_type":"Organization","html_url":"https://github.com/apps/myapp/installations/4"}
			]`))
		case "3":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"id":5,"account->login":"user3","target_type":"User","html_url":"https://github.com/apps/myapp/installations/5"},
				{"id":6,"account->login":"org3","target_type":"Organization","html_url":"https://github.com/apps/myapp/installations/6"}
			]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	installations, err := a.ListInstallations(context.Background())
	if err != nil {
		t.Fatalf("ListInstallations: %v", err)
	}
	if len(installations) != 6 {
		t.Fatalf("got %d installations, want 6", len(installations))
	}
	// Verify all page IDs are present.
	ids := make(map[int64]bool)
	for _, inst := range installations {
		ids[inst.ID] = true
	}
	for _, id := range []int64{1, 2, 3, 4, 5, 6} {
		if !ids[id] {
			t.Errorf("missing installation ID %d", id)
		}
	}
}

func TestListInstallations_Empty(t *testing.T) {
	pemKey, privKey := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/app/installations" {
			t.Errorf("path = %s, want /app/installations", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !verifyAuthHasJWT(t, auth, privKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	installations, err := a.ListInstallations(context.Background())
	if err != nil {
		t.Fatalf("ListInstallations: %v", err)
	}
	if len(installations) != 0 {
		t.Fatalf("got %d installations, want 0", len(installations))
	}
}

func TestListInstallations_HTTPError(t *testing.T) {
	pemKey, _ := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	_, err = a.ListInstallations(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}

// ---------------------------------------------------------------------------
// ListInstallationRepositories
// ---------------------------------------------------------------------------

func TestListInstallationRepositories_SinglePage(t *testing.T) {
	pemKey, privKey := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/app/installations/install_42/repositories" {
			t.Errorf("path = %s, want /app/installations/install_42/repositories", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !verifyAuthHasJWT(t, auth, privKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"id":101,"name":"repo-a","full_name":"user1/repo-a","html_url":"https://github.com/user1/repo-a","private":false},
			{"id":102,"name":"repo-b","full_name":"org1/repo-b","html_url":"https://github.com/org1/repo-b","private":true}
		]`))
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	repos, err := a.ListInstallationRepositories(context.Background(), "install_42")
	if err != nil {
		t.Fatalf("ListInstallationRepositories: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("got %d repos, want 2", len(repos))
	}
	if repos[0].ID != 101 {
		t.Errorf("repos[0].ID = %d, want 101", repos[0].ID)
	}
	if repos[0].Name != "repo-a" {
		t.Errorf("repos[0].Name = %q, want %q", repos[0].Name, "repo-a")
	}
	if repos[1].Private != true {
		t.Errorf("repos[1].Private = %t, want true", repos[1].Private)
	}
}

func TestListInstallationRepositories_MultiplePages(t *testing.T) {
	pemKey, privKey := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/app/installations/install_42/repositories" {
			t.Errorf("path = %s, want /app/installations/install_42/repositories", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !verifyAuthHasJWT(t, auth, privKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")

		nextBase := fmt.Sprintf("http://%s", r.Host)
		switch page {
		case "", "1":
			w.Header().Set("Link", fmt.Sprintf(`<%s/app/installations/install_42/repositories?page=2>; rel="next"`, nextBase))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"id":101,"name":"repo-a","full_name":"user1/repo-a","html_url":"https://github.com/user1/repo-a","private":false}
			]`))
		case "2":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"id":102,"name":"repo-b","full_name":"org1/repo-b","html_url":"https://github.com/org1/repo-b","private":true}
			]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	repos, err := a.ListInstallationRepositories(context.Background(), "install_42")
	if err != nil {
		t.Fatalf("ListInstallationRepositories: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("got %d repos, want 2", len(repos))
	}
	// Verify both page IDs are present.
	ids := make(map[int64]bool)
	for _, r := range repos {
		ids[r.ID] = true
	}
	for _, id := range []int64{101, 102} {
		if !ids[id] {
			t.Errorf("missing repository ID %d", id)
		}
	}
}

func TestListInstallationRepositories_Empty(t *testing.T) {
	pemKey, privKey := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/app/installations/install_42/repositories" {
			t.Errorf("path = %s, want /app/installations/install_42/repositories", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !verifyAuthHasJWT(t, auth, privKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	repos, err := a.ListInstallationRepositories(context.Background(), "install_42")
	if err != nil {
		t.Fatalf("ListInstallationRepositories: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("got %d repos, want 0", len(repos))
	}
}

func TestListInstallationRepositories_HTTPError(t *testing.T) {
	pemKey, _ := generateTestKey(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not found"}`))
	}))
	defer srv.Close()

	a, err := NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	a.httpClient = srv.Client()
	a.baseURL = srv.URL

	_, err = a.ListInstallationRepositories(context.Background(), "install_42")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

// verifyAuthHasJWT checks that auth contains "Bearer " + a valid JWT signed
// by privKey. Returns true if the JWT is valid.
func verifyAuthHasJWT(t *testing.T, auth string, privKey *rsa.PrivateKey) bool {
	t.Helper()

	const prefix = "Bearer "
	if len(auth) <= len(prefix) {
		t.Error("Authorization header too short or missing")
		return false
	}
	tokenStr := auth[len(prefix):]

	claims := &jwt.RegisteredClaims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return &privKey.PublicKey, nil
	})
	if err != nil {
		t.Errorf("JWT verification failed: %v", err)
		return false
	}
	return parsed.Valid
}
