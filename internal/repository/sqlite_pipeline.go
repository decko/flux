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

// SQLitePipelineRunRepository implements PipelineRunRepository using a SQLite
// database. JSON-serializable fields (Phases, Cost) are stored as TEXT columns
// and marshaled/unmarshaled on reads and writes.
//
// Pipeline runs are immutable audit records — there is no Delete method.
type SQLitePipelineRunRepository struct {
	db *sqlx.DB
}

// NewSQLitePipelineRunRepository creates a new SQLitePipelineRunRepository
// backed by the given *sqlx.DB connection.
//
// The caller is responsible for configuring the underlying *sql.DB via
// ConfigureSQLiteDB before wrapping it with sqlx.NewDb.
func NewSQLitePipelineRunRepository(db *sqlx.DB) *SQLitePipelineRunRepository {
	return &SQLitePipelineRunRepository{db: db}
}

// Create persists a new pipeline run. time.Time values are normalized to UTC
// before storage. The Phases field is JSON-marshaled; Cost is marshaled when
// non-nil and stored as SQL NULL when nil. Returns an error if a run with the
// same ID already exists (SQLite UNIQUE constraint violation).
func (r *SQLitePipelineRunRepository) Create(ctx context.Context, run model.PipelineRun) error {
	phases, err := json.Marshal(run.Phases)
	if err != nil {
		return fmt.Errorf("marshaling phases: %w", err)
	}

	var costJSON *string
	if run.Cost != nil {
		b, err := json.Marshal(run.Cost)
		if err != nil {
			return fmt.Errorf("marshaling cost: %w", err)
		}
		s := string(b)
		costJSON = &s
	}

	var completedAt interface{}
	if run.CompletedAt != nil {
		utc := run.CompletedAt.UTC()
		completedAt = utc
	}

	query := `INSERT INTO pipeline_runs (id, project_id, ticket_id, orchestrator, pipeline, status, phases, started_at, completed_at, cost) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = r.db.ExecContext(ctx, query,
		run.ID,
		run.ProjectID,
		run.TicketID,
		run.Orchestrator,
		run.Pipeline,
		run.Status,
		string(phases),
		run.StartedAt.UTC(),
		completedAt,
		costJSON,
	)
	if err != nil {
		return fmt.Errorf("creating pipeline run: %w", err)
	}
	return nil
}

// Get retrieves a pipeline run by ID. Returns ErrNotFound if no run with the
// given ID exists. JSON fields (Phases, Cost) are unmarshaled on read.
func (r *SQLitePipelineRunRepository) Get(ctx context.Context, id string) (model.PipelineRun, error) {
	query := `SELECT id, project_id, ticket_id, orchestrator, pipeline, status, phases, started_at, completed_at, cost FROM pipeline_runs WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var run model.PipelineRun
	var phases string
	var completedAt sql.NullTime
	var costStr *string

	err := row.Scan(
		&run.ID,
		&run.ProjectID,
		&run.TicketID,
		&run.Orchestrator,
		&run.Pipeline,
		&run.Status,
		&phases,
		&run.StartedAt,
		&completedAt,
		&costStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PipelineRun{}, ErrNotFound
	}
	if err != nil {
		return model.PipelineRun{}, fmt.Errorf("getting pipeline run: %w", err)
	}

	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}

	if costStr != nil {
		var cost model.CostBreakdown
		if err := json.Unmarshal([]byte(*costStr), &cost); err != nil {
			return model.PipelineRun{}, fmt.Errorf("unmarshaling cost: %w", err)
		}
		run.Cost = &cost
	}

	if err := json.Unmarshal([]byte(phases), &run.Phases); err != nil {
		return model.PipelineRun{}, fmt.Errorf("unmarshaling phases: %w", err)
	}

	return run, nil
}

// List returns all pipeline runs matching the given filter criteria.
// Zero values in the filter are ignored. Returns an empty non-nil slice when
// no pipeline runs exist.
func (r *SQLitePipelineRunRepository) List(ctx context.Context, filter PipelineRunFilter) ([]model.PipelineRun, error) {
	query := `SELECT id, project_id, ticket_id, orchestrator, pipeline, status, phases, started_at, completed_at, cost FROM pipeline_runs`

	var clauses []string
	var args []interface{}

	if filter.ProjectID != "" {
		clauses = append(clauses, "project_id = ?")
		args = append(args, filter.ProjectID)
	}
	if filter.TicketID != "" {
		clauses = append(clauses, "ticket_id = ?")
		args = append(args, filter.TicketID)
	}
	if filter.Status != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, filter.Status)
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing pipeline runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	runs := make([]model.PipelineRun, 0)
	for rows.Next() {
		var run model.PipelineRun
		var phases string
		var completedAt sql.NullTime
		var costStr *string

		if err := rows.Scan(
			&run.ID,
			&run.ProjectID,
			&run.TicketID,
			&run.Orchestrator,
			&run.Pipeline,
			&run.Status,
			&phases,
			&run.StartedAt,
			&completedAt,
			&costStr,
		); err != nil {
			return nil, fmt.Errorf("scanning pipeline run row: %w", err)
		}

		if completedAt.Valid {
			run.CompletedAt = &completedAt.Time
		}

		if costStr != nil {
			var cost model.CostBreakdown
			if err := json.Unmarshal([]byte(*costStr), &cost); err != nil {
				return nil, fmt.Errorf("unmarshaling cost: %w", err)
			}
			run.Cost = &cost
		}

		if err := json.Unmarshal([]byte(phases), &run.Phases); err != nil {
			return nil, fmt.Errorf("unmarshaling phases: %w", err)
		}

		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pipeline run rows: %w", err)
	}

	return runs, nil
}

// Update modifies an existing pipeline run. Only mutable fields are updated:
// status, phases, completed_at, and cost. The id, project_id, ticket_id,
// orchestrator, pipeline, and started_at fields are not changed. Returns
// ErrNotFound if no run with the given ID exists.
func (r *SQLitePipelineRunRepository) Update(ctx context.Context, run model.PipelineRun) error {
	phases, err := json.Marshal(run.Phases)
	if err != nil {
		return fmt.Errorf("marshaling phases: %w", err)
	}

	var costJSON *string
	if run.Cost != nil {
		b, err := json.Marshal(run.Cost)
		if err != nil {
			return fmt.Errorf("marshaling cost: %w", err)
		}
		s := string(b)
		costJSON = &s
	}

	var completedAt interface{}
	if run.CompletedAt != nil {
		utc := run.CompletedAt.UTC()
		completedAt = utc
	}

	query := `UPDATE pipeline_runs SET status = ?, phases = ?, completed_at = ?, cost = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query,
		run.Status,
		string(phases),
		completedAt,
		costJSON,
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("updating pipeline run: %w", err)
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
