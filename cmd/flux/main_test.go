package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	r := newRouter()
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/health", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", res.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if string(body) != "ok" {
		t.Errorf("got body %q, want %q", string(body), "ok")
	}
}

func TestProjectsEndpoint(t *testing.T) {
	r := newRouter()
	srv := httptest.NewServer(r)
	defer srv.Close()

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/projects", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/projects: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", res.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if string(body) != "[]" {
		t.Errorf("got body %q, want %q", string(body), "[]")
	}
}
