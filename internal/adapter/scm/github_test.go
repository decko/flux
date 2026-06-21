package scm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/decko/flux/internal/model"
)

// ---------------------------------------------------------------------------
// Test fixture types for GitHub REST API JSON responses
// ---------------------------------------------------------------------------

type ghUser struct {
	Login string `json:"login"`
}

type ghPR struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	HTMLURL   string  `json:"html_url"`
	State     string  `json:"state"`
	MergedAt  *string `json:"merged_at"`
	Body      string  `json:"body"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	User      ghUser  `json:"user"`
}

type ghReview struct {
	User        ghUser `json:"user"`
	State       string `json:"state"`
	Body        string `json:"body"`
	SubmittedAt string `json:"submitted_at"`
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func strPtr(s string) *string { return &s }

// newTestAdapter creates a GitHubSCMAdapter pointing at the given test server.
func newTestAdapter(t *testing.T, srv *httptest.Server) *GitHubSCMAdapter {
	t.Helper()
	return NewGitHubAdapter("test-owner", "test-repo", "test-token", srv.Client(), WithBaseURL(srv.URL))
}

// linkHeader builds a single Link header relation entry.
func linkHeader(rel, url string) string {
	return fmt.Sprintf(`<%s>; rel="%s"`, url, rel)
}

// writeJSON is a convenience for writing JSON responses in test handlers.
func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestNewGitHubAdapter
// ---------------------------------------------------------------------------

func TestNewGitHubAdapter(t *testing.T) {
	a := NewGitHubAdapter("owner", "repo", "token", nil)
	if got := a.Name(); got != "github" {
		t.Errorf("Name() = %q, want %q", got, "github")
	}
}

func TestNewGitHubAdapter_WithBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewGitHubAdapter("owner", "repo", "token", srv.Client(), WithBaseURL(srv.URL))
	if err := a.Health(context.Background()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestGitHubAdapter_ListPullRequests
// ---------------------------------------------------------------------------

func TestGitHubAdapter_ListPullRequests(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", r.Method)
			}
			if r.URL.Path != "/repos/test-owner/test-repo/pulls" {
				t.Errorf("path = %s, want /repos/test-owner/test-repo/pulls", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
			}
			writeJSON(t, w, http.StatusOK, []ghPR{
				{
					Number:    1,
					Title:     "Add authentication middleware",
					HTMLURL:   "https://github.com/test-owner/test-repo/pull/1",
					State:     "open",
					MergedAt:  nil,
					Body:      "Closes #42\nRefs #7",
					CreatedAt: "2024-01-15T10:00:00Z",
					UpdatedAt: "2024-01-16T12:00:00Z",
					User:      ghUser{Login: "alice"},
				},
			})
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		prs, err := adapter.ListPullRequests(context.Background(), "proj-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(prs) != 1 {
			t.Fatalf("got %d PRs, want 1", len(prs))
		}
		pr := prs[0]
		if pr.ExternalID != "1" {
			t.Errorf("ExternalID = %q, want %q", pr.ExternalID, "1")
		}
		if pr.Title != "Add authentication middleware" {
			t.Errorf("Title = %q, want %q", pr.Title, "Add authentication middleware")
		}
		if pr.URL != "https://github.com/test-owner/test-repo/pull/1" {
			t.Errorf("URL = %q, want %q", pr.URL, "https://github.com/test-owner/test-repo/pull/1")
		}
		if pr.Source != model.PRSourceGitHub {
			t.Errorf("Source = %q, want %q", pr.Source, model.PRSourceGitHub)
		}
		if pr.ProjectID != "proj-1" {
			t.Errorf("ProjectID = %q, want %q", pr.ProjectID, "proj-1")
		}
		if pr.Status != model.PRStatusOpen {
			t.Errorf("Status = %q, want %q", pr.Status, model.PRStatusOpen)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		var pageNum int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page := r.URL.Query().Get("page")
			pageNum++

			switch page {
			case "", "1":
				// Link header must be set before writeJSON (which calls WriteHeader).
				nextURL := fmt.Sprintf("http://%s/repos/test-owner/test-repo/pulls?page=2", r.Host)
				w.Header().Add("Link", linkHeader("next", nextURL))
				writeJSON(t, w, http.StatusOK, []ghPR{
					{
						Number:    1,
						Title:     "PR page 1",
						HTMLURL:   "https://github.com/test-owner/test-repo/pull/1",
						State:     "open",
						MergedAt:  nil,
						Body:      "",
						CreatedAt: "2024-01-15T10:00:00Z",
						UpdatedAt: "2024-01-16T12:00:00Z",
						User:      ghUser{Login: "alice"},
					},
				})
			case "2":
				writeJSON(t, w, http.StatusOK, []ghPR{
					{
						Number:    2,
						Title:     "PR page 2",
						HTMLURL:   "https://github.com/test-owner/test-repo/pull/2",
						State:     "open",
						MergedAt:  nil,
						Body:      "",
						CreatedAt: "2024-01-17T10:00:00Z",
						UpdatedAt: "2024-01-18T12:00:00Z",
						User:      ghUser{Login: "bob"},
					},
				})
				// No Link header = no more pages
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		prs, err := adapter.ListPullRequests(context.Background(), "proj-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(prs) != 2 {
			t.Fatalf("got %d PRs, want 2", len(prs))
		}
		if prs[0].ExternalID != "1" {
			t.Errorf("prs[0].ExternalID = %q, want %q", prs[0].ExternalID, "1")
		}
		if prs[1].ExternalID != "2" {
			t.Errorf("prs[1].ExternalID = %q, want %q", prs[1].ExternalID, "2")
		}
	})

	t.Run("empty result", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusOK, []ghPR{})
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		prs, err := adapter.ListPullRequests(context.Background(), "proj-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(prs) != 0 {
			t.Fatalf("got %d PRs, want 0", len(prs))
		}
	})

	t.Run("auth failure 401", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprintf(w, `{"message":"Bad credentials"}`)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		_, err := adapter.ListPullRequests(context.Background(), "proj-1")
		if err == nil {
			t.Fatal("expected error for 401, got nil")
		}
	})

	t.Run("rate limit 403", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprintf(w, `{"message":"API rate limit exceeded"}`)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		_, err := adapter.ListPullRequests(context.Background(), "proj-1")
		if err == nil {
			t.Fatal("expected error for 403, got nil")
		}
	})

	t.Run("rate limit 429", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprintf(w, `{"message":"Too many requests"}`)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		_, err := adapter.ListPullRequests(context.Background(), "proj-1")
		if err == nil {
			t.Fatal("expected error for 429, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGitHubAdapter_GetPullRequest
// ---------------------------------------------------------------------------

func TestGitHubAdapter_GetPullRequest(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", r.Method)
			}
			if r.URL.Path != "/repos/test-owner/test-repo/pulls/42" {
				t.Errorf("path = %s, want /repos/test-owner/test-repo/pulls/42", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
			}
			writeJSON(t, w, http.StatusOK, ghPR{
				Number:    42,
				Title:     "Fix login redirect",
				HTMLURL:   "https://github.com/test-owner/test-repo/pull/42",
				State:     "open",
				MergedAt:  nil,
				Body:      "Fixes #99",
				CreatedAt: "2024-02-01T08:00:00Z",
				UpdatedAt: "2024-02-02T09:00:00Z",
				User:      ghUser{Login: "charlie"},
			})
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		pr, err := adapter.GetPullRequest(context.Background(), "proj-2", "42")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr == nil {
			t.Fatal("expected non-nil PR")
		}
		if pr.ExternalID != "42" {
			t.Errorf("ExternalID = %q, want %q", pr.ExternalID, "42")
		}
		if pr.Title != "Fix login redirect" {
			t.Errorf("Title = %q, want %q", pr.Title, "Fix login redirect")
		}
		if pr.URL != "https://github.com/test-owner/test-repo/pull/42" {
			t.Errorf("URL = %q, want %q", pr.URL, "https://github.com/test-owner/test-repo/pull/42")
		}
		if pr.Source != model.PRSourceGitHub {
			t.Errorf("Source = %q, want %q", pr.Source, model.PRSourceGitHub)
		}
		if pr.ProjectID != "proj-2" {
			t.Errorf("ProjectID = %q, want %q", pr.ProjectID, "proj-2")
		}
		if pr.Status != model.PRStatusOpen {
			t.Errorf("Status = %q, want %q", pr.Status, model.PRStatusOpen)
		}
	})

	t.Run("not found 404", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintf(w, `{"message":"Not Found"}`)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		pr, err := adapter.GetPullRequest(context.Background(), "proj-2", "999")
		if err == nil {
			t.Fatal("expected error for 404, got nil")
		}
		if pr != nil {
			t.Errorf("expected nil PR on error, got %+v", pr)
		}
	})

	t.Run("merged PR", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusOK, ghPR{
				Number:    5,
				Title:     "Merged feature",
				HTMLURL:   "https://github.com/test-owner/test-repo/pull/5",
				State:     "closed",
				MergedAt:  strPtr("2024-03-01T12:00:00Z"),
				Body:      "Closes #101",
				CreatedAt: "2024-02-20T08:00:00Z",
				UpdatedAt: "2024-03-01T12:00:00Z",
				User:      ghUser{Login: "dave"},
			})
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		pr, err := adapter.GetPullRequest(context.Background(), "proj-2", "5")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr == nil {
			t.Fatal("expected non-nil PR")
		}
		if pr.Status != model.PRStatusMerged {
			t.Errorf("Status = %q, want %q", pr.Status, model.PRStatusMerged)
		}
	})

	t.Run("rate limit 429", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprintf(w, `{"message":"API rate limit exceeded"}`)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		_, err := adapter.GetPullRequest(context.Background(), "proj-2", "42")
		if err == nil {
			t.Fatal("expected error for rate limit, got nil")
		}
	})

	t.Run("invalid external ID", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		_, err := adapter.GetPullRequest(context.Background(), "proj-2", "abc")
		if err == nil {
			t.Fatal("expected error for non-numeric external ID, got nil")
		}
	})

	t.Run("closed PR (not merged)", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusOK, ghPR{
				Number:    6,
				Title:     "WIP abandoned",
				HTMLURL:   "https://github.com/test-owner/test-repo/pull/6",
				State:     "closed",
				MergedAt:  nil,
				Body:      "",
				CreatedAt: "2024-01-10T08:00:00Z",
				UpdatedAt: "2024-01-15T08:00:00Z",
				User:      ghUser{Login: "eve"},
			})
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		pr, err := adapter.GetPullRequest(context.Background(), "proj-2", "6")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr == nil {
			t.Fatal("expected non-nil PR")
		}
		if pr.Status != model.PRStatusClosed {
			t.Errorf("Status = %q, want %q", pr.Status, model.PRStatusClosed)
		}
	})
}

// ---------------------------------------------------------------------------
// TestGitHubAdapter_ListReviews
// ---------------------------------------------------------------------------

func TestGitHubAdapter_ListReviews(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", r.Method)
			}
			if r.URL.Path != "/repos/test-owner/test-repo/pulls/10/reviews" {
				t.Errorf("path = %s, want /repos/test-owner/test-repo/pulls/10/reviews", r.URL.Path)
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
				t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
			}
			writeJSON(t, w, http.StatusOK, []ghReview{
				{
					User:        ghUser{Login: "reviewer1"},
					State:       "APPROVED",
					Body:        "LGTM",
					SubmittedAt: "2024-01-16T12:00:00Z",
				},
				{
					User:        ghUser{Login: "reviewer2"},
					State:       "CHANGES_REQUESTED",
					Body:        "Please fix the edge case",
					SubmittedAt: "2024-01-16T13:00:00Z",
				},
			})
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		reviews, err := adapter.ListReviews(context.Background(), "proj-1", "10")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(reviews) != 2 {
			t.Fatalf("got %d reviews, want 2", len(reviews))
		}
		if reviews[0].Author != "reviewer1" {
			t.Errorf("reviews[0].Author = %q, want %q", reviews[0].Author, "reviewer1")
		}
		if reviews[0].Status != model.ReviewStatusApproved {
			t.Errorf("reviews[0].Status = %q, want %q", reviews[0].Status, model.ReviewStatusApproved)
		}
		if reviews[0].Comment != "LGTM" {
			t.Errorf("reviews[0].Comment = %q, want %q", reviews[0].Comment, "LGTM")
		}
		if reviews[1].Author != "reviewer2" {
			t.Errorf("reviews[1].Author = %q, want %q", reviews[1].Author, "reviewer2")
		}
		if reviews[1].Status != model.ReviewStatusChangesRequested {
			t.Errorf("reviews[1].Status = %q, want %q", reviews[1].Status, model.ReviewStatusChangesRequested)
		}
		if reviews[1].Comment != "Please fix the edge case" {
			t.Errorf("reviews[1].Comment = %q, want %q", reviews[1].Comment, "Please fix the edge case")
		}
	})

	t.Run("no reviews", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusOK, []ghReview{})
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		reviews, err := adapter.ListReviews(context.Background(), "proj-1", "11")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(reviews) != 0 {
			t.Fatalf("got %d reviews, want 0", len(reviews))
		}
	})

	t.Run("auth failure 401", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprintf(w, `{"message":"Bad credentials"}`)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		_, err := adapter.ListReviews(context.Background(), "proj-1", "10")
		if err == nil {
			t.Fatal("expected error for 401, got nil")
		}
	})

	t.Run("pagination", func(t *testing.T) {
		var pageNum int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pageNum++
			switch pageNum {
			case 1:
				nextURL := fmt.Sprintf("http://%s/repos/test-owner/test-repo/pulls/10/reviews?page=2", r.Host)
				w.Header().Add("Link", linkHeader("next", nextURL))
				writeJSON(t, w, http.StatusOK, []ghReview{
					{
						User:        ghUser{Login: "reviewer1"},
						State:       "APPROVED",
						Body:        "LGTM",
						SubmittedAt: "2024-01-16T12:00:00Z",
					},
				})
			case 2:
				writeJSON(t, w, http.StatusOK, []ghReview{
					{
						User:        ghUser{Login: "reviewer2"},
						State:       "COMMENTED",
						Body:        "Looks good",
						SubmittedAt: "2024-01-17T12:00:00Z",
					},
				})
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		reviews, err := adapter.ListReviews(context.Background(), "proj-1", "10")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(reviews) != 2 {
			t.Fatalf("got %d reviews, want 2", len(reviews))
		}
		if reviews[0].Author != "reviewer1" {
			t.Errorf("reviews[0].Author = %q, want %q", reviews[0].Author, "reviewer1")
		}
		if reviews[1].Author != "reviewer2" {
			t.Errorf("reviews[1].Author = %q, want %q", reviews[1].Author, "reviewer2")
		}
	})

	t.Run("unknown review states are skipped", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, http.StatusOK, []ghReview{
				{
					User:        ghUser{Login: "reviewer1"},
					State:       "PENDING",
					Body:        "pending review",
					SubmittedAt: "2024-01-16T12:00:00Z",
				},
				{
					User:        ghUser{Login: "reviewer2"},
					State:       "APPROVED",
					Body:        "LGTM",
					SubmittedAt: "2024-01-17T12:00:00Z",
				},
				{
					User:        ghUser{Login: "reviewer3"},
					State:       "DISMISSED",
					Body:        "dismissed",
					SubmittedAt: "2024-01-18T12:00:00Z",
				},
			})
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		reviews, err := adapter.ListReviews(context.Background(), "proj-1", "10")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(reviews) != 1 {
			t.Fatalf("got %d reviews, want 1 (only APPROVED should be included)", len(reviews))
		}
		if reviews[0].Author != "reviewer2" {
			t.Errorf("reviews[0].Author = %q, want %q", reviews[0].Author, "reviewer2")
		}
		if reviews[0].Status != model.ReviewStatusApproved {
			t.Errorf("reviews[0].Status = %q, want %q", reviews[0].Status, model.ReviewStatusApproved)
		}
	})

	t.Run("rate limit 429", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprintf(w, `{"message":"API rate limit exceeded"}`)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		_, err := adapter.ListReviews(context.Background(), "proj-1", "10")
		if err == nil {
			t.Fatal("expected error for rate limit, got nil")
		}
	})

	t.Run("invalid external ID", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		_, err := adapter.ListReviews(context.Background(), "proj-1", "abc")
		if err == nil {
			t.Fatal("expected error for non-numeric external ID, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGitHubAdapter_Health
// ---------------------------------------------------------------------------

func TestGitHubAdapter_Health(t *testing.T) {
	t.Parallel()

	t.Run("healthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"message":"OK"}`)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		if err := adapter.Health(context.Background()); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("unhealthy", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, `{"message":"Internal Server Error"}`)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		if err := adapter.Health(context.Background()); err == nil {
			t.Fatal("expected error for 500, got nil")
		}
	})

	t.Run("rate limit 429", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		adapter := newTestAdapter(t, srv)
		if err := adapter.Health(context.Background()); err == nil {
			t.Fatal("expected error for rate limit, got nil")
		}
	})

	t.Run("network error", func(t *testing.T) {
		// Point at a server that's not running to simulate connection refused.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		srv.Close() // close immediately so requests fail

		adapter := newTestAdapter(t, srv)
		if err := adapter.Health(context.Background()); err == nil {
			t.Fatal("expected error for closed server, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGitHubAdapter_PRStatusMapping — table-driven via GetPullRequest
// ---------------------------------------------------------------------------

func TestGitHubAdapter_PRStatusMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		state      string
		mergedAt   *string
		wantStatus model.PRStatus
	}{
		{
			name:       "open",
			state:      "open",
			mergedAt:   nil,
			wantStatus: model.PRStatusOpen,
		},
		{
			name:       "merged",
			state:      "closed",
			mergedAt:   strPtr("2024-04-01T00:00:00Z"),
			wantStatus: model.PRStatusMerged,
		},
		{
			name:       "closed without merge",
			state:      "closed",
			mergedAt:   nil,
			wantStatus: model.PRStatusClosed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, ghPR{
					Number:    1,
					Title:     "Test PR",
					HTMLURL:   "https://github.com/test-owner/test-repo/pull/1",
					State:     tt.state,
					MergedAt:  tt.mergedAt,
					Body:      "",
					CreatedAt: "2024-01-01T00:00:00Z",
					UpdatedAt: "2024-01-02T00:00:00Z",
					User:      ghUser{Login: "user"},
				})
			}))
			defer srv.Close()

			adapter := newTestAdapter(t, srv)
			pr, err := adapter.GetPullRequest(context.Background(), "proj-1", "1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pr == nil {
				t.Fatal("expected non-nil PR")
			}
			if pr.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", pr.Status, tt.wantStatus)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestGitHubAdapter_ReviewStateMapping — table-driven via ListReviews
// ---------------------------------------------------------------------------

func TestGitHubAdapter_ReviewStateMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		state      string
		wantStatus model.ReviewStatus
	}{
		{
			name:       "approved",
			state:      "APPROVED",
			wantStatus: model.ReviewStatusApproved,
		},
		{
			name:       "changes requested",
			state:      "CHANGES_REQUESTED",
			wantStatus: model.ReviewStatusChangesRequested,
		},
		{
			name:       "commented",
			state:      "COMMENTED",
			wantStatus: model.ReviewStatusCommented,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, []ghReview{
					{
						User:        ghUser{Login: "reviewer"},
						State:       tt.state,
						Body:        "review body",
						SubmittedAt: "2024-01-01T12:00:00Z",
					},
				})
			}))
			defer srv.Close()

			adapter := newTestAdapter(t, srv)
			reviews, err := adapter.ListReviews(context.Background(), "proj-1", "1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(reviews) != 1 {
				t.Fatalf("got %d reviews, want 1", len(reviews))
			}
			if reviews[0].Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", reviews[0].Status, tt.wantStatus)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestGitHubAdapter_TicketIDExtraction — verifies PR body parsing
