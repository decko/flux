package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// setupAuditServer creates an in-memory SQLite database, migrates it,
// creates an AuditService-backed Server, and returns the server.
func setupAuditServer(t *testing.T) *Server {
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
	repo := repository.NewSQLiteAuditRepository(db)
	svc := domain.NewAuditService(repo)
	return NewServer(WithJWTSecret(testJWTSecretBytes), WithAuditService(svc))
}

// seedAuditEvents inserts test audit events into the repository.
func seedAuditEvents(t *testing.T, repo *repository.SQLiteAuditRepository) {
	t.Helper()

	now := time.Now().UTC()

	events := []model.AuditEvent{
		{
			ID:           "evt-1",
			ActorID:      "user-1",
			Action:       "project.created",
			ResourceType: "project",
			ResourceID:   "proj-1",
			Metadata:     `{}`,
			CreatedAt:    now.Add(-3 * time.Hour),
		},
		{
			ID:           "evt-2",
			ActorID:      "user-2",
			Action:       "project.updated",
			ResourceType: "project",
			ResourceID:   "proj-1",
			Metadata:     `{"field":"name"}`,
			CreatedAt:    now.Add(-2 * time.Hour),
		},
		{
			ID:           "evt-3",
			ActorID:      "user-1",
			Action:       "ticket.created",
			ResourceType: "ticket",
			ResourceID:   "tkt-1",
			Metadata:     `{}`,
			CreatedAt:    now.Add(-1 * time.Hour),
		},
	}

	for _, e := range events {
		if err := repo.Insert(context.Background(), e); err != nil {
			t.Fatalf("seed: insert event %s: %v", e.ID, err)
		}
	}
}

// generateNonAdminToken creates a signed JWT token with the "user" role.
func generateNonAdminToken() string {
	claims := jwt.MapClaims{
		"sub":   "non-admin-user",
		"email": "user@example.com",
		"role":  "user",
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(testJWTSecretBytes)
	return tokenStr
}

// ─── HandleAuditEvents ──────────────────────────────────────────────────────

func TestHandleAuditEvents(t *testing.T) {
	// Seed events via the repository directly.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}
	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Need to seed into the same DB that the server uses.
	// Instead, let's use a different approach — build the server with seeded data.
	t.Run("returns all audit events", func(t *testing.T) {
		// Create a fresh server with seeded data.
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
		repo := repository.NewSQLiteAuditRepository(db)
		seedAuditEvents(t, repo)

		svc := domain.NewAuditService(repo)
		srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithAuditService(svc))
		ts := httptest.NewServer(srv)
		defer ts.Close()

		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/audit-events", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/audit-events: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var events []model.AuditEvent
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			t.Fatalf("decode JSON response: %v", err)
		}

		if len(events) != 3 {
			t.Fatalf("got %d events, want 3", len(events))
		}

		// Events should be ordered by created_at DESC.
		if events[0].ID != "evt-3" {
			t.Errorf("events[0].ID = %q, want %q (most recent first)", events[0].ID, "evt-3")
		}
		if events[2].ID != "evt-1" {
			t.Errorf("events[2].ID = %q, want %q (oldest last)", events[2].ID, "evt-1")
		}
	})
}

func TestHandleAuditEvents_FilterByResource(t *testing.T) {
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
	repo := repository.NewSQLiteAuditRepository(db)
	seedAuditEvents(t, repo)

	svc := domain.NewAuditService(repo)
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithAuditService(svc))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("filter by resource_type", func(t *testing.T) {
		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/audit-events?resource_type=ticket", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/audit-events: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var events []model.AuditEvent
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			t.Fatalf("decode JSON response: %v", err)
		}

		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}
		if events[0].ResourceType != "ticket" {
			t.Errorf("ResourceType = %q, want %q", events[0].ResourceType, "ticket")
		}
	})

	t.Run("filter by actor_id", func(t *testing.T) {
		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/audit-events?actor_id=user-2", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/audit-events: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var events []model.AuditEvent
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			t.Fatalf("decode JSON response: %v", err)
		}

		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}
		if events[0].ActorID != "user-2" {
			t.Errorf("ActorID = %q, want %q", events[0].ActorID, "user-2")
		}
	})

	t.Run("filter by limit", func(t *testing.T) {
		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/audit-events?limit=2", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/audit-events: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var events []model.AuditEvent
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			t.Fatalf("decode JSON response: %v", err)
		}

		if len(events) != 2 {
			t.Fatalf("got %d events, want 2", len(events))
		}
	})

	t.Run("filter by action", func(t *testing.T) {
		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/audit-events?action=project.updated", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/audit-events: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var events []model.AuditEvent
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			t.Fatalf("decode JSON response: %v", err)
		}

		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}
		if events[0].Action != model.AuditAction("project.updated") {
			t.Errorf("Action = %q, want %q", events[0].Action, model.AuditAction("project.updated"))
		}
	})

	t.Run("empty result for no match", func(t *testing.T) {
		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/audit-events?actor_id=nonexistent", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/audit-events: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var events []model.AuditEvent
		if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
			t.Fatalf("decode JSON response: %v", err)
		}

		if len(events) != 0 {
			t.Fatalf("got %d events, want 0", len(events))
		}
	})
}

func TestHandleAuditEvents_Unauthorized(t *testing.T) {
	srv := setupAuditServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Request without Authorization header.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/v1/audit-events", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/audit-events: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	var errResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if _, ok := errResp["error"]; !ok {
		t.Error("error response missing 'error' field")
	}
}

func TestHandleAuditEvents_NotAdmin(t *testing.T) {
	srv := setupAuditServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Request with non-admin token.
	nonAdminToken := generateNonAdminToken()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/v1/audit-events", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+nonAdminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/audit-events: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	var errResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if _, ok := errResp["error"]; !ok {
		t.Error("error response missing 'error' field")
	}
}

func TestHandleAuditEvents_EmptyList(t *testing.T) {
	srv := setupAuditServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/audit-events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/audit-events: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var events []model.AuditEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("got %d events, want 0", len(events))
	}

	if events == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
}
