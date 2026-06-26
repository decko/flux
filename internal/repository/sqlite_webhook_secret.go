package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
<<<<<<< HEAD
	"time"
=======
>>>>>>> origin/main

	"github.com/jmoiron/sqlx"
)

// SQLiteWebhookSecretRepository implements WebhookSecretRepository using a
<<<<<<< HEAD
// SQLite database.
=======
// SQLite database. Secrets are stored in the webhook_secrets table keyed by
// repo_url.
>>>>>>> origin/main
type SQLiteWebhookSecretRepository struct {
	db *sqlx.DB
}

// NewSQLiteWebhookSecretRepository creates a new SQLiteWebhookSecretRepository
// backed by the given *sqlx.DB connection.
<<<<<<< HEAD
=======
//
// The caller is responsible for configuring the underlying *sql.DB via
// ConfigureSQLiteDB before wrapping it with sqlx.NewDb.
>>>>>>> origin/main
func NewSQLiteWebhookSecretRepository(db *sqlx.DB) *SQLiteWebhookSecretRepository {
	return &SQLiteWebhookSecretRepository{db: db}
}

<<<<<<< HEAD
// Get retrieves the webhook secret for the given project ID.
// Returns ErrNotFound if no secret exists for the project.
func (r *SQLiteWebhookSecretRepository) Get(ctx context.Context, projectID string) (string, error) {
	query := `SELECT secret FROM webhook_secrets WHERE project_id = ?`
	var secret string
	err := r.db.GetContext(ctx, &secret, query, projectID)
=======
// Get retrieves the webhook secret for a given repo URL. Returns ErrNotFound
// if no secret exists for the repo URL.
func (r *SQLiteWebhookSecretRepository) Get(ctx context.Context, repoURL string) (string, error) {
	query := `SELECT secret FROM webhook_secrets WHERE repo_url = ?`
	var secret string
	err := r.db.QueryRowContext(ctx, query, repoURL).Scan(&secret)
>>>>>>> origin/main
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
<<<<<<< HEAD
		return "", fmt.Errorf("get webhook secret: %w", err)
=======
		return "", fmt.Errorf("getting webhook secret: %w", err)
>>>>>>> origin/main
	}
	return secret, nil
}

<<<<<<< HEAD
// Set stores or updates the webhook secret for the given project ID.
func (r *SQLiteWebhookSecretRepository) Set(ctx context.Context, projectID, secret string) error {
	query := `INSERT INTO webhook_secrets (project_id, secret, created_at) VALUES (?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET secret = excluded.secret`
	_, err := r.db.ExecContext(ctx, query, projectID, secret, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("set webhook secret: %w", err)
=======
// Set stores or updates the webhook secret for a given repo URL.
func (r *SQLiteWebhookSecretRepository) Set(ctx context.Context, repoURL, secret string) error {
	query := `INSERT OR REPLACE INTO webhook_secrets (repo_url, secret) VALUES (?, ?)`
	_, err := r.db.ExecContext(ctx, query, repoURL, secret)
	if err != nil {
		return fmt.Errorf("setting webhook secret: %w", err)
>>>>>>> origin/main
	}
	return nil
}

<<<<<<< HEAD
// Delete removes the webhook secret for the given project ID.
// Returns nil if no secret exists (idempotent delete).
func (r *SQLiteWebhookSecretRepository) Delete(ctx context.Context, projectID string) error {
	query := `DELETE FROM webhook_secrets WHERE project_id = ?`
	_, err := r.db.ExecContext(ctx, query, projectID)
	if err != nil {
		return fmt.Errorf("delete webhook secret: %w", err)
	}
	return nil
}
=======
// Delete removes the webhook secret for a given repo URL. Returns ErrNotFound
// if no secret exists for the repo URL.
func (r *SQLiteWebhookSecretRepository) Delete(ctx context.Context, repoURL string) error {
	query := `DELETE FROM webhook_secrets WHERE repo_url = ?`
	result, err := r.db.ExecContext(ctx, query, repoURL)
	if err != nil {
		return fmt.Errorf("deleting webhook secret: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ensure interface compliance.
var _ WebhookSecretRepository = (*SQLiteWebhookSecretRepository)(nil)
>>>>>>> origin/main
