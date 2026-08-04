package repository

import (
	"context"

	"github.com/decko/flux/internal/model"
)

// SyncStatusRepository defines the contract for per-project sync status
// persistence. Sync status survives process restarts so the dashboard can
// show historical per-project sync results even before the next sync pass.
type SyncStatusRepository interface {
	// Upsert persists the sync status for a project, creating the row if it
	// does not exist and overwriting it otherwise.
	Upsert(ctx context.Context, status model.SyncStatusRow) error

	// GetByProjectID retrieves the sync status for a single project.
	// Returns ErrNotFound if no row exists for the project.
	GetByProjectID(ctx context.Context, projectID string) (model.SyncStatusRow, error)

	// List returns all persisted sync status rows. Returns an empty non-nil
	// slice when no rows exist.
	List(ctx context.Context) ([]model.SyncStatusRow, error)
}
