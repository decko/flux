package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/decko/flux/internal/config"
)

func TestSetupServer_Success(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-16-chars!")
	ctx := context.Background()
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8080},
		Database: config.DatabaseConfig{Path: ":memory:"},
		CORS:     config.CORSConfig{Origin: "*"},
		Logging:  config.LoggingConfig{Level: "info"},
	}

	srv, cleanup, err := setupServer(ctx, cfg)
	if err != nil {
		t.Fatalf("setupServer() error = %v", err)
	}
	if srv == nil {
		t.Fatal("setupServer() returned nil server")
	}
	if cleanup == nil {
		t.Fatal("setupServer() returned nil cleanup function")
	}

	// Verify the cleanup function does not panic when called.
	cleanup()
}

func TestSetupServer_HealthEndpoint(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-16-chars!")
	ctx := context.Background()
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8080},
		Database: config.DatabaseConfig{Path: ":memory:"},
		CORS:     config.CORSConfig{Origin: "*"},
		Logging:  config.LoggingConfig{Level: "info"},
	}

	srv, cleanup, err := setupServer(ctx, cfg)
	if err != nil {
		t.Fatalf("setupServer() error = %v", err)
	}
	t.Cleanup(cleanup)

	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/health", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body := readAll(t, resp)
	if strings.TrimSpace(body) != "ok" {
		t.Errorf("got body %q, want %q", body, "ok")
	}
}

func TestSetupServer_InvalidDBPath(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-16-chars!")
	ctx := context.Background()
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 8080},
		Database: config.DatabaseConfig{Path: "/nonexistent/directory/flux.db"},
		CORS:     config.CORSConfig{Origin: "*"},
		Logging:  config.LoggingConfig{Level: "info"},
	}

	_, cleanup, err := setupServer(ctx, cfg)
	if err == nil {
		t.Fatal("expected error for invalid DB path, got nil")
	}
	if cleanup != nil {
		cleanup()
		t.Error("cleanup should be nil when setupServer fails")
	}
}

// readAll reads all bytes from the response body and returns them as a string.
func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	return string(data)
}
