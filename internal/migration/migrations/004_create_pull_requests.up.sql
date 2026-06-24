CREATE TABLE IF NOT EXISTS pull_requests (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    external_id TEXT NOT NULL,
    source TEXT NOT NULL,
    title TEXT NOT NULL,
    url TEXT NOT NULL,
    status TEXT NOT NULL,
    ticket_ids TEXT NOT NULL DEFAULT '[]',
    reviews TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
