-- 001_create_projects.sql
-- Creates the projects table for the flux project repository.
--
-- Nested types (definition, adapters, pipelines) are stored as JSON TEXT
-- columns and marshaled/unmarshaled on reads and writes.

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    repo_url TEXT NOT NULL,
    definition TEXT NOT NULL DEFAULT '{}',
    adapters TEXT NOT NULL DEFAULT '[]',
    pipelines TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
