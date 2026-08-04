-- 015_create_sync_status.up.sql
-- Creates the sync_status table for M284: persisted per-project sync status.
-- Each project has at most one row, keyed by project_id, which is the natural
-- primary key (consistent with webhook_secrets). Rows are removed when the
-- owning project is deleted (ON DELETE CASCADE). No extra index is needed:
-- SQLite auto-indexes primary keys.

CREATE TABLE IF NOT EXISTS sync_status (
    project_id TEXT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    last_sync_at DATETIME,
    last_sync_error TEXT NOT NULL DEFAULT '',
    tickets_synced INTEGER NOT NULL DEFAULT 0,
    prs_synced INTEGER NOT NULL DEFAULT 0,
    webhooks_healthy INTEGER NOT NULL DEFAULT 1,
    updated_at DATETIME NOT NULL
);
