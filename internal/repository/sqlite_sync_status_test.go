package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// setupSyncStatusTestDB opens an in-memory SQLite database, configures it for
// SQLite use, runs all migrations, and returns a SQLiteSyncStatusRepository
// and the underlying *sqlx.DB for seeding parent data.
func setupSyncStatusTestDB(t *testing.T) (*repository.SQLiteSyncStatusRepository, *sqlx.DB) {
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
	sdb := sqlx.NewDb(db, "sqlite")
	return repository.NewSQLiteSyncStatusRepository(sdb), sdb
}

// seedProjectForSyncStatus inserts a minimal project row so that foreign key
// constraints on sync_status.project_id are satisfied.
func seedProjectForSyncStatus(t *testing.T, sdb *sqlx.DB, projectID string) {
	t.Helper()
	now := time.Now().UTC()
	_, err := sdb.ExecContext(context.Background(),
		`INSERT INTO projects (id, name, repo_url, github_installation_id, webhook_id, definition, adapters, pipelines, created_at, updated_at)
		 VALUES (?, ?, ?, 0, 0, '{}', '[]', '[]', ?, ?)`,
		projectID, projectID, "https://github.com/"+projectID+"/repo", now, now,
	)
	if err != nil {
		t.Fatalf("seed project %s: %v", projectID, err)
	}
}

// testSyncStatusRow builds a SyncStatusRow with deterministic values for a
// given project.
func testSyncStatusRow(projectID string, ticketsSynced int) model.SyncStatusRow {
	now := time.Now().UTC().Truncate(time.Second)
	return model.SyncStatusRow{
		ProjectID:       projectID,
		LastSyncAt:      &now,
		LastSyncError:   "",
		TicketsSynced:   ticketsSynced,
		PRsSynced:       ticketsSynced / 2,
		WebhooksHealthy: true,
		UpdatedAt:       now,
	}
}

func TestUpsert_NewStatus(t *testing.T) {
	repo, sdb := setupSyncStatusTestDB(t)
	seedProjectForSyncStatus(t, sdb, "proj-1")
	ctx := context.Background()

	want := testSyncStatusRow("proj-1", 3)
	if err := repo.Upsert(ctx, want); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	got, err := repo.GetByProjectID(ctx, "proj-1")
	if err != nil {
		t.Fatalf("GetByProjectID returned error: %v", err)
	}
	if got.ProjectID != "proj-1" {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, "proj-1")
	}
	if got.LastSyncAt == nil {
		t.Fatal("expected non-nil LastSyncAt")
	}
	if !got.LastSyncAt.Equal(*want.LastSyncAt) {
		t.Errorf("got LastSyncAt %v, want %v", *got.LastSyncAt, *want.LastSyncAt)
	}
	if got.LastSyncError != "" {
		t.Errorf("got LastSyncError %q, want ''", got.LastSyncError)
	}
	if got.TicketsSynced != 3 {
		t.Errorf("got TicketsSynced %d, want 3", got.TicketsSynced)
	}
	if got.PRsSynced != want.PRsSynced {
		t.Errorf("got PRsSynced %d, want %d", got.PRsSynced, want.PRsSynced)
	}
	if !got.WebhooksHealthy {
		t.Error("expected WebhooksHealthy true")
	}
	if !got.UpdatedAt.Equal(want.UpdatedAt) {
		t.Errorf("got UpdatedAt %v, want %v", got.UpdatedAt, want.UpdatedAt)
	}
}

