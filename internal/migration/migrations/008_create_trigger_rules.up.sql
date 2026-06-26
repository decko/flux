-- 008_create_trigger_rules.up.sql
-- Creates the trigger_rules table for M10: UI-managed pipeline triggers.
-- Each rule maps a label to a pipeline for a project.

CREATE TABLE IF NOT EXISTS trigger_rules (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    pipeline   TEXT NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    priority   INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_trigger_rules_project ON trigger_rules(project_id, enabled);
