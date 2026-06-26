package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/decko/flux/internal/model"
)

// SQLiteTriggerRuleRepository implements TriggerRuleRepository using a SQLite
// database. The enabled field is stored as INTEGER (0/1) and converted to/from
// bool on reads and writes.
type SQLiteTriggerRuleRepository struct {
	db *sqlx.DB
}

// NewSQLiteTriggerRuleRepository creates a new SQLiteTriggerRuleRepository
// backed by the given *sqlx.DB connection.
//
// The caller is responsible for configuring the underlying *sql.DB via
// ConfigureSQLiteDB before wrapping it with sqlx.NewDb.
func NewSQLiteTriggerRuleRepository(db *sqlx.DB) *SQLiteTriggerRuleRepository {
	return &SQLiteTriggerRuleRepository{db: db}
}

// Create persists a new trigger rule. All time.Time values are normalized to
// UTC before storage. Returns an error if a rule with the same ID already
// exists (SQLite UNIQUE constraint violation).
func (r *SQLiteTriggerRuleRepository) Create(ctx context.Context, rule model.TriggerRule) error {
	enabled := 0
	if rule.Enabled {
		enabled = 1
	}

	query := `INSERT INTO trigger_rules (id, project_id, label, pipeline, enabled, priority, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		rule.ID,
		rule.ProjectID,
		rule.Label,
		rule.Pipeline,
		enabled,
		rule.Priority,
		rule.CreatedAt.UTC(),
		rule.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("creating trigger rule: %w", err)
	}
	return nil
}

// ListByProject returns all trigger rules for a given project, ordered by
// priority descending. Enabled status is converted from INTEGER to bool.
// Returns an empty non-nil slice when no rules exist.
func (r *SQLiteTriggerRuleRepository) ListByProject(ctx context.Context, projectID string) ([]model.TriggerRule, error) {
	query := `SELECT id, project_id, label, pipeline, enabled, priority, created_at, updated_at FROM trigger_rules WHERE project_id = ? ORDER BY priority DESC`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing trigger rules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	rules := make([]model.TriggerRule, 0)
	for rows.Next() {
		var rule model.TriggerRule
		var enabled int
		if err := rows.Scan(
			&rule.ID,
			&rule.ProjectID,
			&rule.Label,
			&rule.Pipeline,
			&enabled,
			&rule.Priority,
			&rule.CreatedAt,
			&rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning trigger rule row: %w", err)
		}
		rule.Enabled = enabled != 0
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating trigger rule rows: %w", err)
	}
	return rules, nil
}

// Update modifies an existing trigger rule. All time.Time values are
// normalized to UTC before storage. Returns ErrNotFound if no rule with the
// given ID exists.
func (r *SQLiteTriggerRuleRepository) Update(ctx context.Context, rule model.TriggerRule) error {
	enabled := 0
	if rule.Enabled {
		enabled = 1
	}

	query := `UPDATE trigger_rules SET project_id = ?, label = ?, pipeline = ?, enabled = ?, priority = ?, updated_at = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query,
		rule.ProjectID,
		rule.Label,
		rule.Pipeline,
		enabled,
		rule.Priority,
		rule.UpdatedAt.UTC(),
		rule.ID,
	)
	if err != nil {
		return fmt.Errorf("updating trigger rule: %w", err)
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

// Delete removes a trigger rule by ID. Returns ErrNotFound if no rule with
// the given ID exists.
func (r *SQLiteTriggerRuleRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM trigger_rules WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting trigger rule: %w", err)
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
var _ TriggerRuleRepository = (*SQLiteTriggerRuleRepository)(nil)
