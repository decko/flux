package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/decko/flux/internal/model"
)

// SQLiteAuditRepository implements AuditRepository using a SQLite database.
// Audit events are append-only immutable records stored in the audit_events
// table with indexes on actor_id, resource, and created_at for efficient
// filtering and ordering.
type SQLiteAuditRepository struct {
	db *sql.DB
}

// NewSQLiteAuditRepository creates a new SQLiteAuditRepository backed by
// the given *sql.DB connection.
//
// The caller is responsible for configuring the *sql.DB via ConfigureSQLiteDB
// before calling this constructor. NewSQLiteAuditRepository does not mutate
// the connection pool — it only holds a reference to the already-configured
// database handle.
//
// The caller must also ensure the "sqlite3" driver is imported:
//
//	import _ "modernc.org/sqlite"
func NewSQLiteAuditRepository(db *sql.DB) *SQLiteAuditRepository {
	return &SQLiteAuditRepository{db: db}
}

// Migrate creates the audit_events table and associated indexes if they do not
// already exist. Safe to call multiple times.
func (r *SQLiteAuditRepository) Migrate(ctx context.Context) error {
	query := `CREATE TABLE IF NOT EXISTS audit_events (
		id TEXT PRIMARY KEY,
		actor_id TEXT NOT NULL,
		action TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		metadata TEXT NOT NULL DEFAULT '{}',
		previous_hash TEXT NOT NULL DEFAULT '',
		hash TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_events(actor_id);
	CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_events(resource_type, resource_id);
	CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_events(created_at);`
	if _, err := r.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("creating audit_events table: %w", err)
	}
	return nil
}

// Insert persists a new audit event. If the event's ID is empty, a UUID is
// generated automatically. If CreatedAt is zero, the current UTC time is used.
func (r *SQLiteAuditRepository) Insert(ctx context.Context, event model.AuditEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
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
		return fmt.Errorf("inserting audit event: %w", err)
	}
	return nil
}

// List returns audit events matching the given filter criteria. Events are
// ordered by created_at descending (most recent first). Zero values in the
// filter are ignored. Returns an empty non-nil slice when no events match.
func (r *SQLiteAuditRepository) List(ctx context.Context, filter AuditFilter) ([]model.AuditEvent, error) {
	var where []string
	var args []any

	if filter.ActorID != "" {
		where = append(where, "actor_id = ?")
		args = append(args, filter.ActorID)
	}
	if filter.ResourceType != "" {
		where = append(where, "resource_type = ?")
		args = append(args, filter.ResourceType)
	}
	if filter.ResourceID != "" {
		where = append(where, "resource_id = ?")
		args = append(args, filter.ResourceID)
	}
	if filter.Action != "" {
		where = append(where, "action = ?")
		args = append(args, filter.Action)
	}
	if !filter.Since.IsZero() {
		where = append(where, "created_at >= ?")
		args = append(args, filter.Since.UTC())
	}
	if !filter.Until.IsZero() {
		where = append(where, "created_at <= ?")
		args = append(args, filter.Until.UTC())
	}

	query := "SELECT id, actor_id, action, resource_type, resource_id, metadata, previous_hash, hash, created_at FROM audit_events"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
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

// Latest returns the most recent audit event by created_at, or nil if no
// events exist. Used by the hash chain to link consecutive events.
func (r *SQLiteAuditRepository) Latest(ctx context.Context) (*model.AuditEvent, error) {
	query := "SELECT id, actor_id, action, resource_type, resource_id, metadata, previous_hash, hash, created_at FROM audit_events ORDER BY created_at DESC LIMIT 1"
	row := r.db.QueryRowContext(ctx, query)
	var event model.AuditEvent
	if err := row.Scan(
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("latest audit event: %w", err)
	}
	return &event, nil
}

// PurgeOlderThan deletes audit events older than the given time.
// Returns the count of deleted rows.
func (r *SQLiteAuditRepository) PurgeOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, "DELETE FROM audit_events WHERE created_at < ?", before.UTC())
	if err != nil {
		return 0, fmt.Errorf("purging: %w", err)
	}
	return result.RowsAffected()
}
