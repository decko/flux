package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/decko/flux/internal/model"
)

// SQLiteTicketRepository implements TicketRepository using a SQLite database.
// JSON-serializable fields (Labels, Relationships, PRs) are stored as TEXT
// columns and marshaled/unmarshaled on reads and writes.
//
// Transactions are not used for single-statement CRUD operations.
// If multi-statement atomicity is needed in the future, transactional
// wrappers (e.g., CreateBatch) will be added to the TicketRepository
// interface with a separate issue.
type SQLiteTicketRepository struct {
	db *sql.DB
}

// NewSQLiteTicketRepository creates a new SQLiteTicketRepository backed by
// the given *sql.DB connection.
//
// The caller is responsible for configuring the *sql.DB via ConfigureSQLiteDB
// before calling this constructor. NewSQLiteTicketRepository does not mutate
// the connection pool — it only holds a reference to the already-configured
// database handle.
//
// The caller must also ensure the "sqlite3" driver is imported:
//
//	import _ "modernc.org/sqlite"
func NewSQLiteTicketRepository(db *sql.DB) *SQLiteTicketRepository {
	return &SQLiteTicketRepository{db: db}
}

// Migrate creates the tickets table if it does not already exist.
// SQLite journal mode and connection pool settings are managed by
// ConfigureSQLiteDB (called once at application startup), not here.
func (r *SQLiteTicketRepository) Migrate(ctx context.Context) error {
	query := `CREATE TABLE IF NOT EXISTS tickets (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		external_id TEXT NOT NULL,
		source TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		labels TEXT NOT NULL DEFAULT '[]',
		relationships TEXT NOT NULL DEFAULT '[]',
		prs TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`
	if _, err := r.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("creating tickets table: %w", err)
	}
	return nil
}

// Create persists a new ticket. All time.Time values are normalized to UTC
// before storage. Returns an error if a ticket with the same ID already
// exists (SQLite UNIQUE constraint violation).
func (r *SQLiteTicketRepository) Create(ctx context.Context, ticket model.Ticket) error {
	labels, err := json.Marshal(ticket.Labels)
	if err != nil {
		return fmt.Errorf("marshaling labels: %w", err)
	}
	relationships, err := json.Marshal(ticket.Relationships)
	if err != nil {
		return fmt.Errorf("marshaling relationships: %w", err)
	}
	prs, err := json.Marshal(ticket.PRs)
	if err != nil {
		return fmt.Errorf("marshaling prs: %w", err)
	}

	query := `INSERT INTO tickets (id, project_id, external_id, source, title, description, status, labels, relationships, prs, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = r.db.ExecContext(ctx, query,
		ticket.ID,
		ticket.ProjectID,
		ticket.ExternalID,
		ticket.Source,
		ticket.Title,
		ticket.Description,
		ticket.Status,
		string(labels),
		string(relationships),
		string(prs),
		ticket.CreatedAt.UTC(),
		ticket.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("creating ticket: %w", err)
	}
	return nil
}

// Get retrieves a ticket by ID. Returns ErrNotFound if no ticket with the
// given ID exists.
func (r *SQLiteTicketRepository) Get(ctx context.Context, id string) (model.Ticket, error) {
	query := `SELECT id, project_id, external_id, source, title, description, status, labels, relationships, prs, created_at, updated_at FROM tickets WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var ticket model.Ticket
	var labels, relationships, prs string
	err := row.Scan(
		&ticket.ID,
		&ticket.ProjectID,
		&ticket.ExternalID,
		&ticket.Source,
		&ticket.Title,
		&ticket.Description,
		&ticket.Status,
		&labels,
		&relationships,
		&prs,
		&ticket.CreatedAt,
		&ticket.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, ErrNotFound
	}
	if err != nil {
		return model.Ticket{}, fmt.Errorf("getting ticket: %w", err)
	}

	if err := json.Unmarshal([]byte(labels), &ticket.Labels); err != nil {
		return model.Ticket{}, fmt.Errorf("unmarshaling labels: %w", err)
	}
	if err := json.Unmarshal([]byte(relationships), &ticket.Relationships); err != nil {
		return model.Ticket{}, fmt.Errorf("unmarshaling relationships: %w", err)
	}
	if err := json.Unmarshal([]byte(prs), &ticket.PRs); err != nil {
		return model.Ticket{}, fmt.Errorf("unmarshaling prs: %w", err)
	}

	return ticket, nil
}

