package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/decko/flux/internal/model"
)

// SQLiteProjectRepository implements ProjectRepository using a SQLite database.
// JSON-serializable fields (Definition, Adapters, Pipelines) are stored as
// TEXT columns and marshaled/unmarshaled on reads and writes.
//
// Transactions are not used for single-statement CRUD operations.
// If multi-statement atomicity is needed in the future, transactional
// wrappers (e.g., CreateBatch) will be added to the ProjectRepository
// interface with a separate issue.
type SQLiteProjectRepository struct {
	db *sql.DB
}

// NewSQLiteProjectRepository creates a new SQLiteProjectRepository backed by
// the given *sql.DB connection.
//
// The caller is responsible for configuring the *sql.DB via ConfigureSQLiteDB
// before calling this constructor. NewSQLiteProjectRepository does not mutate
// the connection pool — it only holds a reference to the already-configured
// database handle.
//
// The caller must also ensure the "sqlite3" driver is imported:
//
//	import _ "modernc.org/sqlite"
func NewSQLiteProjectRepository(db *sql.DB) *SQLiteProjectRepository {
	return &SQLiteProjectRepository{db: db}
}

// Migrate creates the projects table if it does not already exist.
// SQLite journal mode and connection pool settings are managed by
// ConfigureSQLiteDB (called once at application startup), not here.
func (r *SQLiteProjectRepository) Migrate(ctx context.Context) error {
	query := `CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		repo_url TEXT NOT NULL,
		definition TEXT NOT NULL DEFAULT '{}',
		adapters TEXT NOT NULL DEFAULT '[]',
		pipelines TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`
	if _, err := r.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("creating projects table: %w", err)
	}
	return nil
}

// Create persists a new project. All time.Time values are normalized to UTC
// before storage. Returns an error if a project with the same ID already
// exists (SQLite UNIQUE constraint violation).
func (r *SQLiteProjectRepository) Create(ctx context.Context, project model.Project) error {
	def, err := json.Marshal(project.Definition)
	if err != nil {
		return fmt.Errorf("marshaling definition: %w", err)
	}
	adapters, err := json.Marshal(project.Adapters)
	if err != nil {
		return fmt.Errorf("marshaling adapters: %w", err)
	}
	pipelines, err := json.Marshal(project.Pipelines)
	if err != nil {
		return fmt.Errorf("marshaling pipelines: %w", err)
	}

	query := `INSERT INTO projects (id, name, repo_url, definition, adapters, pipelines, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = r.db.ExecContext(ctx, query,
		project.ID,
		project.Name,
		project.RepoURL,
		string(def),
		string(adapters),
		string(pipelines),
		project.CreatedAt.UTC(),
		project.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("creating project: %w", err)
	}
	return nil
}

// Get retrieves a project by ID. Returns ErrNotFound if no project with the
// given ID exists.
func (r *SQLiteProjectRepository) Get(ctx context.Context, id string) (model.Project, error) {
	query := `SELECT id, name, repo_url, definition, adapters, pipelines, created_at, updated_at FROM projects WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var project model.Project
	var def, adapters, pipelines string
	err := row.Scan(
		&project.ID,
		&project.Name,
		&project.RepoURL,
		&def,
		&adapters,
		&pipelines,
		&project.CreatedAt,
		&project.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Project{}, ErrNotFound
	}
	if err != nil {
		return model.Project{}, fmt.Errorf("getting project: %w", err)
	}

	if err := json.Unmarshal([]byte(def), &project.Definition); err != nil {
		return model.Project{}, fmt.Errorf("unmarshaling definition: %w", err)
	}
	if err := json.Unmarshal([]byte(adapters), &project.Adapters); err != nil {
		return model.Project{}, fmt.Errorf("unmarshaling adapters: %w", err)
	}
	if err := json.Unmarshal([]byte(pipelines), &project.Pipelines); err != nil {
		return model.Project{}, fmt.Errorf("unmarshaling pipelines: %w", err)
	}

	return project, nil
}

// List returns all projects matching the given filter criteria.
// Since ProjectFilter is currently empty, this returns all projects.
// Returns an empty non-nil slice when no projects exist.
func (r *SQLiteProjectRepository) List(ctx context.Context, _ ProjectFilter) ([]model.Project, error) {
	query := `SELECT id, name, repo_url, definition, adapters, pipelines, created_at, updated_at FROM projects`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	projects := make([]model.Project, 0)
	for rows.Next() {
		var project model.Project
		var def, adapters, pipelines string
		if err := rows.Scan(
			&project.ID,
			&project.Name,
			&project.RepoURL,
			&def,
			&adapters,
			&pipelines,
			&project.CreatedAt,
			&project.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning project row: %w", err)
		}

		if err := json.Unmarshal([]byte(def), &project.Definition); err != nil {
			return nil, fmt.Errorf("unmarshaling definition: %w", err)
		}
		if err := json.Unmarshal([]byte(adapters), &project.Adapters); err != nil {
			return nil, fmt.Errorf("unmarshaling adapters: %w", err)
		}
		if err := json.Unmarshal([]byte(pipelines), &project.Pipelines); err != nil {
			return nil, fmt.Errorf("unmarshaling pipelines: %w", err)
		}

		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating project rows: %w", err)
	}

	return projects, nil
}

// Update modifies an existing project. All time.Time values are normalized to
// UTC before storage. Returns ErrNotFound if no project with the given ID
// exists.
func (r *SQLiteProjectRepository) Update(ctx context.Context, project model.Project) error {
	def, err := json.Marshal(project.Definition)
	if err != nil {
		return fmt.Errorf("marshaling definition: %w", err)
	}
	adapters, err := json.Marshal(project.Adapters)
	if err != nil {
		return fmt.Errorf("marshaling adapters: %w", err)
	}
	pipelines, err := json.Marshal(project.Pipelines)
	if err != nil {
		return fmt.Errorf("marshaling pipelines: %w", err)
	}

	query := `UPDATE projects SET name = ?, repo_url = ?, definition = ?, adapters = ?, pipelines = ?, updated_at = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query,
		project.Name,
		project.RepoURL,
		string(def),
		string(adapters),
		string(pipelines),
		project.UpdatedAt.UTC(),
		project.ID,
	)
	if err != nil {
		return fmt.Errorf("updating project: %w", err)
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

// Delete removes a project by ID. Returns ErrNotFound if no project with the
// given ID exists.
func (r *SQLiteProjectRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM projects WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting project: %w", err)
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
