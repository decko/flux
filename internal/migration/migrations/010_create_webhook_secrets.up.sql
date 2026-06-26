CREATE TABLE IF NOT EXISTS webhook_secrets (
    repo_url TEXT PRIMARY KEY,
    secret TEXT NOT NULL
);