func TestUpsert_ExistingStatus(t *testing.T) {
	repo, sdb := setupSyncStatusTestDB(t)
	seedProjectForSyncStatus(t, sdb, "proj-1")
	ctx := context.Background()

	first := testSyncStatusRow("proj-1", 1)
	if err := repo.Upsert(ctx, first); err != nil {
		t.Fatalf("first Upsert returned error: %v", err)
	}

	second := testSyncStatusRow("proj-1", 7)
	second.LastSyncError = "ticket API error"
	second.WebhooksHealthy = false
	if err := repo.Upsert(ctx, second); err != nil {
		t.Fatalf("second Upsert returned error: %v", err)
	}

	// Second value must win — exactly one row for the project.
	rows, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows after double upsert, want 1", len(rows))
	}

	got, err := repo.GetByProjectID(ctx, "proj-1")
	if err != nil {
		t.Fatalf("GetByProjectID returned error: %v", err)
	}
	if got.TicketsSynced != 7 {
		t.Errorf("got TicketsSynced %d, want 7 (second upsert wins)", got.TicketsSynced)
	}
	if got.LastSyncError != "ticket API error" {
		t.Errorf("got LastSyncError %q, want %q", got.LastSyncError, "ticket API error")
	}
	if got.WebhooksHealthy {
		t.Error("expected WebhooksHealthy false (second upsert wins)")
	}
}

func TestGetByProjectID_NotFound(t *testing.T) {
	repo, _ := setupSyncStatusTestDB(t)
	ctx := context.Background()

	_, err := repo.GetByProjectID(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestList_Empty(t *testing.T) {
	repo, _ := setupSyncStatusTestDB(t)
	ctx := context.Background()

	rows, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if rows == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(rows) != 0 {
		t.Errorf("got %d rows, want 0", len(rows))
	}
}

func TestList_MultipleProjects(t *testing.T) {
	repo, sdb := setupSyncStatusTestDB(t)
	seedProjectForSyncStatus(t, sdb, "proj-1")
	seedProjectForSyncStatus(t, sdb, "proj-2")
	seedProjectForSyncStatus(t, sdb, "proj-3")
	ctx := context.Background()

	for _, pid := range []string{"proj-1", "proj-2", "proj-3"} {
		if err := repo.Upsert(ctx, testSyncStatusRow(pid, 1)); err != nil {
			t.Fatalf("Upsert %s: %v", pid, err)
		}
	}

	rows, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}

	seen := map[string]bool{}
	for _, row := range rows {
		if row.ProjectID == "" {
			t.Error("expected non-empty ProjectID on listed row")
		}
		seen[row.ProjectID] = true
	}
	for _, pid := range []string{"proj-1", "proj-2", "proj-3"} {
		if !seen[pid] {
			t.Errorf("List missing project %s", pid)
		}
	}
}

func TestCascadeDelete(t *testing.T) {
	repo, sdb := setupSyncStatusTestDB(t)
	ctx := context.Background()

	// Create a project through the real ProjectRepository so the FK parent
	// row exists exactly as production creates it.
	projectRepo := repository.NewSQLiteProjectRepository(sdb)
	now := time.Now().UTC()
	project := model.Project{
		ID:        "proj-1",
		Name:      "Test Project",
		RepoURL:   "https://github.com/test/repo",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	if err := repo.Upsert(ctx, testSyncStatusRow("proj-1", 2)); err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}

	// Sanity check: status exists before the delete.
	if _, err := repo.GetByProjectID(ctx, "proj-1"); err != nil {
		t.Fatalf("GetByProjectID before delete: %v", err)
	}

	// Deleting the project must cascade and remove the sync status row.
	if err := projectRepo.Delete(ctx, "proj-1"); err != nil {
		t.Fatalf("delete project: %v", err)
	}

	_, err := repo.GetByProjectID(ctx, "proj-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after cascade delete, got %v", err)
	}
}

func TestConcurrentUpsert(t *testing.T) {
	repo, sdb := setupSyncStatusTestDB(t)
	seedProjectForSyncStatus(t, sdb, "proj-1")
	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			row := testSyncStatusRow("proj-1", i)
			_ = repo.Upsert(ctx, row)
		}()
	}
	wg.Wait()

	// The final state must be a single consistent row — no duplicate
	// project rows, no corruption. Run with -race to detect data races.
	rows, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows after concurrent upserts, want 1", len(rows))
	}
	got, err := repo.GetByProjectID(ctx, "proj-1")
	if err != nil {
		t.Fatalf("GetByProjectID returned error: %v", err)
	}
	if got.ProjectID != "proj-1" {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, "proj-1")
	}
}
