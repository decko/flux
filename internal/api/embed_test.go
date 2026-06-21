package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSPA_ServesRootIndexHTML(t *testing.T) {
	srv := NewServer(WithSPA())
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `<div id="root">`) {
		t.Errorf("response body missing SPA root element")
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("got Content-Type %q, want text/html", ct)
	}
}

func TestSPA_ServesStaticAssets(t *testing.T) {
	srv := NewServer(WithSPA())
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/favicon.svg", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /favicon.svg: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/svg+xml") {
		t.Errorf("got Content-Type %q, want image/svg+xml", ct)
	}
}

func TestSPA_Fallback_ServesIndexHTML(t *testing.T) {
	srv := NewServer(WithSPA())
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/projects", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /projects: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d for SPA fallback", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `<div id="root">`) {
		t.Errorf("SPA fallback response missing SPA root element")
	}
}

func TestSPA_HealthEndpointStillWorks(t *testing.T) {
	srv := NewServer(WithSPA())
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d for /health", resp.StatusCode, http.StatusOK)
	}
}

func TestWithoutSPA_RootReturns404(t *testing.T) {
	srv := NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d for / without SPA", resp.StatusCode, http.StatusNotFound)
	}
}

func TestSPA_APIRoutesReturnJSON404(t *testing.T) {
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithSPA())
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/nonexistent", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/nonexistent: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("got Content-Type %q, want application/json", ct)
	}
}
