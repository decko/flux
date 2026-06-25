package github

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Default GitHub REST API base URL.
const defaultBaseURL = "https://api.github.com"

// Installation represents a GitHub App installation.
type Installation struct {
	ID           int64  `json:"id"`
	AccountLogin string `json:"account->login"`
	TargetType   string `json:"target_type"`
	HTMLURL      string `json:"html_url"`
}

// InstallationRepository represents a repository accessible via an installation.
type InstallationRepository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
	Private  bool   `json:"private"`
}

// AppAuth handles GitHub App authentication. It generates signed JWTs from a
// private key and exchanges them for short-lived installation access tokens,
// caching tokens until they are close to expiry.
type AppAuth struct {
	appID      string
	privateKey *rsa.PrivateKey
	cache      map[string]*cachedToken
	mu         sync.RWMutex
	httpClient *http.Client
	baseURL    string
}

// cachedToken holds an installation access token and its expiry time.
type cachedToken struct {
	token     string
	expiresAt time.Time
}

// NewAppAuth creates a new AppAuth from a GitHub App ID and a PEM-encoded RSA
// private key. The PEM block must contain an RSA private key in PKCS1 or PKCS8
// format. Returns an error if the PEM data is missing, malformed, or the key
// type is not RSA.
func NewAppAuth(appID string, privateKeyPEM string) (*AppAuth, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("github app auth: no PEM block found")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Fall back to PKCS8.
		parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("github app auth: parse private key: %w", err)
		}
		var ok bool
		key, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("github app auth: key is not RSA")
		}
	}

	return &AppAuth{
		appID:      appID,
		privateKey: key,
		cache:      make(map[string]*cachedToken),
		httpClient: http.DefaultClient,
		baseURL:    defaultBaseURL,
	}, nil
}

// GetToken returns a valid installation access token for the given GitHub App
// installation ID. It returns a cached token if one exists and its expiry is
// more than 5 minutes away; otherwise it generates a new JWT, exchanges it
// with the GitHub API, caches the fresh token, and returns it.
func (a *AppAuth) GetToken(ctx context.Context, installationID string) (string, error) {
	// Fast path — check cache under read lock.
	a.mu.RLock()
	cached, ok := a.cache[installationID]
	a.mu.RUnlock()
	if ok && time.Now().Add(5*time.Minute).Before(cached.expiresAt) {
		return cached.token, nil
	}

	// Slow path — acquire write lock, double-check, then refresh.
	a.mu.Lock()
	cached, ok = a.cache[installationID]
	if ok && time.Now().Add(5*time.Minute).Before(cached.expiresAt) {
		a.mu.Unlock()
		return cached.token, nil
	}

	jwtToken, err := a.generateJWT()
	if err != nil {
		a.mu.Unlock()
		return "", fmt.Errorf("generate JWT: %w", err)
	}

	token, expiresAt, err := a.exchangeJWTForToken(ctx, jwtToken, installationID)
	if err != nil {
		a.mu.Unlock()
		return "", fmt.Errorf("exchange JWT: %w", err)
	}

	a.cache[installationID] = &cachedToken{token: token, expiresAt: expiresAt}
	a.mu.Unlock()
	return token, nil
}

// generateJWT creates a signed RS256 JWT with the app ID as issuer, valid for
// 10 minutes from the current time.
func (a *AppAuth) generateJWT() (string, error) {
	now := time.Now()
	claims := &jwt.RegisteredClaims{
		Issuer:    a.appID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(a.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	return signed, nil
}

// exchangeJWTForToken sends a POST request to the GitHub API to exchange a JWT
// for an installation access token. It returns the token string and its expiry
// time.
func (a *AppAuth) exchangeJWTForToken(ctx context.Context, jwtStr, installationID string) (string, time.Time, error) {
	url := a.baseURL + "/app/installations/" + installationID + "/access_tokens"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, http.NoBody)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwtStr)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", time.Time{}, fmt.Errorf("decode response: %w", err)
	}

	expiresAt, err := time.Parse(time.RFC3339, result.ExpiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse expires_at: %w", err)
	}

	return result.Token, expiresAt, nil
}
