package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// setupPRServer creates an in-memory SQLite database, migrates it,
// creates a PullRequestService-backed Server, and returns the server along with
// a seed function for populating pull requests into the same database.
func setupPRServer(t *testing.T) (*Server, func(t *testing.T, pr model.PullRequest) model.PullRequest) {
	t.Helper()

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
	repo := repository.NewSQLitePullRequestRepository(db)

	svc := domain.NewPullRequestService(repo)
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithPRService(svc))

	seed := func(t *testing.T, pr model.PullRequest) model.PullRequest {
		t.Helper()
		if pr.ID == "" {
			pr.ID = uuid.NewString()
		}
		now := time.Now().UTC().Truncate(time.Second)
		pr.CreatedAt = now
		pr.UpdatedAt = now
		if err := svc.Create(context.Background(), pr); err != nil {
			t.Fatalf("failed to seed pull request: %v", err)
		}
		return pr
	}

	return srv, seed
}

// ─── List ─────────────────────────────────────────────────────────────────

func TestListPRs(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		srv, _ := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/pull-requests", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pull-requests: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body prPage
		mustDecode(t, resp, &body)
		if body.Items == nil {
			t.Fatal("expected non-nil items array, got nil")
		}
		if len(body.Items) != 0 {
			t.Errorf("got %d items, want 0", len(body.Items))
		}
	})

	t.Run("with items", func(t *testing.T) {
		srv, seed := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.PullRequest{
			Title: "PR 1", ProjectID: "proj-1",
			Source: model.PRSourceGitHub, Status: model.PRStatusOpen,
			URL: "https://github.com/example/repo/pull/1",
		})
		seed(t, model.PullRequest{
			Title: "PR 2", ProjectID: "proj-1",
			Source: model.PRSourceGitLab, Status: model.PRStatusMerged,
			URL: "https://gitlab.com/example/repo/merge/2",
		})

		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/pull-requests", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pull-requests: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body prPage
		mustDecode(t, resp, &body)
		if len(body.Items) != 2 {
			t.Errorf("got %d items, want 2", len(body.Items))
		}
	})

	t.Run("filter by project_id", func(t *testing.T) {
		srv, seed := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.PullRequest{
			Title: "P1 PR", ProjectID: "proj-1",
			Source: model.PRSourceGitHub, Status: model.PRStatusOpen,
			URL: "https://github.com/example/repo/pull/1",
		})
		seed(t, model.PullRequest{
			Title: "P2 PR", ProjectID: "proj-2",
			Source: model.PRSourceGitLab, Status: model.PRStatusOpen,
			URL: "https://gitlab.com/example/repo/merge/2",
		})

		u := ts.URL + "/api/v1/pull-requests?project_id=proj-1"
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pull-requests?project_id=proj-1: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body prPage
		mustDecode(t, resp, &body)
		if len(body.Items) != 1 {
			t.Fatalf("got %d items, want 1", len(body.Items))
		}
		if body.Items[0].ProjectID != "proj-1" {
			t.Errorf("got project_id %q, want %q", body.Items[0].ProjectID, "proj-1")
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		srv, seed := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.PullRequest{
			Title: "Open PR", ProjectID: "proj-1",
			Source: model.PRSourceGitHub, Status: model.PRStatusOpen,
			URL: "https://github.com/example/repo/pull/1",
		})
		seed(t, model.PullRequest{
			Title: "Merged PR", ProjectID: "proj-1",
			Source: model.PRSourceGitLab, Status: model.PRStatusMerged,
			URL: "https://gitlab.com/example/repo/merge/2",
		})

		u := ts.URL + "/api/v1/pull-requests?status=open"
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pull-requests?status=open: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body prPage
		mustDecode(t, resp, &body)
		if len(body.Items) != 1 {
			t.Fatalf("got %d items, want 1", len(body.Items))
		}
		if body.Items[0].Status != model.PRStatusOpen {
			t.Errorf("got status %q, want %q", body.Items[0].Status, model.PRStatusOpen)
		}
	})
}

// ─── Get ──────────────────────────────────────────────────────────────────

func TestGetPR(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		srv, seed := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.PullRequest{
			Title: "Test PR", ProjectID: "proj-1",
			Source: model.PRSourceGitHub, Status: model.PRStatusOpen,
			URL: "https://github.com/example/repo/pull/42",
		})

		u := fmt.Sprintf("%s/api/v1/pull-requests/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pull-requests/%s: %v", orig.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var got model.PullRequest
		mustDecode(t, resp, &got)
		if got.ID != orig.ID {
			t.Errorf("got ID %q, want %q", got.ID, orig.ID)
		}
		if got.Title != orig.Title {
			t.Errorf("got title %q, want %q", got.Title, orig.Title)
		}
		if got.Status != orig.Status {
			t.Errorf("got status %q, want %q", got.Status, orig.Status)
		}
	})

	t.Run("not found", func(t *testing.T) {
		srv, _ := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		id := uuid.NewString()
		u := fmt.Sprintf("%s/api/v1/pull-requests/%s", ts.URL, id)
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pull-requests/%s: %v", id, err)
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
	})
}

