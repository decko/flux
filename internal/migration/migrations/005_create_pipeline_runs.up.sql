CREATE TABLE IF NOT EXISTS pipeline_runs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    ticket_id TEXT NOT NULL,
    orchestrator TEXT NOT NULL,
    pipeline TEXT NOT NULL,
    status TEXT NOT NULL,
    phases TEXT NOT NULL DEFAULT '[]',
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    cost TEXT
);
