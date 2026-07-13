package repository

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// TestConfigureSQLiteDB_ForeignKeysEnabled verifies that ConfigureSQLiteDB
// enables SQLite foreign key enforcement via PRAGMA foreign_keys = ON.
// Without this, ON DELETE CASCADE constraints in migrations are silently
// ignored and referential integrity is not enforced.
func TestConfigureSQLiteDB_ForeignKeysEnabled(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("ConfigureSQLiteDB: %v", err)
	}

	// Query the current foreign_keys setting.
	var fkEnabled int
	if err := db.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Errorf("foreign_keys = %d, want 1 (enabled)", fkEnabled)
	}

	t.Log("ConfigureSQLiteDB correctly enables PRAGMA foreign_keys = ON")
}
