package migration

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// TestUp_CreatesAllTables verifies that running Up() creates all expected
// tables and indexes from the embedded migration files.
func TestUp_CreatesAllTables(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Run migrations.
	if err := Up(db); err != nil {
		t.Fatalf("Up failed: %v", err)
	}

	// Verify all tables exist.
	tables := []string{"users", "projects", "tickets", "pull_requests", "pipeline_runs", "audit_events", "trigger_rules", "webhook_secrets"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(context.Background(),
			"SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&count)
		if err != nil {
			t.Errorf("checking %s: %v", table, err)
		} else if count != 1 {
			t.Errorf("table %s: expected to exist, got count=%d", table, count)
		}
	}

	// Verify indexes on audit_events.
	indexes := []string{"idx_audit_actor", "idx_audit_resource", "idx_audit_created"}
	for _, idx := range indexes {
		var count int
		if err := db.QueryRowContext(context.Background(),
			"SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?",
			idx,
		).Scan(&count); err != nil {
			t.Errorf("checking index %s: %v", idx, err)
		} else if count != 1 {
			t.Errorf("index %s: expected to exist", idx)
		}
	}

	// Verify schema_migrations table tracks applied migrations.
	var versionCount int
	if err := db.QueryRowContext(context.Background(), "SELECT count(*) FROM schema_migrations").Scan(&versionCount); err != nil {
		t.Errorf("schema_migrations query: %v", err)
	} else if versionCount < 1 {
		t.Errorf("expected at least 1 migration record, got %d", versionCount)
	}

	// Verify trigger_rules has event column (migration 011).
	var eventColCount int
	if err := db.QueryRowContext(context.Background(),
		"SELECT count(*) FROM pragma_table_info('trigger_rules') WHERE name='event'",
	).Scan(&eventColCount); err != nil {
		t.Errorf("checking trigger_rules.event column: %v", err)
	} else if eventColCount != 1 {
		t.Errorf("trigger_rules.event column should exist after migration 011, got count=%d", eventColCount)
	}

	// Verify projects has webhook_id column (migration 012).
	var whColCount int
	if err := db.QueryRowContext(context.Background(),
		"SELECT count(*) FROM pragma_table_info('projects') WHERE name='webhook_id'",
	).Scan(&whColCount); err != nil {
		t.Errorf("checking projects.webhook_id column: %v", err)
	} else if whColCount != 1 {
		t.Errorf("projects.webhook_id column should exist after migration 012, got count=%d", whColCount)
	}

	// Verify projects has last_webhook_at column (migration 014).
	var lwhColCount int
	if err := db.QueryRowContext(context.Background(),
		"SELECT count(*) FROM pragma_table_info('projects') WHERE name='last_webhook_at'",
	).Scan(&lwhColCount); err != nil {
		t.Errorf("checking projects.last_webhook_at column: %v", err)
	} else if lwhColCount != 1 {
		t.Errorf("projects.last_webhook_at column should exist after migration 014, got count=%d", lwhColCount)
	}

	t.Log("migration smoke test: all 8 tables + 3 indexes + schema_migrations + event/webhook_id/last_webhook_at columns verified")
}

// TestUp_Idempotent verifies that running Up() twice does not fail.
// TestUp_ProjectsTableInstallationIDColumn verifies that the projects table
// has the github_installation_id column after migration 007 is applied.
func TestUp_ProjectsTableInstallationIDColumn(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := Up(db); err != nil {
		t.Fatalf("Up failed: %v", err)
	}

	// Verify the column exists (added by migration 007).
	var count int
	err = db.QueryRowContext(context.Background(),
		"SELECT count(*) FROM pragma_table_info('projects') WHERE name='github_installation_id'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("checking column: %v", err)
	}
	if count != 1 {
		t.Errorf("github_installation_id column should exist after migration 007, got count=%d", count)
	}

	// Verify the current migration version is at least 7.
	var version int
	err = db.QueryRowContext(context.Background(), "SELECT max(version) FROM schema_migrations").Scan(&version)
	if err != nil {
		t.Fatalf("checking schema version: %v", err)
	}
	if version < 7 {
		t.Errorf("expected at least migration version 7, got %d", version)
	}

	t.Log("projects table has github_installation_id column (migration 007 applied)")
}

func TestUp_Idempotent(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := Up(db); err != nil {
		t.Fatalf("first Up: %v", err)
	}
	if err := Up(db); err != nil {
		t.Fatalf("second Up should be idempotent, got: %v", err)
	}

	t.Log("migration Up is idempotent")
}
