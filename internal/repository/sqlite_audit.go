package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/decko/flux/internal/model"
)

// SQLiteAuditRepository implements AuditRepository using a SQLite database.
// Audit events are append-only — there is no Delete or Update method.
type SQLiteAuditRepository struct {
	db *sql.DB
}

// NewSQLiteAuditRepository creates a new SQLiteAuditRepository backed by the
// given *sql.DB connection. The caller is responsible for configuring the
// *sql.DB via ConfigureSQLiteDB before calling this constructor.
//
// The caller must also ensure the "sqlite3" driver is imported:
//
//	import _ "github.com/mattn/go-sqlite3"
func NewSQLiteAuditRepository(db *sql.DB) *SQLiteAuditRepository {
	return &SQLiteAuditRepository{db: db}
}

// DB returns the underlying *sql.DB for direct SQL access (e.g., testing).
func (r *SQLiteAuditRepository) DB() *sql.DB {
	return r.db
}

// Migrate creates the audit_events table if it does not already exist.
// The table stores the hash chain fields (previous_hash, hash) alongside
// the event payload for tamper detection.
func (r *SQLiteAuditRepository) Migrate(ctx context.Context) error {
	query := `CREATE TABLE IF NOT EXISTS audit_events (
		id TEXT PRIMARY KEY,
		actor_id TEXT NOT NULL,
		action TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		metadata TEXT NOT NULL DEFAULT '',
		previous_hash TEXT NOT NULL DEFAULT '',
		hash TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL
	)`
	if _, err := r.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("creating audit_events table: %w", err)
	}
	return nil
}

// Create persists a new audit event. The event's id, created_at, previous_hash,
// and hash fields must be populated by the caller. time.Time values are
// normalized to UTC before storage.
func (r *SQLiteAuditRepository) Create(ctx context.Context, event model.AuditEvent) error {
	query := `INSERT INTO audit_events (id, actor_id, action, resource_type, resource_id, metadata, previous_hash, hash, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		event.ID,
		event.ActorID,
		event.Action,
		event.ResourceType,
		event.ResourceID,
		event.Metadata,
		event.PreviousHash,
		event.Hash,
		event.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("creating audit event: %w", err)
	}
	return nil
}

// Latest returns the most recent audit event ordered by created_at DESC.
// Returns ErrNotFound if no events exist.
func (r *SQLiteAuditRepository) Latest(ctx context.Context) (model.AuditEvent, error) {
	query := `SELECT id, actor_id, action, resource_type, resource_id, metadata, previous_hash, hash, created_at FROM audit_events ORDER BY created_at DESC LIMIT 1`
	row := r.db.QueryRowContext(ctx, query)

	var event model.AuditEvent
	err := row.Scan(
		&event.ID,
		&event.ActorID,
		&event.Action,
		&event.ResourceType,
		&event.ResourceID,
		&event.Metadata,
		&event.PreviousHash,
		&event.Hash,
		&event.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AuditEvent{}, ErrNotFound
	}
	if err != nil {
		return model.AuditEvent{}, fmt.Errorf("getting latest audit event: %w", err)
	}
	return event, nil
}

// List returns all audit events ordered by created_at ASC.
// Returns an empty non-nil slice when no events exist.
func (r *SQLiteAuditRepository) List(ctx context.Context) ([]model.AuditEvent, error) {
	query := `SELECT id, actor_id, action, resource_type, resource_id, metadata, previous_hash, hash, created_at FROM audit_events ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing audit events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	events := make([]model.AuditEvent, 0)
	for rows.Next() {
		var event model.AuditEvent
		if err := rows.Scan(
			&event.ID,
			&event.ActorID,
			&event.Action,
			&event.ResourceType,
			&event.ResourceID,
			&event.Metadata,
			&event.PreviousHash,
			&event.Hash,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning audit event row: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating audit event rows: %w", err)
	}

	return events, nil
}

// Ensure interface compliance.
var _ AuditRepository = (*SQLiteAuditRepository)(nil)
