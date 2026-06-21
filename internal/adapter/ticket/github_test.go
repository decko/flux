package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/decko/flux/internal/model"
)

// githubLabel maps to a GitHub API v3 label object.
type githubLabel struct {
	Name string `json:"name"`
}

// githubIssue maps to a GitHub API v3 Issue object. Only fields relevant
// to the adapter are included.
type githubIssue struct {
	ID          int64         `json:"id"`
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	Body        string        `json:"body"`
	State       string        `json:"state"`
	Labels      []githubLabel `json:"labels,omitempty"`
	PullRequest *struct{}     `json:"pull_request,omitempty"`
	HTMLURL     string        `json:"html_url"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// sendJSON is a helper for test handlers to write JSON responses.
func sendJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic(fmt.Sprintf("sendJSON encode: %v", err))
	}
}

// newGitHubAdapter creates a GitHubAdapter pointed at the test server.
// The adapter uses "test-owner", "test-repo", and "test-token".
func newGitHubAdapter(t *testing.T, handler http.Handler) *GitHubAdapter {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return NewGitHubAdapter("test-owner", "test-repo", "test-token", http.DefaultClient, WithBaseURL(ts.URL))
}

// assertAuthorization checks that the request has the expected Bearer token.
// If not, it writes a 401 and returns false.
func assertAuthorization(t *testing.T, w http.ResponseWriter, r *http.Request) bool {
	t.Helper()
	if r.Header.Get("Authorization") != "Bearer test-token" {
		sendJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestNewGitHubAdapter(t *testing.T) {
	t.Parallel()

	a := NewGitHubAdapter("my-owner", "my-repo", "my-token", http.DefaultClient)
	if got := a.Name(); got != "github" {
		t.Errorf("Name() = %q, want %q", got, "github")
	}
}

func TestGitHubAdapter_ListTickets(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC)
	issues := []githubIssue{
		{
			ID: 1, Number: 42, Title: "Fix bug", Body: "Crash on startup",
			State: "open", Labels: []githubLabel{{Name: "bug"}},
			HTMLURL:   "https://github.com/test-owner/test-repo/issues/42",
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: 2, Number: 43, Title: "Add feature", Body: "New login page",
			State: "closed", Labels: []githubLabel{{Name: "enhancement"}},
			HTMLURL:   "https://github.com/test-owner/test-repo/issues/43",
			CreatedAt: now, UpdatedAt: now,
		},
	}

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantLen int
		wantErr bool
		check   func(t *testing.T, tickets []model.Ticket)
	}{
		{
			name: "happy path returns ticket list",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				if r.URL.RawQuery != "state=all" {
					t.Errorf("RawQuery = %q, want %q", r.URL.RawQuery, "state=all")
				}
				sendJSON(w, http.StatusOK, issues)
			},
			wantLen: 2,
			wantErr: false,
			check: func(t *testing.T, tickets []model.Ticket) {
				if len(tickets) < 2 {
					return
				}
				// First ticket
				if tickets[0].ExternalID != "42" {
					t.Errorf("tickets[0].ExternalID = %q, want %q", tickets[0].ExternalID, "42")
				}
				if tickets[0].Title != "Fix bug" {
					t.Errorf("tickets[0].Title = %q, want %q", tickets[0].Title, "Fix bug")
				}
				if tickets[0].Description != "Crash on startup" {
					t.Errorf("tickets[0].Description = %q, want %q", tickets[0].Description, "Crash on startup")
				}
				if tickets[0].Status != model.TicketStatusOpen {
					t.Errorf("tickets[0].Status = %q, want %q", tickets[0].Status, model.TicketStatusOpen)
				}
				if tickets[0].Source != model.TicketSourceGitHub {
					t.Errorf("tickets[0].Source = %q, want %q", tickets[0].Source, model.TicketSourceGitHub)
				}
				if len(tickets[0].Labels) != 1 || tickets[0].Labels[0] != "bug" {
					t.Errorf("tickets[0].Labels = %v, want [bug]", tickets[0].Labels)
				}
				// Second ticket
				if tickets[1].ExternalID != "43" {
					t.Errorf("tickets[1].ExternalID = %q, want %q", tickets[1].ExternalID, "43")
				}
				if tickets[1].Title != "Add feature" {
					t.Errorf("tickets[1].Title = %q, want %q", tickets[1].Title, "Add feature")
				}
				if tickets[1].Status != model.TicketStatusClosed {
					t.Errorf("tickets[1].Status = %q, want %q", tickets[1].Status, model.TicketStatusClosed)
				}
			},
		},
		{
			name: "empty result returns empty slice",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				sendJSON(w, http.StatusOK, []githubIssue{})
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "pull requests are filtered out",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				sendJSON(w, http.StatusOK, []githubIssue{
					{
						ID: 1, Number: 42, Title: "Real issue",
						Body: "This is an issue", State: "open",
					},
					{
						ID: 2, Number: 43, Title: "PR #43",
						Body: "This is a PR", State: "open",
						PullRequest: &struct{}{},
					},
				})
			},
			wantLen: 1,
			wantErr: false,
			check: func(t *testing.T, tickets []model.Ticket) {
				if len(tickets) != 1 {
					return
				}
				if tickets[0].ExternalID != "42" {
					t.Errorf("ExternalID = %q, want %q", tickets[0].ExternalID, "42")
				}
			},
		},
		{
			name: "auth failure returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				sendJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
			},
			wantLen: 0,
			wantErr: true,
		},
		{
			name: "rate limit exceeded returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-RateLimit-Remaining", "0")
				sendJSON(w, http.StatusForbidden, map[string]string{"message": "API rate limit exceeded"})
			},
			wantLen: 0,
			wantErr: true,
		},
		{
			name: "server error returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				sendJSON(w, http.StatusInternalServerError, map[string]string{"message": "Internal Server Error"})
			},
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := newGitHubAdapter(t, tt.handler)
			tickets, err := a.ListTickets(context.Background(), "test-project")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(tickets) != tt.wantLen {
				t.Errorf("got %d tickets, want %d", len(tickets), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, tickets)
			}
		})
	}
}

func TestGitHubAdapter_GetTicket(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		externalID string
		wantTitle  string
		wantErr    bool
	}{
		{
			name: "happy path returns single ticket",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				sendJSON(w, http.StatusOK, githubIssue{
					ID: 1, Number: 42, Title: "Fix bug",
					Body: "Crash on startup", State: "open",
					HTMLURL:   "https://github.com/test-owner/test-repo/issues/42",
					CreatedAt: now, UpdatedAt: now,
				})
			},
			externalID: "42",
			wantTitle:  "Fix bug",
			wantErr:    false,
		},
		{
			name: "not found returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				sendJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
			},
			externalID: "99999",
			wantErr:    true,
		},
		{
			name: "auth failure returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				sendJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
			},
			externalID: "42",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := newGitHubAdapter(t, tt.handler)
			ticket, err := a.GetTicket(context.Background(), "test-project", tt.externalID)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ticket == nil {
				t.Fatal("expected non-nil ticket, got nil")
			}
			if ticket.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", ticket.Title, tt.wantTitle)
			}
			if ticket.Source != model.TicketSourceGitHub {
				t.Errorf("Source = %q, want %q", ticket.Source, model.TicketSourceGitHub)
			}
			if ticket.ExternalID != tt.externalID {
				t.Errorf("ExternalID = %q, want %q", ticket.ExternalID, tt.externalID)
			}
		})
	}
}

func TestGitHubAdapter_CreateTicket(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 6, 21, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		handler http.HandlerFunc
		input   *model.Ticket
		wantErr bool
	}{
		{
			name: "happy path creates ticket",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				if r.Method != http.MethodPost {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				// Verify Content-Type
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type = %q, want %q", ct, "application/json")
				}
				// Decode and verify the request body
				body, _ := io.ReadAll(r.Body)
				var req struct {
					Title  string   `json:"title"`
					Body   string   `json:"body"`
					Labels []string `json:"labels"`
				}
				if err := json.Unmarshal(body, &req); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}
				if req.Title != "New ticket" {
					t.Errorf("request title = %q, want %q", req.Title, "New ticket")
				}

				sendJSON(w, http.StatusCreated, githubIssue{
					ID: 100, Number: 1, Title: req.Title,
					Body: req.Body, State: "open",
					Labels:    []githubLabel{{Name: "bug"}},
					HTMLURL:   "https://github.com/test-owner/test-repo/issues/1",
					CreatedAt: now, UpdatedAt: now,
				})
			},
			input: &model.Ticket{
				Title:       "New ticket",
				Description: "Description of the new ticket",
				Labels:      []string{"bug"},
			},
			wantErr: false,
		},
		{
			name: "validation error returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				sendJSON(w, http.StatusUnprocessableEntity, map[string]string{"message": "Validation Failed"})
			},
			input:   &model.Ticket{Title: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := newGitHubAdapter(t, tt.handler)
			created, err := a.CreateTicket(context.Background(), tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if created == nil {
				t.Fatal("expected non-nil created ticket, got nil")
			}
			if created.Title != tt.input.Title {
				t.Errorf("Title = %q, want %q", created.Title, tt.input.Title)
			}
			if created.Source != model.TicketSourceGitHub {
				t.Errorf("Source = %q, want %q", created.Source, model.TicketSourceGitHub)
			}
			if created.ExternalID == "" {
				t.Error("ExternalID should be set by server, got empty")
			}
		})
	}
}

func TestGitHubAdapter_UpdateTicket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		input   *model.Ticket
		wantErr bool
	}{
		{
			name: "happy path updates ticket",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				if r.Method != http.MethodPatch {
					t.Errorf("expected PATCH, got %s", r.Method)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				if !strings.Contains(r.URL.Path, "/42") {
					t.Errorf("expected path to contain issue number 42, got %s", r.URL.Path)
				}
				if ua := r.Header.Get("User-Agent"); ua != "flux/0.1" {
					t.Errorf("User-Agent = %q, want %q", ua, "flux/0.1")
				}
				// Verify Content-Type
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type = %q, want %q", ct, "application/json")
				}
				// Verify request body includes status mapping, title, and body
				body, _ := io.ReadAll(r.Body)
				var req struct {
					Title string `json:"title"`
					Body  string `json:"body"`
					State string `json:"state"`
				}
				if err := json.Unmarshal(body, &req); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}
				if req.State != "closed" {
					t.Errorf("request state = %q, want %q", req.State, "closed")
				}
				if req.Title != "Updated title" {
					t.Errorf("request title = %q, want %q", req.Title, "Updated title")
				}
				if req.Body != "New description" {
					t.Errorf("request body = %q, want %q", req.Body, "New description")
				}
				sendJSON(w, http.StatusOK, githubIssue{
					ID: 1, Number: 42, Title: req.Title,
					State: req.State,
				})
			},
			input: &model.Ticket{
				ExternalID:  "42",
				Title:       "Updated title",
				Description: "New description",
				Status:      model.TicketStatusClosed,
			},
			wantErr: false,
		},
		{
			name: "clears labels when empty slice is passed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				if r.Method != http.MethodPatch {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				body, _ := io.ReadAll(r.Body)
				var req struct {
					Labels []string `json:"labels"`
				}
				if err := json.Unmarshal(body, &req); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}
				if len(req.Labels) != 0 {
					t.Errorf("request labels = %v, want empty slice", req.Labels)
				}
				sendJSON(w, http.StatusOK, githubIssue{})
			},
			input: &model.Ticket{
				ExternalID: "42",
				Labels:     []string{},
				Status:     model.TicketStatusOpen,
			},
			wantErr: false,
		},
		{
			name: "not found returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				sendJSON(w, http.StatusNotFound, map[string]string{"message": "Not Found"})
			},
			input: &model.Ticket{
				ExternalID: "99999",
				Title:      "Ghost ticket",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := newGitHubAdapter(t, tt.handler)
			err := a.UpdateTicket(context.Background(), tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGitHubAdapter_Health(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "healthy returns no error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				sendJSON(w, http.StatusOK, map[string]any{"id": 1})
			},
			wantErr: false,
		},
		{
			name: "non-200 status returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if !assertAuthorization(t, w, r) {
					return
				}
				sendJSON(w, http.StatusInternalServerError, map[string]string{"message": "Server Error"})
			},
			wantErr: true,
		},
		{
			name: "auth failure returns error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				sendJSON(w, http.StatusUnauthorized, map[string]string{"message": "Bad credentials"})
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := newGitHubAdapter(t, tt.handler)
			err := a.Health(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGitHubAdapter_ListTickets_Pagination(t *testing.T) {
	t.Parallel()

	var serverURL string
	pageCalls := 0

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assertAuthorization(t, w, r) {
			return
		}
		pageCalls++

		switch pageCalls {
		case 1:
			// First page: must include state=all query param.
			if r.URL.RawQuery != "state=all" {
				t.Errorf("first page RawQuery = %q, want %q", r.URL.RawQuery, "state=all")
			}
			// Return 1 issue and a Link header pointing to page 2.
			nextURL := fmt.Sprintf(`<%s/repos/test-owner/test-repo/issues?page=2>; rel="next"`, serverURL)
			w.Header().Set("Link", nextURL)
			sendJSON(w, http.StatusOK, []githubIssue{
				{
					ID: 1, Number: 1, Title: "First issue",
					Body: "First page", State: "open",
				},
			})
		case 2:
			// Second page returns 2 issues and no Link header (last page).
			if r.URL.RawQuery != "page=2" {
				t.Errorf("second page RawQuery = %q, want %q", r.URL.RawQuery, "page=2")
			}
			sendJSON(w, http.StatusOK, []githubIssue{
				{
					ID: 2, Number: 2, Title: "Second issue",
					Body: "Second page", State: "open",
				},
				{
					ID: 3, Number: 3, Title: "Third issue",
					Body: "Second page", State: "closed",
				},
			})
		default:
			t.Errorf("unexpected page request: page %d, URL %s", pageCalls, r.URL)
			w.WriteHeader(http.StatusTeapot)
		}
	})

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	serverURL = ts.URL

	a := NewGitHubAdapter("test-owner", "test-repo", "test-token", http.DefaultClient, WithBaseURL(ts.URL))
	tickets, err := a.ListTickets(context.Background(), "test-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tickets) != 3 {
		t.Fatalf("got %d tickets, want 3 (merged across 2 pages)", len(tickets))
	}
	// Verify ordering: first page results come before second page.
	if tickets[0].Title != "First issue" {
		t.Errorf("tickets[0].Title = %q, want %q", tickets[0].Title, "First issue")
	}
	if tickets[1].Title != "Second issue" {
		t.Errorf("tickets[1].Title = %q, want %q", tickets[1].Title, "Second issue")
	}
	if tickets[2].Title != "Third issue" {
		t.Errorf("tickets[2].Title = %q, want %q", tickets[2].Title, "Third issue")
	}
	if pageCalls != 2 {
		t.Errorf("expected 2 page requests, got %d", pageCalls)
	}
}
