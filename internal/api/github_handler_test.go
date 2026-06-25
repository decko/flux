package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/decko/flux/internal/adapter/github"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// generateTestKeyGH creates an RSA key pair and returns the PEM-encoded
// private key for use in GitHub AppAuth tests.
func generateTestKeyGH(t *testing.T) (pemPrivateKey string, key *rsa.PrivateKey) {
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

// setupGitHubServer creates a mock GitHub API server (using the given handler),
// a real AppAuth pointed at the mock server, and a flux Server wired to the
// AppAuth. Returns the mock server and flux Server. Callers must close the
// mock server and the httptest.Server wrapping the flux Server.
//
// Note: AppAuth's httpClient and baseURL are unexported fields in the github
// package. The go-coder implementing the handler will need to either export
// setter methods on AppAuth or configure them via the github package's own
// test helpers. In the RED phase, the mock server is created for documentation
// but is not wired into AppAuth — the stub handler returns 501 regardless.
func setupGitHubServer(t *testing.T, mockHandler http.HandlerFunc) (*httptest.Server, *Server) {
	t.Helper()
	mockGH := httptest.NewServer(mockHandler)

	pemKey, _ := generateTestKeyGH(t)
	appAuth, err := github.NewAppAuth("12345", pemKey)
	if err != nil {
		mockGH.Close()
		t.Fatalf("NewAppAuth: %v", err)
	}

	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithAppAuth(appAuth))
	return mockGH, srv
}

// ---------------------------------------------------------------------------
// GET /api/v1/github/installations
// ---------------------------------------------------------------------------

func TestHandleGitHubInstallations_Success(t *testing.T) {
	mockGH, srv := setupGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/app/installations" {
			t.Errorf("path = %s, want /app/installations", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
			{"id":1,"account":{"login":"user1"},"target_type":"User","html_url":"https://github.com/apps/myapp/installations/1"},
			{"id":2,"account":{"login":"org1"},"target_type":"Organization","html_url":"https://github.com/apps/myapp/installations/2"}
		]`))
	})
	defer mockGH.Close()

	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/github/installations", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/github/installations: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var installations []github.Installation
	mustDecode(t, resp, &installations)

	if len(installations) != 2 {
		t.Fatalf("got %d installations, want 2", len(installations))
	}
	if installations[0].ID != 1 {
		t.Errorf("installations[0].ID = %d, want 1", installations[0].ID)
	}
	if installations[0].Account.Login != "user1" {
		t.Errorf("installations[0].Account.Login = %q, want %q", installations[0].Account.Login, "user1")
	}
	if installations[1].TargetType != "Organization" {
		t.Errorf("installations[1].TargetType = %q, want %q", installations[1].TargetType, "Organization")
	}
}

func TestHandleGitHubInstallations_NoAuth(t *testing.T) {
	mockGH, srv := setupGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be reached without auth")
		w.WriteHeader(http.StatusOK)
	})
	defer mockGH.Close()

	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/v1/github/installations", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/github/installations (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestHandleGitHubInstallations_AppNotConfigured(t *testing.T) {
	srv := NewServer(WithJWTSecret(testJWTSecretBytes))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/github/installations", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/github/installations: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

// ---------------------------------------------------------------------------
// GET /api/v1/github/installations/{id}/repositories
// ---------------------------------------------------------------------------

func TestHandleGitHubInstallationRepos_Success(t *testing.T) {
	mockGH, srv := setupGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/installation/repositories" {
			t.Errorf("path = %s, want /installation/repositories", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"total_count":2,
			"repositories":[
				{"id":101,"name":"repo-a","full_name":"user1/repo-a","html_url":"https://github.com/user1/repo-a","private":false},
				{"id":102,"name":"repo-b","full_name":"org1/repo-b","html_url":"https://github.com/org1/repo-b","private":true}
			]
		}`))
	})
	defer mockGH.Close()

	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/github/installations/42/repositories", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/github/installations/42/repositories: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var repos []github.InstallationRepository
	mustDecode(t, resp, &repos)

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

func TestHandleGitHubInstallationRepos_InvalidID(t *testing.T) {
	mockGH, srv := setupGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be reached for invalid installation ID")
		w.WriteHeader(http.StatusOK)
	})
	defer mockGH.Close()

	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/github/installations/abc/repositories", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/github/installations/abc/repositories: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleGitHubInstallationRepos_NoAuth(t *testing.T) {
	mockGH, srv := setupGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be reached without auth")
		w.WriteHeader(http.StatusOK)
	})
	defer mockGH.Close()

	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/v1/github/installations/42/repositories", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/github/installations/42/repositories (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestHandleGitHubInstallationRepos_AppNotConfigured(t *testing.T) {
	srv := NewServer(WithJWTSecret(testJWTSecretBytes))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/github/installations/42/repositories", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/github/installations/42/repositories: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}
