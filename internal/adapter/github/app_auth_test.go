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
	expiry := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
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