// ─── Update ───────────────────────────────────────────────────────────────

func TestUpdatePR(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		srv, seed := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.PullRequest{
			Title: "Update Me", ProjectID: "proj-1",
			Source: model.PRSourceGitHub, Status: model.PRStatusOpen,
			URL: "https://github.com/example/repo/pull/99",
		})

		body := fmt.Sprintf(`{"id":%q,"title":"Updated","project_id":"proj-1","source":"github","status":"merged","url":"https://github.com/example/repo/pull/99"}`, orig.ID)
		u := fmt.Sprintf("%s/api/v1/pull-requests/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/pull-requests/%s: %v", orig.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var updated model.PullRequest
		mustDecode(t, resp, &updated)
		if updated.Title != "Updated" {
			t.Errorf("got title %q, want %q", updated.Title, "Updated")
		}
		if updated.Status != model.PRStatusMerged {
			t.Errorf("got status %q, want %q", updated.Status, model.PRStatusMerged)
		}
	})

	t.Run("not found", func(t *testing.T) {
		srv, _ := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		id := uuid.NewString()
		body := fmt.Sprintf(`{"id":%q,"title":"Ghost","project_id":"proj-1","source":"github","status":"open","url":"https://github.com/example/repo/pull/404"}`, id)
		u := fmt.Sprintf("%s/api/v1/pull-requests/%s", ts.URL, id)
		req := authedRequest(http.MethodPut, u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/pull-requests/%s: %v", id, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("invalid body - missing title", func(t *testing.T) {
		srv, seed := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.PullRequest{
			Title: "Validate Me", ProjectID: "proj-1",
			Source: model.PRSourceGitHub, Status: model.PRStatusOpen,
			URL: "https://github.com/example/repo/pull/1",
		})

		body := fmt.Sprintf(`{"id":%q,"project_id":"proj-1","source":"github","status":"open","url":"https://github.com/example/repo/pull/1"}`, orig.ID)
		u := fmt.Sprintf("%s/api/v1/pull-requests/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/pull-requests/%s: %v", orig.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		var errResp map[string]string
		mustDecode(t, resp, &errResp)
		if _, ok := errResp["error"]; !ok {
			t.Error("JSON response missing 'error' field")
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		srv, seed := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.PullRequest{
			Title: "Bad Status", ProjectID: "proj-1",
			Source: model.PRSourceGitHub, Status: model.PRStatusOpen,
			URL: "https://github.com/example/repo/pull/1",
		})

		body := fmt.Sprintf(`{"id":%q,"title":"Bad","project_id":"proj-1","source":"github","status":"bogus","url":"https://github.com/example/repo/pull/1"}`, orig.ID)
		u := fmt.Sprintf("%s/api/v1/pull-requests/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/pull-requests/%s: %v", orig.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("id mismatch", func(t *testing.T) {
		srv, seed := setupPRServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.PullRequest{
			Title: "Mismatch Me", ProjectID: "proj-1",
			Source: model.PRSourceGitHub, Status: model.PRStatusOpen,
			URL: "https://github.com/example/repo/pull/1",
		})

		otherID := uuid.NewString()
		body := fmt.Sprintf(`{"id":%q,"title":"Mismatched","project_id":"proj-1","source":"github","status":"open","url":"https://github.com/example/repo/pull/1"}`, otherID)
		u := fmt.Sprintf("%s/api/v1/pull-requests/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/pull-requests/%s: %v", orig.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		var errResp map[string]string
		mustDecode(t, resp, &errResp)
		if _, ok := errResp["error"]; !ok {
			t.Error("JSON response missing 'error' field")
		}
	})
}

// ─── Method Not Allowed ───────────────────────────────────────────────────

func TestPRMethodNotAllowed(t *testing.T) {
	srv, _ := setupPRServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "post to PRs list", method: http.MethodPost, path: "/api/v1/pull-requests"},
		{name: "delete to PR detail", method: http.MethodDelete, path: "/api/v1/pull-requests/some-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := authedRequest(tt.method, ts.URL+tt.path, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", tt.method, tt.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
			}

			var errResp map[string]string
			mustDecode(t, resp, &errResp)
			if _, ok := errResp["error"]; !ok {
				t.Error("JSON response missing 'error' field")
			}
		})
	}
}
