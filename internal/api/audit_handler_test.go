package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// setupAuditServer creates an in-memory SQLite database, migrates it,
// creates an AuditService-backed Server, and returns the server together
// with the service and repo for test seeding and tampering.
func setupAuditServer(t *testing.T) (*Server, *domain.AuditService, *repository.SQLiteAuditRepository) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}

	repo := repository.NewSQLiteAuditRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to run migration: %v", err)
	}

	svc := domain.NewAuditService(repo)
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithAuditService(svc))

	return srv, svc, repo
}

// integrityResponse is the JSON shape of the /api/v1/audit/integrity endpoint.
type integrityResponse struct {
	Valid         bool   `json:"valid"`
	FirstBrokenAt string `json:"first_broken_at"`
}

func TestAuditIntegrity_ValidChain(t *testing.T) {
	srv, svc, _ := setupAuditServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Seed two audit events.
	events := []model.AuditEvent{
		{ID: "evt-1", ActorID: "user-1", Action: "create", ResourceType: "project", ResourceID: "proj-1"},
		{ID: "evt-2", ActorID: "user-1", Action: "update", ResourceType: "project", ResourceID: "proj-1"},
	}
	for i := range events {
		if err := svc.Record(context.Background(), &events[i]); err != nil {
			t.Fatalf("Record event %d: %v", i, err)
		}
	}

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/audit/integrity", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/audit/integrity: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result integrityResponse
	mustDecode(t, resp, &result)
	if !result.Valid {
		t.Errorf("expected valid=true for clean chain, got false")
	}
	if result.FirstBrokenAt != "" {
		t.Errorf("expected empty first_broken_at, got %q", result.FirstBrokenAt)
	}
}

func TestAuditIntegrity_TamperedChain(t *testing.T) {
	srv, svc, repo := setupAuditServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Seed two audit events.
	events := []model.AuditEvent{
		{ID: "evt-1", ActorID: "user-1", Action: "create", ResourceType: "project", ResourceID: "proj-1"},
		{ID: "evt-2", ActorID: "user-1", Action: "update", ResourceType: "project", ResourceID: "proj-1"},
	}
	for i := range events {
		if err := svc.Record(context.Background(), &events[i]); err != nil {
			t.Fatalf("Record event %d: %v", i, err)
		}
	}

	// Tamper with the first event's hash directly in SQLite.
	if _, err := repo.DB().ExecContext(context.Background(),
		"UPDATE audit_events SET hash = 'tampered' WHERE id = 'evt-1'"); err != nil {
		t.Fatalf("tampering with hash: %v", err)
	}

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/audit/integrity", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/audit/integrity: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result integrityResponse
	mustDecode(t, resp, &result)
	if result.Valid {
		t.Errorf("expected valid=false for tampered chain, got true")
	}
	if result.FirstBrokenAt == "" {
		t.Errorf("expected non-empty first_broken_at for tampered chain")
	}
}

func TestAuditIntegrity_EmptyStore(t *testing.T) {
	srv, _, _ := setupAuditServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodGet, ts.URL+"/api/v1/audit/integrity", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/audit/integrity: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result integrityResponse
	mustDecode(t, resp, &result)
	if !result.Valid {
		t.Errorf("expected valid=true for empty store")
	}
	if result.FirstBrokenAt != "" {
		t.Errorf("expected empty first_broken_at, got %q", result.FirstBrokenAt)
	}
}

func TestAuditIntegrity_Unauthenticated(t *testing.T) {
	srv, _, _ := setupAuditServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// No Authorization header — should be 401.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/api/v1/audit/integrity", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/audit/integrity: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
