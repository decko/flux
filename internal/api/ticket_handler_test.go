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
	_ "github.com/mattn/go-sqlite3"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// setupTicketServer creates an in-memory SQLite database, migrates it,
// creates a TicketService-backed Server, and returns the server along with
// a seed function for populating tickets into the same database.
func setupTicketServer(t *testing.T) (*Server, func(t *testing.T, tkt model.Ticket) model.Ticket) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}

	repo := repository.NewSQLiteTicketRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to run migration: %v", err)
	}

	svc := domain.NewTicketService(repo)
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithTicketService(svc))

	seed := func(t *testing.T, tkt model.Ticket) model.Ticket {
		t.Helper()
		if tkt.ID == "" {
			tkt.ID = uuid.NewString()
		}
		now := time.Now().UTC().Truncate(time.Second)
		tkt.CreatedAt = now
		tkt.UpdatedAt = now
		if err := svc.Create(context.Background(), tkt); err != nil {
			t.Fatalf("failed to seed ticket: %v", err)
		}
		return tkt
	}

	return srv, seed
}

// ─── List ─────────────────────────────────────────────────────────────────

func TestListTickets(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		srv, _ := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/tickets", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/tickets: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var page ticketPage
		mustDecode(t, resp, &page)
		if page.Items == nil {
			t.Fatal("expected non-nil items array, got nil")
		}
		if len(page.Items) != 0 {
			t.Errorf("got %d items, want 0", len(page.Items))
		}
		if page.Page != 1 {
			t.Errorf("got page %d, want 1", page.Page)
		}
		if page.Limit != 20 {
			t.Errorf("got limit %d, want 20", page.Limit)
		}
		if page.Total != 0 {
			t.Errorf("got total %d, want 0", page.Total)
		}
	})

	t.Run("with items", func(t *testing.T) {
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.Ticket{
			Title: "Ticket 1", ProjectID: "proj-1",
			Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
		})
		seed(t, model.Ticket{
			Title: "Ticket 2", ProjectID: "proj-1",
			Source: model.TicketSourceJira, Status: model.TicketStatusClosed,
		})

		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/tickets", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/tickets: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var page ticketPage
		mustDecode(t, resp, &page)
		if len(page.Items) != 2 {
			t.Fatalf("got %d items, want 2", len(page.Items))
		}
		if page.Total != 2 {
			t.Errorf("got total %d, want 2", page.Total)
		}
	})

	t.Run("filter by project_id", func(t *testing.T) {
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.Ticket{
			Title: "P1 Ticket", ProjectID: "proj-1",
			Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
		})
		seed(t, model.Ticket{
			Title: "P2 Ticket", ProjectID: "proj-2",
			Source: model.TicketSourceJira, Status: model.TicketStatusOpen,
		})

		u := ts.URL + "/api/v1/tickets?project_id=proj-1"
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/tickets?project_id=proj-1: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var page ticketPage
		mustDecode(t, resp, &page)
		if len(page.Items) != 1 {
			t.Fatalf("got %d items, want 1", len(page.Items))
		}
		if page.Items[0].ProjectID != "proj-1" {
			t.Errorf("got project_id %q, want %q", page.Items[0].ProjectID, "proj-1")
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.Ticket{
			Title: "Open Ticket", ProjectID: "proj-1",
			Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
		})
		seed(t, model.Ticket{
			Title: "Closed Ticket", ProjectID: "proj-1",
			Source: model.TicketSourceJira, Status: model.TicketStatusClosed,
		})

		u := ts.URL + "/api/v1/tickets?status=open"
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/tickets?status=open: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var page ticketPage
		mustDecode(t, resp, &page)
		if len(page.Items) != 1 {
			t.Fatalf("got %d items, want 1", len(page.Items))
		}
		if page.Items[0].Status != model.TicketStatusOpen {
			t.Errorf("got status %q, want %q", page.Items[0].Status, model.TicketStatusOpen)
		}
	})

	t.Run("filter by labels", func(t *testing.T) {
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.Ticket{
			Title: "Bug Ticket", ProjectID: "proj-1",
			Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
			Labels: []string{"bug", "critical"},
		})
		seed(t, model.Ticket{
			Title: "Feature Ticket", ProjectID: "proj-1",
			Source: model.TicketSourceJira, Status: model.TicketStatusOpen,
			Labels: []string{"feature"},
		})
		seed(t, model.Ticket{
			Title: "Chore Ticket", ProjectID: "proj-1",
			Source: model.TicketSourceLinear, Status: model.TicketStatusClosed,
			Labels: []string{"chore"},
		})

		u := ts.URL + "/api/v1/tickets?labels=bug,feature"
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/tickets?labels=bug,feature: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var page ticketPage
		mustDecode(t, resp, &page)
		if len(page.Items) != 2 {
			t.Fatalf("got %d items, want 2", len(page.Items))
		}
	})

	t.Run("pagination default", func(t *testing.T) {
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		for i := 0; i < 25; i++ {
			seed(t, model.Ticket{
				Title: fmt.Sprintf("Ticket %d", i), ProjectID: "proj-1",
				Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
			})
		}

		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/tickets", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/tickets: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var page ticketPage
		mustDecode(t, resp, &page)
		if page.Page != 1 {
			t.Errorf("got page %d, want 1", page.Page)
		}
		if page.Limit != 20 {
			t.Errorf("got limit %d, want 20", page.Limit)
		}
		if len(page.Items) != 20 {
			t.Errorf("got %d items, want 20", len(page.Items))
		}
		if page.Total != 25 {
			t.Errorf("got total %d, want 25", page.Total)
		}
	})

	t.Run("pagination second page", func(t *testing.T) {
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		for i := 0; i < 25; i++ {
			seed(t, model.Ticket{
				Title: fmt.Sprintf("Ticket %d", i), ProjectID: "proj-1",
				Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
			})
		}

		u := ts.URL + "/api/v1/tickets?page=2&limit=10"
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/tickets?page=2&limit=10: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var page ticketPage
		mustDecode(t, resp, &page)
		if page.Page != 2 {
			t.Errorf("got page %d, want 2", page.Page)
		}
		if page.Limit != 10 {
			t.Errorf("got limit %d, want 10", page.Limit)
		}
		if len(page.Items) != 10 {
			t.Errorf("got %d items, want 10", len(page.Items))
		}
		if page.Total != 25 {
			t.Errorf("got total %d, want 25", page.Total)
		}
	})

	t.Run("bad page", func(t *testing.T) {
		srv, _ := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		for _, pageVal := range []string{"0", "-1"} {
			t.Run("page="+pageVal, func(t *testing.T) {
				u := ts.URL + "/api/v1/tickets?page=" + pageVal
				req := authedRequest(http.MethodGet, u, nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("GET /api/v1/tickets?page=%s: %v", pageVal, err)
				}
				defer func() { _ = resp.Body.Close() }()

				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
				}
			})
		}
	})

	t.Run("bad limit", func(t *testing.T) {
		srv, _ := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		for _, limitVal := range []string{"0", "101"} {
			t.Run("limit="+limitVal, func(t *testing.T) {
				u := ts.URL + "/api/v1/tickets?limit=" + limitVal
				req := authedRequest(http.MethodGet, u, nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("GET /api/v1/tickets?limit=%s: %v", limitVal, err)
				}
				defer func() { _ = resp.Body.Close() }()

				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
				}
			})
		}
	})
}

