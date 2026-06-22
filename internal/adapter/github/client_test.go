package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TestClient_DoRequest_AddsAuthHeaders
// ---------------------------------------------------------------------------

func TestClient_DoRequest_AddsAuthHeaders(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github.v3+json" {
			t.Errorf("Accept = %q, want %q", got, "application/vnd.github.v3+json")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient("test-token", srv.Client())
	resp, err := client.DoRequest(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp.Body.Close()
}

// ---------------------------------------------------------------------------
// TestClient_DoRequest_RateLimitDetected
// ---------------------------------------------------------------------------

func TestClient_DoRequest_RateLimitDetected(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewClient("test-token", srv.Client())
	_, err := client.DoRequest(context.Background(), http.MethodGet, srv.URL, nil) //nolint:bodyclose // body is closed by DoRequest on error
	if err == nil {
		t.Fatal("expected error for rate limit, got nil")
	}
	if err != ErrRateLimited {
		t.Errorf("err = %v, want %v", err, ErrRateLimited)
	}
}

// ---------------------------------------------------------------------------
// TestClient_DoRequest_ErrorWrapping
// ---------------------------------------------------------------------------

func TestClient_DoRequest_ErrorWrapping(t *testing.T) {
	t.Parallel()

	// Start and immediately close the server to force a transport error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	client := NewClient("test-token", srv.Client())
	_, err := client.DoRequest(context.Background(), http.MethodGet, srv.URL, nil) //nolint:bodyclose // transport error, no response body
	if err == nil {
		t.Fatal("expected error for closed server, got nil")
	}
	if !strings.Contains(err.Error(), "execute request") {
		t.Errorf("error %q does not contain %q", err.Error(), "execute request")
	}
}

// ---------------------------------------------------------------------------
// TestGetNextPageURL — table-driven
// ---------------------------------------------------------------------------

func TestGetNextPageURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "standard next link",
			header: `<https://api.github.com/repos/owner/repo/pulls?page=2>; rel="next"`,
			want:   "https://api.github.com/repos/owner/repo/pulls?page=2",
		},
		{
			name:   "no next link",
			header: `<https://api.github.com/repos/owner/repo/pulls?page=2>; rel="last"`,
			want:   "",
		},
		{
			name:   "multiple links with next",
			header: `<https://api.github.com/repos/owner/repo/pulls?page=1>; rel="first", <https://api.github.com/repos/owner/repo/pulls?page=2>; rel="next", <https://api.github.com/repos/owner/repo/pulls?page=5>; rel="last"`,
			want:   "https://api.github.com/repos/owner/repo/pulls?page=2",
		},
		{
			name:   "malformed header missing angle brackets",
			header: `https://api.github.com/repos/owner/repo/pulls?page=2; rel="next"`,
			want:   "",
		},
		{
			name:   "empty header",
			header: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{Header: make(http.Header)}
			resp.Header.Set("Link", tt.header)
			got := GetNextPageURL(resp)
			if got != tt.want {
				t.Errorf("GetNextPageURL = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestClient_DoRequest_Success
// ---------------------------------------------------------------------------

func TestClient_DoRequest_Success(t *testing.T) {
	t.Parallel()

	type payload struct {
		Message string `json:"message"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"message":"ok"}`)
	}))
	defer srv.Close()

	client := NewClient("test-token", srv.Client())
	resp, err := client.DoRequest(context.Background(), http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if p.Message != "ok" {
		t.Errorf("Message = %q, want %q", p.Message, "ok")
	}
}
