package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/decko/flux/internal/model"
)

// SQLitePullRequestRepository implements PullRequestRepository using a SQLite
// database. JSON-serializable fields (TicketIDs, Reviews) are stored as TEXT
// columns and marshaled/unmarshaled on reads and writes.
//
// Transactions are not used for single-statement CRUD operations.
// If multi-statement atomicity is needed in the future, transactional
// wrappers (e.g., CreateBatch) will be added to the PullRequestRepository
// interface with a separate issue.
type SQLitePullRequestRepository struct {
	db *sqlx.DB
}

// NewSQLitePullRequestRepository creates a new SQLitePullRequestRepository
// backed by the given *sqlx.DB connection.
//
// The caller is responsible for configuring the underlying *sql.DB via
// ConfigureSQLiteDB before wrapping it with sqlx.NewDb.
func NewSQLitePullRequestRepository(db *sqlx.DB) *SQLitePullRequestRepository {
	return &SQLitePullRequestRepository{db: db}
}

// Create persists a new pull request. All time.Time values are normalized to
// UTC before storage. Returns an error if a pull request with the same ID
// already exists (SQLite UNIQUE constraint violation).
func (r *SQLitePullRequestRepository) Create(ctx context.Context, pr model.PullRequest) error {
	ticketIDs, err := json.Marshal(pr.TicketIDs)
	if err != nil {
		return fmt.Errorf("marshaling ticket_ids: %w", err)
	}
	reviews, err := json.Marshal(pr.Reviews)
	if err != nil {
		return fmt.Errorf("marshaling reviews: %w", err)
	}

	query := `INSERT INTO pull_requests (id, project_id, external_id, source, title, url, status, ticket_ids, reviews, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = r.db.ExecContext(ctx, query,
		pr.ID,
		pr.ProjectID,
		pr.ExternalID,
		pr.Source,
		pr.Title,
		pr.URL,
		pr.Status,
		string(ticketIDs),
		string(reviews),
		pr.CreatedAt.UTC(),
		pr.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("creating pull request: %w", err)
	}
	return nil
}

// Get retrieves a pull request by ID. Returns ErrNotFound if no pull request
// with the given ID exists.
func (r *SQLitePullRequestRepository) Get(ctx context.Context, id string) (model.PullRequest, error) {
	query := `SELECT id, project_id, external_id, source, title, url, status, ticket_ids, reviews, created_at, updated_at FROM pull_requests WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var pr model.PullRequest
	var ticketIDs, reviews string
	err := row.Scan(
		&pr.ID,
		&pr.ProjectID,
		&pr.ExternalID,
		&pr.Source,
		&pr.Title,
		&pr.URL,
		&pr.Status,
		&ticketIDs,
		&reviews,
		&pr.CreatedAt,
		&pr.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PullRequest{}, ErrNotFound
	}
	if err != nil {
		return model.PullRequest{}, fmt.Errorf("getting pull request: %w", err)
	}

	if err := json.Unmarshal([]byte(ticketIDs), &pr.TicketIDs); err != nil {
		return model.PullRequest{}, fmt.Errorf("unmarshaling ticket_ids: %w", err)
	}
	if err := json.Unmarshal([]byte(reviews), &pr.Reviews); err != nil {
		return model.PullRequest{}, fmt.Errorf("unmarshaling reviews: %w", err)
	}

	return pr, nil
}

// List returns all pull requests matching the given filter criteria.
// Zero values in the filter are ignored. Returns an empty non-nil slice when
// no pull requests exist.
func (r *SQLitePullRequestRepository) List(ctx context.Context, filter PullRequestFilter) ([]model.PullRequest, error) {
	query := `SELECT id, project_id, external_id, source, title, url, status, ticket_ids, reviews, created_at, updated_at FROM pull_requests`

	var clauses []string
	var args []interface{}

	if filter.ProjectID != "" {
		clauses = append(clauses, "project_id = ?")
		args = append(args, filter.ProjectID)
	}
	if filter.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.TicketID != "" {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM json_each(pull_requests.ticket_ids) WHERE value = ?)")
		args = append(args, filter.TicketID)
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing pull requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	prs := make([]model.PullRequest, 0)
	for rows.Next() {
		var pr model.PullRequest
		var ticketIDs, reviews string
		if err := rows.Scan(
			&pr.ID,
			&pr.ProjectID,
			&pr.ExternalID,
			&pr.Source,
			&pr.Title,
			&pr.URL,
			&pr.Status,
			&ticketIDs,
			&reviews,
			&pr.CreatedAt,
			&pr.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning pull request row: %w", err)
		}

		if err := json.Unmarshal([]byte(ticketIDs), &pr.TicketIDs); err != nil {
			return nil, fmt.Errorf("unmarshaling ticket_ids: %w", err)
		}
		if err := json.Unmarshal([]byte(reviews), &pr.Reviews); err != nil {
			return nil, fmt.Errorf("unmarshaling reviews: %w", err)
		}

		prs = append(prs, pr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pull request rows: %w", err)
	}

	return prs, nil
}

// Update modifies an existing pull request. All time.Time values are
// normalized to UTC before storage. Returns ErrNotFound if no pull request
// with the given ID exists.
func (r *SQLitePullRequestRepository) Update(ctx context.Context, pr model.PullRequest) error {
	ticketIDs, err := json.Marshal(pr.TicketIDs)
	if err != nil {
		return fmt.Errorf("marshaling ticket_ids: %w", err)
	}
	reviews, err := json.Marshal(pr.Reviews)
	if err != nil {
		return fmt.Errorf("marshaling reviews: %w", err)
	}

	query := `UPDATE pull_requests SET project_id = ?, external_id = ?, source = ?, title = ?, url = ?, status = ?, ticket_ids = ?, reviews = ?, updated_at = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query,
		pr.ProjectID,
		pr.ExternalID,
		pr.Source,
		pr.Title,
		pr.URL,
		pr.Status,
		string(ticketIDs),
		string(reviews),
		pr.UpdatedAt.UTC(),
		pr.ID,
	)
	if err != nil {
		return fmt.Errorf("updating pull request: %w", err)
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

// Delete removes a pull request by ID. Returns ErrNotFound if no pull request
// with the given ID exists.
func (r *SQLitePullRequestRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM pull_requests WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting pull request: %w", err)
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
