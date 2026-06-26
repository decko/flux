package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// SQLiteWebhookSecretRepository implements WebhookSecretRepository using a
// SQLite database. Secrets are stored in the webhook_secrets table keyed by
// repo_url.
type SQLiteWebhookSecretRepository struct {
	db *sqlx.DB
}

// NewSQLiteWebhookSecretRepository creates a new SQLiteWebhookSecretRepository
// backed by the given *sqlx.DB connection.
//
// The caller is responsible for configuring the underlying *sql.DB via
// ConfigureSQLiteDB before wrapping it with sqlx.NewDb.
func NewSQLiteWebhookSecretRepository(db *sqlx.DB) *SQLiteWebhookSecretRepository {
	return &SQLiteWebhookSecretRepository{db: db}
}

// Get retrieves the webhook secret for a given repo URL. Returns ErrNotFound
// if no secret exists for the repo URL.
func (r *SQLiteWebhookSecretRepository) Get(ctx context.Context, repoURL string) (string, error) {
	query := `SELECT secret FROM webhook_secrets WHERE repo_url = ?`
	var secret string
	err := r.db.QueryRowContext(ctx, query, repoURL).Scan(&secret)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("getting webhook secret: %w", err)
	}
	return secret, nil
}

// Set stores or updates the webhook secret for a given repo URL.
func (r *SQLiteWebhookSecretRepository) Set(ctx context.Context, repoURL, secret string) error {
	query := `INSERT OR REPLACE INTO webhook_secrets (repo_url, secret) VALUES (?, ?)`
	_, err := r.db.ExecContext(ctx, query, repoURL, secret)
	if err != nil {
		return fmt.Errorf("setting webhook secret: %w", err)
	}
	return nil
}

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
