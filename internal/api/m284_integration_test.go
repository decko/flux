package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/adapter/scm"
	"github.com/decko/flux/internal/adapter/ticket"
	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Minimal adapter stubs for the functional test ──────────────────────────

// m284TicketAdapter is a minimal TicketAdapter that returns a fixed ticket
// list, avoiding any external network dependency in the test.
type m284TicketAdapter struct {
	tickets []model.Ticket
	err     error
}

func (a *m284TicketAdapter) Name() string { return "github" }

func (a *m284TicketAdapter) ListTickets(_ context.Context, _ string) ([]model.Ticket, error) {
	return a.tickets, a.err
}

func (a *m284TicketAdapter) GetTicket(_ context.Context, _, _ string) (*model.Ticket, error) {
	return nil, errors.New("not implemented")
}

func (a *m284TicketAdapter) CreateTicket(_ context.Context, _ *model.Ticket) (*model.Ticket, error) {
	return nil, errors.New("not implemented")
}

func (a *m284TicketAdapter) UpdateTicket(_ context.Context, _ *model.Ticket) error {
	return errors.New("not implemented")
}

func (a *m284TicketAdapter) SyncRelationships(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (a *m284TicketAdapter) Health(_ context.Context) error { return nil }

// m284SCMAdapter is a minimal SCMAdapter that returns a fixed PR list.
type m284SCMAdapter struct {
	prs []model.PullRequest
	err error
}

func (a *m284SCMAdapter) Name() string { return "github" }

func (a *m284SCMAdapter) ListPullRequests(_ context.Context, _ string) ([]model.PullRequest, error) {
	return a.prs, a.err
}

func (a *m284SCMAdapter) GetPullRequest(_ context.Context, _, _ string) (*model.PullRequest, error) {
	return nil, errors.New("not implemented")
}

func (a *m284SCMAdapter) ListReviews(_ context.Context, _, _ string) ([]model.Review, error) {
	return nil, errors.New("not implemented")
}

func (a *m284SCMAdapter) Health(_ context.Context) error { return nil }

func m284SampleTicket(projectID, externalID string) model.Ticket {
	now := time.Now().UTC().Truncate(time.Second)
	return model.Ticket{
		ProjectID:     projectID,
		ExternalID:    externalID,
		Source:        model.TicketSourceGitHub,
		Title:         "Ticket " + externalID,
		Description:   "Description for " + externalID,
		Status:        model.TicketStatusOpen,
		Labels:        []string{},
		Relationships: []model.Relationship{},
		PRs:           []string{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func m284SamplePR(projectID, externalID string) model.PullRequest {
	now := time.Now().UTC().Truncate(time.Second)
	return model.PullRequest{
		ProjectID:  projectID,
		ExternalID: externalID,
		Source:     model.PRSourceGitHub,
		Title:      "PR " + externalID,
		URL:        "https://github.com/test/repo/pull/" + externalID,
		Status:     model.PRStatusOpen,
		TicketIDs:  []string{},
		Reviews:    []model.Review{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// ─── Functional Test: Persistence Across Restart ────────────────────────────

// TestM284_SyncStatusPersistsAcrossRestart verifies end-to-end that per-project
// sync status is persisted to the database and survives a service restart
// (modeled as a fresh SyncService instance backed by the same repositories).
func TestM284_SyncStatusPersistsAcrossRestart(t *testing.T) {
	// 1. In-memory SQLite with migrations.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("configure sqlite: %v", err)
	}
	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlite")

	projectRepo := repository.NewSQLiteProjectRepository(sdb)
	ticketRepo := repository.NewSQLiteTicketRepository(sdb)
	prRepo := repository.NewSQLitePullRequestRepository(sdb)
	syncStatusRepo := repository.NewSQLiteSyncStatusRepository(sdb)

	// 2. Seed a project (syncOnce requires the project to exist).
	now := time.Now().UTC()
	if err := projectRepo.Create(t.Context(), model.Project{
		ID:        "proj-1",
		Name:      "Test Project",
		RepoURL:   "https://github.com/test/repo",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// 3. First SyncService instance (pre-restart) with real repos + stub adapters.
	factory := domain.AdapterFactory(func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
		if projectID == "proj-1" {
			return &m284TicketAdapter{
					tickets: []model.Ticket{m284SampleTicket("proj-1", "ext-1")},
				}, &m284SCMAdapter{
					prs: []model.PullRequest{m284SamplePR("proj-1", "ext-101")},
				}, nil
		}
		return nil, nil, fmt.Errorf("unknown project: %s", projectID)
	})
	syncSvc := domain.NewSyncService(ticketRepo, prRepo, projectRepo, factory, time.Hour)
	syncSvc.WithSyncStatusRepository(syncStatusRepo)

	// 4. Run a sync.
	if err := syncSvc.SyncNow(t.Context()); err != nil {
		t.Fatalf("SyncNow: %v", err)
	}

	// 5. First server returns per-project data in /sync/status.
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithSyncService(syncSvc))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	authHeader := "Bearer " + generateTestToken()
	status1 := getSyncStatus(t, ts.URL+"/api/v1/sync/status", authHeader)
	p1, ok := status1.Projects["proj-1"]
	if !ok {
		t.Fatalf("expected proj-1 in /sync/status projects, got %d projects", len(status1.Projects))
	}
	if p1.TicketsSynced != 1 {
		t.Errorf("pre-restart: got TicketsSynced %d, want 1", p1.TicketsSynced)
	}
	if p1.PRsSynced != 1 {
		t.Errorf("pre-restart: got PRsSynced %d, want 1", p1.PRsSynced)
	}
	if p1.LastSyncAt == nil {
		t.Error("pre-restart: expected non-nil last_sync_at")
	}

	// 6. Second SyncService — a fresh instance sharing the same repositories,
	// simulating a process restart. Status must be loaded from the DB.
	syncSvc2 := domain.NewSyncService(ticketRepo, prRepo, projectRepo, factory, time.Hour)
	syncSvc2.WithSyncStatusRepository(syncStatusRepo)

	status2 := syncSvc2.Status()
	ps, ok := status2.Projects["proj-1"]
	if !ok {
		t.Fatalf("expected persisted per-project status after restart, got %d projects", len(status2.Projects))
	}
	if ps.TicketsSynced != 1 {
		t.Errorf("post-restart: got TicketsSynced %d, want 1", ps.TicketsSynced)
	}
	if ps.PRsSynced != 1 {
		t.Errorf("post-restart: got PRsSynced %d, want 1", ps.PRsSynced)
	}
	if ps.LastSyncAt == nil {
		t.Error("post-restart: expected non-nil LastSyncAt")
	}

	// 7. Server backed by the second service also returns per-project data.
	srv2 := NewServer(WithJWTSecret(testJWTSecretBytes), WithSyncService(syncSvc2))
	ts2 := httptest.NewServer(srv2)
	defer ts2.Close()

	status3 := getSyncStatus(t, ts2.URL+"/api/v1/sync/status", authHeader)
	p2, ok := status3.Projects["proj-1"]
	if !ok {
		t.Fatalf("expected proj-1 in post-restart /sync/status projects, got %d projects", len(status3.Projects))
	}
	if p2.TicketsSynced != 1 {
		t.Errorf("post-restart API: got TicketsSynced %d, want 1", p2.TicketsSynced)
	}
	if p2.PRsSynced != 1 {
		t.Errorf("post-restart API: got PRsSynced %d, want 1", p2.PRsSynced)
	}
	// Top-level aggregates must be recomputed from the persisted rows.
	if status3.TicketsSynced != 1 {
		t.Errorf("post-restart aggregate: got TicketsSynced %d, want 1", status3.TicketsSynced)
	}
	if status3.PRsSynced != 1 {
		t.Errorf("post-restart aggregate: got PRsSynced %d, want 1", status3.PRsSynced)
	}
	if status3.LastSyncAt == nil {
		t.Error("post-restart aggregate: expected non-nil last_sync_at")
	}
	if !status3.WebhooksHealthy {
		t.Error("post-restart aggregate: expected webhooks_healthy true")
	}
	if status3.LastSyncError != "" {
		t.Errorf("post-restart aggregate: got last_sync_error %q, want ''", status3.LastSyncError)
	}
}

// getSyncStatus performs an authenticated GET against /api/v1/sync/status and
// decodes the response into syncStatusResponse.
func getSyncStatus(t *testing.T, url, authHeader string) syncStatusResponse {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", authHeader)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: got status %d, want 200", url, resp.StatusCode)
	}
	var status syncStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode sync status: %v", err)
	}
	return status
}
