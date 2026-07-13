package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ConfigureSQLiteDB configures a *sql.DB for SQLite use. It sets the
// connection pool for single-writer safety (SQLite serializes writes),
// enables WAL journal mode for concurrent reads, enables foreign key
// enforcement so that ON DELETE CASCADE and referential integrity
// constraints are honored, and disables the idle connection timeout.
// Call this once at application startup, before constructing any
// repository instances.
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

	// Enable foreign key enforcement so that ON DELETE CASCADE and
	// referential integrity constraints in migrations are honored.
	if _, err := db.ExecContext(context.Background(), "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enabling foreign keys: %w", err)
	}

	return nil
}
