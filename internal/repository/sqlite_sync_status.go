package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/decko/flux/internal/model"
)

// SQLiteSyncStatusRepository implements SyncStatusRepository using a SQLite
// database. The webhooks_healthy field is stored as INTEGER (0/1) and
// converted to/from bool on reads and writes. All time.Time values are
// normalized to UTC before storage.
type SQLiteSyncStatusRepository struct {
	db *sqlx.DB
}

// NewSQLiteSyncStatusRepository creates a new SQLiteSyncStatusRepository
// backed by the given *sqlx.DB connection.
//
// The caller is responsible for configuring the underlying *sql.DB via
// ConfigureSQLiteDB before wrapping it with sqlx.NewDb.
func NewSQLiteSyncStatusRepository(db *sqlx.DB) *SQLiteSyncStatusRepository {
	return &SQLiteSyncStatusRepository{db: db}
}

// Upsert persists the sync status for a project, creating the row if it does
// not exist and overwriting it otherwise. All time.Time values are normalized
// to UTC before storage and WebhooksHealthy is stored as INTEGER (0/1).
func (r *SQLiteSyncStatusRepository) Upsert(ctx context.Context, status model.SyncStatusRow) error {
	webhooksHealthy := 0
	if status.WebhooksHealthy {
		webhooksHealthy = 1
	}
	var lastSyncAt any
	if status.LastSyncAt != nil {
		lastSyncAt = status.LastSyncAt.UTC()
	}

	query := `INSERT OR REPLACE INTO sync_status (id, project_id, last_sync_at, last_sync_error, tickets_synced, prs_synced, webhooks_healthy, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		status.ID,
		status.ProjectID,
		lastSyncAt,
		status.LastSyncError,
		status.TicketsSynced,
		status.PRsSynced,
		webhooksHealthy,
		status.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("upserting sync status: %w", err)
	}
	return nil
}

// GetByProjectID retrieves the sync status for a single project. Returns
// ErrNotFound if no row exists for the project.
func (r *SQLiteSyncStatusRepository) GetByProjectID(ctx context.Context, projectID string) (model.SyncStatusRow, error) {
	query := `SELECT id, project_id, last_sync_at, last_sync_error, tickets_synced, prs_synced, webhooks_healthy, updated_at FROM sync_status WHERE project_id = ?`
	var row model.SyncStatusRow
	var lastSyncAt sql.NullTime
	var webhooksHealthy int
	err := r.db.QueryRowContext(ctx, query, projectID).Scan(
		&row.ID,
		&row.ProjectID,
		&lastSyncAt,
		&row.LastSyncError,
		&row.TicketsSynced,
		&row.PRsSynced,
		&webhooksHealthy,
		&row.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.SyncStatusRow{}, ErrNotFound
		}
		return model.SyncStatusRow{}, fmt.Errorf("getting sync status: %w", err)
	}
	if lastSyncAt.Valid {
		row.LastSyncAt = &lastSyncAt.Time
	}
	row.WebhooksHealthy = webhooksHealthy != 0
	return row, nil
}

// List returns all persisted sync status rows. Returns an empty non-nil slice
// when no rows exist.
func (r *SQLiteSyncStatusRepository) List(ctx context.Context) ([]model.SyncStatusRow, error) {
	query := `SELECT id, project_id, last_sync_at, last_sync_error, tickets_synced, prs_synced, webhooks_healthy, updated_at FROM sync_status`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing sync status rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	statuses := make([]model.SyncStatusRow, 0)
	for rows.Next() {
		var row model.SyncStatusRow
		var lastSyncAt sql.NullTime
		var webhooksHealthy int
		if err := rows.Scan(
			&row.ID,
			&row.ProjectID,
			&lastSyncAt,
			&row.LastSyncError,
			&row.TicketsSynced,
			&row.PRsSynced,
			&webhooksHealthy,
			&row.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning sync status row: %w", err)
		}
		if lastSyncAt.Valid {
			row.LastSyncAt = &lastSyncAt.Time
		}
		row.WebhooksHealthy = webhooksHealthy != 0
		statuses = append(statuses, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating sync status rows: %w", err)
	}
	return statuses, nil
}

// ensure interface compliance.
var _ SyncStatusRepository = (*SQLiteSyncStatusRepository)(nil)
