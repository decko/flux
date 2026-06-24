package migration

import (
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
	tables := []string{"users", "projects", "tickets", "pull_requests", "pipeline_runs", "audit_events"}
	for _, table := range tables {
		var count int
		err := db.QueryRow(
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
		if err := db.QueryRow(
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
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&versionCount); err != nil {
		t.Errorf("schema_migrations query: %v", err)
	} else if versionCount < 1 {
		t.Errorf("expected at least 1 migration record, got %d", versionCount)
	}

	t.Log("migration smoke test: all 6 tables + 3 indexes + schema_migrations verified")
}

// TestUp_Idempotent verifies that running Up() twice does not fail.
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
