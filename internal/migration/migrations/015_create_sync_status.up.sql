-- 015_create_sync_status.up.sql
-- Creates the sync_status table for M284: persisted per-project sync status.
-- Each project has at most one row, keyed by project_id. Rows are removed
-- when the owning project is deleted (ON DELETE CASCADE).

CREATE TABLE IF NOT EXISTS sync_status (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    last_sync_at DATETIME,
    last_sync_error TEXT NOT NULL DEFAULT '',
    tickets_synced INTEGER NOT NULL DEFAULT 0,
    prs_synced INTEGER NOT NULL DEFAULT 0,
    webhooks_healthy INTEGER NOT NULL DEFAULT 1,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sync_status_project_id ON sync_status(project_id);