// ---------------------------------------------------------------------------

func TestGitHubAdapter_TicketIDExtraction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantTicket []string
	}{
		{
			name:       "closes reference",
			body:       "Closes #42",
			wantTicket: []string{"42"},
		},
		{
			name:       "fixes reference",
			body:       "fixes #7",
			wantTicket: []string{"7"},
		},
		{
			name:       "refs reference",
			body:       "refs #123",
			wantTicket: []string{"123"},
		},
		{
			name:       "multiple references",
			body:       "Closes #42\nRefs #7\nFixes #99",
			wantTicket: []string{"42", "7", "99"},
		},
		{
			name:       "no reference",
			body:       "Just a regular PR description",
			wantTicket: []string{},
		},
		{
			name:       "empty body",
			body:       "",
			wantTicket: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(t, w, http.StatusOK, []ghPR{
					{
						Number:    1,
						Title:     "Test PR",
						HTMLURL:   "https://github.com/test-owner/test-repo/pull/1",
						State:     "open",
						MergedAt:  nil,
						Body:      tt.body,
						CreatedAt: "2024-01-01T00:00:00Z",
						UpdatedAt: "2024-01-02T00:00:00Z",
						User:      ghUser{Login: "user"},
					},
				})
			}))
			defer srv.Close()

			adapter := newTestAdapter(t, srv)
			prs, err := adapter.ListPullRequests(context.Background(), "proj-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(prs) != 1 {
				t.Fatalf("got %d PRs, want 1", len(prs))
			}
			if !stringSliceEqual(prs[0].TicketIDs, tt.wantTicket) {
				t.Errorf("TicketIDs = %v, want %v", prs[0].TicketIDs, tt.wantTicket)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestGitHubAdapter_TimeFields — verifies time parsing
// ---------------------------------------------------------------------------

func TestGitHubAdapter_TimeFields(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, []ghPR{
			{
				Number:    1,
				Title:     "Test PR",
				HTMLURL:   "https://github.com/test-owner/test-repo/pull/1",
				State:     "open",
				MergedAt:  nil,
				Body:      "",
				CreatedAt: "2024-06-15T14:30:00Z",
				UpdatedAt: "2024-06-16T10:00:00Z",
				User:      ghUser{Login: "user"},
			},
		})
	}))
	defer srv.Close()

	adapter := newTestAdapter(t, srv)
	prs, err := adapter.ListPullRequests(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("got %d PRs, want 1", len(prs))
	}
	wantCreated := mustParseTime("2024-06-15T14:30:00Z")
	wantUpdated := mustParseTime("2024-06-16T10:00:00Z")
	if !prs[0].CreatedAt.Equal(wantCreated) {
		t.Errorf("CreatedAt = %v, want %v", prs[0].CreatedAt, wantCreated)
	}
	if !prs[0].UpdatedAt.Equal(wantUpdated) {
		t.Errorf("UpdatedAt = %v, want %v", prs[0].UpdatedAt, wantUpdated)
	}
}