// List returns all tickets matching the given filter criteria.
// Zero values in the filter are ignored. Labels are matched with OR
// semantics — a ticket is included if it has any of the requested labels.
// Returns an empty non-nil slice when no tickets exist.
func (r *SQLiteTicketRepository) List(ctx context.Context, filter TicketFilter) ([]model.Ticket, error) {
	query := `SELECT id, project_id, external_id, source, title, description, status, labels, relationships, prs, created_at, updated_at FROM tickets`

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
	if filter.Source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, filter.Source)
	}
	if len(filter.Labels) > 0 {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM json_each(tickets.labels) WHERE value IN ("+placeholders(len(filter.Labels))+"))")
		for _, l := range filter.Labels {
			args = append(args, l)
		}
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing tickets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tickets := make([]model.Ticket, 0)
	for rows.Next() {
		var ticket model.Ticket
		var labels, relationships, prs string
		if err := rows.Scan(
			&ticket.ID,
			&ticket.ProjectID,
			&ticket.ExternalID,
			&ticket.Source,
			&ticket.Title,
			&ticket.Description,
			&ticket.Status,
			&labels,
			&relationships,
			&prs,
			&ticket.CreatedAt,
			&ticket.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning ticket row: %w", err)
		}

		if err := json.Unmarshal([]byte(labels), &ticket.Labels); err != nil {
			return nil, fmt.Errorf("unmarshaling labels: %w", err)
		}
		if err := json.Unmarshal([]byte(relationships), &ticket.Relationships); err != nil {
			return nil, fmt.Errorf("unmarshaling relationships: %w", err)
		}
		if err := json.Unmarshal([]byte(prs), &ticket.PRs); err != nil {
			return nil, fmt.Errorf("unmarshaling prs: %w", err)
		}

		tickets = append(tickets, ticket)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating ticket rows: %w", err)
	}

	return tickets, nil
}

// Update modifies an existing ticket. All time.Time values are normalized to
// UTC before storage. Returns ErrNotFound if no ticket with the given ID
// exists.
func (r *SQLiteTicketRepository) Update(ctx context.Context, ticket model.Ticket) error {
	labels, err := json.Marshal(ticket.Labels)
	if err != nil {
		return fmt.Errorf("marshaling labels: %w", err)
	}
	relationships, err := json.Marshal(ticket.Relationships)
	if err != nil {
		return fmt.Errorf("marshaling relationships: %w", err)
	}
	prs, err := json.Marshal(ticket.PRs)
	if err != nil {
		return fmt.Errorf("marshaling prs: %w", err)
	}

	query := `UPDATE tickets SET project_id = ?, external_id = ?, source = ?, title = ?, description = ?, status = ?, labels = ?, relationships = ?, prs = ?, updated_at = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query,
		ticket.ProjectID,
		ticket.ExternalID,
		ticket.Source,
		ticket.Title,
		ticket.Description,
		ticket.Status,
		string(labels),
		string(relationships),
		string(prs),
		ticket.UpdatedAt.UTC(),
		ticket.ID,
	)
	if err != nil {
		return fmt.Errorf("updating ticket: %w", err)
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

// Delete removes a ticket by ID. Returns ErrNotFound if no ticket with the
// given ID exists.
func (r *SQLiteTicketRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM tickets WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting ticket: %w", err)
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

// placeholders generates a comma-separated list of SQL placeholder (?)
// characters for use in IN clauses. Returns an empty string for n == 0.
func placeholders(n int) string {
	if n == 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ", ")
}
