package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// SQLiteWebhookSecretRepository implements WebhookSecretRepository.
type SQLiteWebhookSecretRepository struct {
	db *sqlx.DB
}

var _ WebhookSecretRepository = (*SQLiteWebhookSecretRepository)(nil)

// NewSQLiteWebhookSecretRepository creates a new SQLiteWebhookSecretRepository.
func NewSQLiteWebhookSecretRepository(db *sqlx.DB) *SQLiteWebhookSecretRepository {
	return &SQLiteWebhookSecretRepository{db: db}
}

// Get retrieves the webhook secret for a project by repo URL.
func (r *SQLiteWebhookSecretRepository) Get(ctx context.Context, repoURL string) (string, error) {
	var secret string
	err := r.db.QueryRowContext(ctx,
		"SELECT secret FROM webhook_secrets WHERE repo_url = ?", repoURL).Scan(&secret)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get webhook secret: %w", err)
	}
	return secret, nil
}

// Set stores a webhook secret for a project.
func (r *SQLiteWebhookSecretRepository) Set(ctx context.Context, repoURL, secret string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO webhook_secrets (repo_url, secret) VALUES (?, ?)",
		repoURL, secret)
	if err != nil {
		return fmt.Errorf("set webhook secret: %w", err)
	}
	return nil
}

// Delete removes a webhook secret.
func (r *SQLiteWebhookSecretRepository) Delete(ctx context.Context, repoURL string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM webhook_secrets WHERE repo_url = ?", repoURL)
	if err != nil {
		return fmt.Errorf("delete webhook secret: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
