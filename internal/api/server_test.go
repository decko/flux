package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/repository"
)

func TestNewServer(t *testing.T) {
	srv := NewServer()
	if srv == nil {
		t.Fatal("NewServer() returned nil")
	}
}

func TestNewServerWithOptions(t *testing.T) {
	var called bool
	opt := func(s *Server) {
		called = true
	}

	srv := NewServer(opt)
	if srv == nil {
		t.Fatal("NewServer() returned nil")
	}
	if !called {
		t.Error("ServerOption was not applied")
	}
}

func TestHealthEndpoint(t *testing.T) {
	srv := NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	got := strings.TrimSpace(readBody(t, resp))
	if got != "ok" {
		t.Errorf("got body %q, want %q", got, "ok")
	}
}

func TestAPIV1RoutesExist(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}

	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlite")
	repo := repository.NewSQLiteProjectRepository(sdb)
	svc := domain.NewProjectService(repo)
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithProjectService(svc))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		name string
		path string
	}{
		{name: "projects list", path: "/api/v1/projects"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := authedRequest(http.MethodGet, ts.URL+tt.path, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestCORSHeaders(t *testing.T) {
	srv := NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("preflight", func(t *testing.T) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodOptions, ts.URL+"/health", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "GET")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("OPTIONS /health: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("got Access-Control-Allow-Origin %q, want %q",
				resp.Header.Get("Access-Control-Allow-Origin"), "*")
		}
		if resp.Header.Get("Access-Control-Allow-Methods") == "" {
			t.Error("Access-Control-Allow-Methods header is empty")
		}
		if resp.Header.Get("Access-Control-Allow-Headers") == "" {
			t.Error("Access-Control-Allow-Headers header is empty")
		}
	})

	t.Run("actual request", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/health", nil)
		req.Header.Set("Origin", "http://example.com")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /health: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("got Access-Control-Allow-Origin %q, want %q",
				resp.Header.Get("Access-Control-Allow-Origin"), "*")
		}
	})
}

func TestWithCORSOrigin(t *testing.T) {
	srv := NewServer(WithCORSOrigin("http://localhost:3000"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	got := resp.Header.Get("Access-Control-Allow-Origin")
	if got != "http://localhost:3000" {
		t.Errorf("got Access-Control-Allow-Origin %q, want %q", got, "http://localhost:3000")
	}
}

func TestRequestIDHeader(t *testing.T) {
	srv := NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("response missing X-Request-Id header")
	}
}

func TestNotFoundReturnsJSON(t *testing.T) {
	srv := NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/nonexistent", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /nonexistent: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON body: %v", err)
	}

	if _, ok := body["error"]; !ok {
		t.Error("JSON response missing 'error' field")
	}
	if _, ok := body["request_id"]; !ok {
		t.Error("JSON response missing 'request_id' field")
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("got Content-Type %q, want %q", ct, "application/json")
	}
}

func TestMethodNotAllowedReturnsJSON(t *testing.T) {
	srv := NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// POST to a GET-only route
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, ts.URL+"/health", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON body: %v", err)
	}

	if _, ok := body["error"]; !ok {
		t.Error("JSON response missing 'error' field")
	}
	if _, ok := body["request_id"]; !ok {
		t.Error("JSON response missing 'request_id' field")
	}
}

func TestPanicRecoveryReturnsJSON(t *testing.T) {
	srv := NewServer()
	srv.router.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/panic", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /panic: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON body: %v", err)
	}

	if _, ok := body["error"]; !ok {
		t.Error("JSON response missing 'error' field")
	}
	if _, ok := body["request_id"]; !ok {
		t.Error("JSON response missing 'request_id' field")
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("got Content-Type %q, want %q", ct, "application/json")
	}
}

func TestJSONContentType(t *testing.T) {
	srv := NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("api endpoints return application/json", func(t *testing.T) {
		endpoints := []struct {
			name   string
			method string
			path   string
		}{
			{name: "projects", method: http.MethodGet, path: "/api/v1/projects"},
		}

		for _, ep := range endpoints {
			t.Run(ep.name, func(t *testing.T) {
				req, _ := http.NewRequestWithContext(context.Background(), ep.method, ts.URL+ep.path, nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("%s %s: %v", ep.method, ep.path, err)
				}
				defer func() { _ = resp.Body.Close() }()

				ct := resp.Header.Get("Content-Type")
				if ct != "application/json" {
					t.Errorf("%s: got Content-Type %q, want %q", ep.path, ct, "application/json")
				}
			})
		}
	})

	t.Run("all endpoints have Content-Type set", func(t *testing.T) {
		endpoints := []struct {
			name   string
			method string
			path   string
		}{
			{name: "health", method: http.MethodGet, path: "/health"},
			{name: "not found", method: http.MethodGet, path: "/nonexistent"},
		}

		for _, ep := range endpoints {
			t.Run(ep.name, func(t *testing.T) {
				req, _ := http.NewRequestWithContext(context.Background(), ep.method, ts.URL+ep.path, nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("%s %s: %v", ep.method, ep.path, err)
				}
				defer func() { _ = resp.Body.Close() }()

				if resp.Header.Get("Content-Type") == "" {
					t.Errorf("%s: Content-Type header is empty", ep.path)
				}
			})
		}
	})
}

func TestConcurrentRequests(t *testing.T) {
	srv := NewServer()
	ts := httptest.NewServer(srv)
	defer ts.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/health", nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				errs <- err
				return
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("unexpected status %d", resp.StatusCode)
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent request failed: %v", err)
		}
	}
}

// readBody reads the full response body and returns it as a string.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(data)
}
