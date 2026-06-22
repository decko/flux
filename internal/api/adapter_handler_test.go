package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/decko/flux/internal/domain"
)

// adapterInfoResponse is the JSON response shape for adapter information.
type adapterInfoResponse struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Health string `json:"health"`
}

// setupAdapterServer creates a Server with mock adapters for testing.
func setupAdapterServer(t *testing.T) *Server {
	t.Helper()

	adapters := map[string]domain.AdapterInfo{
		"github": {
			Type:   "github",
			Name:   "GitHub",
			Health: "healthy",
		},
		"jira": {
			Type:   "jira",
			Name:   "Jira",
			Health: "healthy",
		},
	}

	return NewServer(WithJWTSecret(testJWTSecretBytes), WithAdapters(adapters))
}

// ─── List Adapters ────────────────────────────────────────────────────────

func TestHandleListAdapters(t *testing.T) {
	srv := setupAdapterServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/adapters", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/adapters: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var adapters []adapterInfoResponse
	mustDecode(t, resp, &adapters)

	if len(adapters) == 0 {
		t.Fatal("expected at least one adapter")
	}

	found := make(map[string]bool)
	for _, a := range adapters {
		found[a.Type] = true
		if a.Name == "" {
			t.Errorf("adapter type %q has empty name", a.Type)
		}
		if a.Health == "" {
			t.Errorf("adapter type %q has empty health", a.Type)
		}
	}
	if !found["github"] {
		t.Error("expected github adapter in list")
	}
	if !found["jira"] {
		t.Error("expected jira adapter in list")
	}
}

// ─── Adapter Health ───────────────────────────────────────────────────────

func TestHandleAdapterHealth(t *testing.T) {
	srv := setupAdapterServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/adapters/github/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/adapters/github/health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var info adapterInfoResponse
	mustDecode(t, resp, &info)
	if info.Type != "github" {
		t.Errorf("got type %q, want %q", info.Type, "github")
	}
	if info.Name != "GitHub" {
		t.Errorf("got name %q, want %q", info.Name, "GitHub")
	}
	if info.Health != "healthy" {
		t.Errorf("got health %q, want %q", info.Health, "healthy")
	}
}

// ─── Unknown Adapter Type ─────────────────────────────────────────────────

func TestHandleAdapterHealth_UnknownType(t *testing.T) {
	srv := setupAdapterServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/adapters/unknown/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/adapters/unknown/health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var errResp map[string]string
	mustDecode(t, resp, &errResp)
	if _, ok := errResp["error"]; !ok {
		t.Error("JSON response missing 'error' field")
	}
}

// ─── Adapters Not Configured ──────────────────────────────────────────────

func TestHandleAdapterHealth_NotConfigured(t *testing.T) {
	srv := NewServer(WithJWTSecret(testJWTSecretBytes))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/adapters/github/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/adapters/github/health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

// ─── Unauthorized ─────────────────────────────────────────────────────────

func TestHandleListAdapters_Unauthorized(t *testing.T) {
	srv := setupAdapterServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/v1/adapters", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/adapters (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
