package model

import "time"

// SyncStatusRow is the persisted per-project sync status. It mirrors
// domain.ProjectSyncStatus and is stored in the sync_status table so that
// per-project sync results survive process restarts. Each project has at most
// one row, keyed by ProjectID; rows are removed when the owning project is
// deleted (ON DELETE CASCADE).
type SyncStatusRow struct {
	ProjectID       string
	LastSyncAt      *time.Time
	LastSyncError   string
	TicketsSynced   int
	PRsSynced       int
	WebhooksHealthy bool
	UpdatedAt       time.Time
}