// ---------------------------------------------------------------------------
// TestGitHubAdapter_GetPullRequest_TicketExtraction — per single PR endpoint
// ---------------------------------------------------------------------------

func TestGitHubAdapter_GetPullRequest_TicketExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, ghPR{
			Number:    42,
			Title:     "Fix thing",
			HTMLURL:   "https://github.com/test-owner/test-repo/pull/42",
			State:     "open",
			MergedAt:  nil,
			Body:      "Closes #101\nRefs #202",
			CreatedAt: "2024-01-01T00:00:00Z",
			UpdatedAt: "2024-01-02T00:00:00Z",
			User:      ghUser{Login: "user"},
		})
	}))
	defer srv.Close()

	adapter := newTestAdapter(t, srv)
	pr, err := adapter.GetPullRequest(context.Background(), "proj-1", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr == nil {
		t.Fatal("expected non-nil PR")
	}

	want := []string{"101", "202"}
	if !stringSliceEqual(pr.TicketIDs, want) {
		t.Errorf("TicketIDs = %v, want %v", pr.TicketIDs, want)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// stringSliceEqual compares two string slices ignoring order.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// Build a map from slice b for O(n) lookup.
	m := make(map[string]int, len(b))
	for _, v := range b {
		m[v]++
	}
	for _, v := range a {
		m[v]--
		if m[v] < 0 {
			return false
		}
	}
	return true
}
