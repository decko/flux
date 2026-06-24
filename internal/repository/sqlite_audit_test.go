package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/repository"
)

// setupAuditTestDB creates an in-memory SQLite database, configures it,
// and runs the audit migration. Returns both the repo and the raw *sql.DB
// for seeding test data directly.
func setupAuditTestDB(t *testing.T) (*repository.SQLiteAuditRepository, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
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
	return repo, db
}

// insertAuditEvent inserts a raw row into the audit_events table for testing.
func insertAuditEvent(t *testing.T, db *sql.DB, id, actorID, action, resourceType, resourceID string, createdAt time.Time) {
	t.Helper()

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO audit_events (id, actor_id, action, resource_type, resource_id, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, actorID, action, resourceType, resourceID, "", createdAt.UTC(),
	)
	if err != nil {
		t.Fatalf("insert audit event %s: %v", id, err)
	}
}

func TestAuditRepository_PurgeOlderThan_DeletesOld(t *testing.T) {
	repo, db := setupAuditTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	oldTime := now.Add(-100 * 24 * time.Hour) // 100 days old
	recentTime := now.Add(-24 * time.Hour)    // 1 day old

	insertAuditEvent(t, db, "e1", "user-1", "project.create", "project", "p1", oldTime)
	insertAuditEvent(t, db, "e2", "user-2", "project.create", "project", "p2", recentTime)
	insertAuditEvent(t, db, "e3", "user-1", "ticket.update", "ticket", "t1", oldTime)

	cutoff := now.Add(-30 * 24 * time.Hour) // 30 days ago
	count, err := repo.PurgeOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("PurgeOlderThan returned error: %v", err)
	}

	if count != 2 {
		t.Errorf("deleted count = %d, want 2", count)
	}

	// Verify remaining rows.
	var remaining int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events").Scan(&remaining)
	if err != nil {
		t.Fatalf("count remaining rows: %v", err)
	}
	if remaining != 1 {
		t.Errorf("remaining rows = %d, want 1", remaining)
	}
}

func TestAuditRepository_PurgeOlderThan_KeepsNew(t *testing.T) {
	repo, db := setupAuditTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	insertAuditEvent(t, db, "e1", "user-1", "project.create", "project", "p1", now)
	insertAuditEvent(t, db, "e2", "user-1", "ticket.update", "ticket", "t1", now)

	cutoff := now.Add(-30 * 24 * time.Hour)
	count, err := repo.PurgeOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("PurgeOlderThan returned error: %v", err)
	}

	if count != 0 {
		t.Errorf("deleted count = %d, want 0", count)
	}

	var remaining int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events").Scan(&remaining)
	if err != nil {
		t.Fatalf("count remaining rows: %v", err)
	}
	if remaining != 2 {
		t.Errorf("remaining rows = %d, want 2", remaining)
	}
}

func TestAuditRepository_PurgeOlderThan_AllOld(t *testing.T) {
	repo, db := setupAuditTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	oldTime := now.Add(-100 * 24 * time.Hour)

	insertAuditEvent(t, db, "e1", "user-1", "project.create", "project", "p1", oldTime)
	insertAuditEvent(t, db, "e2", "user-2", "ticket.update", "ticket", "t1", oldTime.Add(-24*time.Hour))

	cutoff := now.Add(-30 * 24 * time.Hour)
	count, err := repo.PurgeOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("PurgeOlderThan returned error: %v", err)
	}

	if count != 2 {
		t.Errorf("deleted count = %d, want 2", count)
	}

	var remaining int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events").Scan(&remaining)
	if err != nil {
		t.Fatalf("count remaining rows: %v", err)
	}
	if remaining != 0 {
		t.Errorf("remaining rows = %d, want 0", remaining)
	}
}

func TestAuditRepository_PurgeOlderThan_EmptyTable(t *testing.T) {
	repo, _ := setupAuditTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	count, err := repo.PurgeOlderThan(ctx, now)
	if err != nil {
		t.Fatalf("PurgeOlderThan returned error: %v", err)
	}

	if count != 0 {
		t.Errorf("deleted count = %d, want 0", count)
	}
}

func TestAuditRepository_PurgeOlderThan_ExactCutoff(t *testing.T) {
	repo, db := setupAuditTestDB(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	before := now.Add(-30 * 24 * time.Hour)

	// Event at exactly the cutoff boundary.
	insertAuditEvent(t, db, "e1", "user-1", "project.create", "project", "p1", before)

	count, err := repo.PurgeOlderThan(ctx, before)
	if err != nil {
		t.Fatalf("PurgeOlderThan returned error: %v", err)
	}

	// Events at exactly the cutoff are NOT older than the cutoff.
	if count != 0 {
		t.Errorf("deleted count = %d, want 0 (exact boundary should not be deleted)", count)
	}

	// Event one second before cutoff.
	insertAuditEvent(t, db, "e2", "user-1", "project.create", "project", "p2", before.Add(-time.Second))
	count2, err := repo.PurgeOlderThan(ctx, before)
	if err != nil {
		t.Fatalf("PurgeOlderThan returned error: %v", err)
	}
	if count2 != 1 {
		t.Errorf("deleted count = %d, want 1", count2)
	}
}
