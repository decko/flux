package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ConfigureSQLiteDB configures a *sql.DB for SQLite use. It sets the
// connection pool for single-writer safety (SQLite serializes writes),
// enables WAL journal mode for concurrent reads, and disables the idle
// connection timeout. Call this once at application startup, before
// constructing any repository instances.
//
// Callers must ensure the "sqlite3" driver is registered:
//
//	import _ "modernc.org/sqlite"
//
// Driver registration is not done here to avoid pulling the driver into
// every package that imports the repository package.
func ConfigureSQLiteDB(db *sql.DB) error {
	// SQLite serializes writes — single connection for safety.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// WAL enables concurrent reads under write load.
	if _, err := db.ExecContext(context.Background(), "PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("enabling WAL mode: %w", err)
	}

	return nil
}