// ─── Get ──────────────────────────────────────────────────────────────────

func TestGetTicket(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.Ticket{
			Title: "Test Ticket", ProjectID: "proj-1",
			Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
		})

		u := fmt.Sprintf("%s/api/v1/tickets/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/tickets/%s: %v", orig.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var got model.Ticket
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
		srv, _ := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		id := uuid.NewString()
		u := fmt.Sprintf("%s/api/v1/tickets/%s", ts.URL, id)
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/tickets/%s: %v", id, err)
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

func TestUpdateTicket(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.Ticket{
			Title: "Update Me", ProjectID: "proj-1",
			Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
		})

		body := fmt.Sprintf(`{"id":%q,"title":"Updated","project_id":"proj-1","source":"github","status":"closed"}`, orig.ID)
		u := fmt.Sprintf("%s/api/v1/tickets/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/tickets/%s: %v", orig.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var updated model.Ticket
		mustDecode(t, resp, &updated)
		if updated.Title != "Updated" {
			t.Errorf("got title %q, want %q", updated.Title, "Updated")
		}
		if updated.Status != model.TicketStatusClosed {
			t.Errorf("got status %q, want %q", updated.Status, model.TicketStatusClosed)
		}
	})

	t.Run("not found", func(t *testing.T) {
		srv, _ := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		id := uuid.NewString()
		body := fmt.Sprintf(`{"id":%q,"title":"Ghost","project_id":"proj-1","source":"github","status":"open"}`, id)
		u := fmt.Sprintf("%s/api/v1/tickets/%s", ts.URL, id)
		req := authedRequest(http.MethodPut, u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/tickets/%s: %v", id, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("invalid body - missing title", func(t *testing.T) {
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.Ticket{
			Title: "Validate Me", ProjectID: "proj-1",
			Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
		})

		body := fmt.Sprintf(`{"id":%q,"project_id":"proj-1","source":"github","status":"open"}`, orig.ID)
		u := fmt.Sprintf("%s/api/v1/tickets/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/tickets/%s: %v", orig.ID, err)
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
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.Ticket{
			Title: "Bad Status", ProjectID: "proj-1",
			Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
		})

		body := fmt.Sprintf(`{"id":%q,"title":"Bad","project_id":"proj-1","source":"github","status":"bogus"}`, orig.ID)
		u := fmt.Sprintf("%s/api/v1/tickets/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/tickets/%s: %v", orig.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("id mismatch", func(t *testing.T) {
		srv, seed := setupTicketServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.Ticket{
			Title: "Mismatch Me", ProjectID: "proj-1",
			Source: model.TicketSourceGitHub, Status: model.TicketStatusOpen,
		})

		otherID := uuid.NewString()
		body := fmt.Sprintf(`{"id":%q,"title":"Mismatched","project_id":"proj-1","source":"github","status":"open"}`, otherID)
		u := fmt.Sprintf("%s/api/v1/tickets/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/tickets/%s: %v", orig.ID, err)
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

// ─── Method Not Allowed ──────────────────────────────────────────────────

func TestTicketMethodNotAllowed(t *testing.T) {
	srv, _ := setupTicketServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "post to tickets list", method: http.MethodPost, path: "/api/v1/tickets"},
		{name: "delete to ticket detail", method: http.MethodDelete, path: "/api/v1/tickets/some-id"},
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
