package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCORSMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		wantStatus int
	}{
		{name: "preflight", method: http.MethodOptions, wantStatus: http.StatusNoContent},
		{name: "get", method: http.MethodGet, wantStatus: http.StatusOK},
		{name: "post", method: http.MethodPost, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(CORSMiddleware("*"))
			r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			r.Post("/test", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			ts := httptest.NewServer(r)
			defer ts.Close()

			req, _ := http.NewRequestWithContext(context.Background(), tt.method, ts.URL+"/test", nil)
			req.Header.Set("Origin", "http://example.com")
			if tt.method == http.MethodOptions {
				req.Header.Set("Access-Control-Request-Method", "GET")
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s /test: %v", tt.method, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("got status %d, want %d", resp.StatusCode, tt.wantStatus)
			}
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
	}
}

func TestErrorHandlerMiddleware(t *testing.T) {
	r := chi.NewRouter()
	r.Use(ErrorHandlerMiddleware)
	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test error from middleware")
	})
	r.Get("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	t.Run("panic recovery returns 500 JSON", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/panic", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /panic: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusInternalServerError)
		}

		ct := resp.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("got Content-Type %q, want %q", ct, "application/json")
		}

		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode JSON body: %v", err)
		}
		if _, ok := body["error"]; !ok {
			t.Error("JSON response missing 'error' field")
		}
	})

	t.Run("normal request unaffected", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/ok", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /ok: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})
}
