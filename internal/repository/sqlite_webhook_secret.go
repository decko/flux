package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// SQLiteWebhookSecretRepository implements WebhookSecretRepository using a
// SQLite database.
type SQLiteWebhookSecretRepository struct {
	db *sqlx.DB
}

// NewSQLiteWebhookSecretRepository creates a new SQLiteWebhookSecretRepository
// backed by the given *sqlx.DB connection.
func NewSQLiteWebhookSecretRepository(db *sqlx.DB) *SQLiteWebhookSecretRepository {
	return &SQLiteWebhookSecretRepository{db: db}
}

// Get retrieves the webhook secret for the given project ID.
// Returns ErrNotFound if no secret exists for the project.
func (r *SQLiteWebhookSecretRepository) Get(ctx context.Context, projectID string) (string, error) {
	query := `SELECT secret FROM webhook_secrets WHERE project_id = ?`
	var secret string
	err := r.db.GetContext(ctx, &secret, query, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get webhook secret: %w", err)
	}
	return secret, nil
}

// Set stores or updates the webhook secret for the given project ID.
func (r *SQLiteWebhookSecretRepository) Set(ctx context.Context, projectID, secret string) error {
	query := `INSERT INTO webhook_secrets (project_id, secret, created_at) VALUES (?, ?, ?)
		ON CONFLICT(project_id) DO UPDATE SET secret = excluded.secret`
	_, err := r.db.ExecContext(ctx, query, projectID, secret, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("set webhook secret: %w", err)
	}
	return nil
}

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
