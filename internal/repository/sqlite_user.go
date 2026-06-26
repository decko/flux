package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/decko/flux/internal/model"
)

// SQLiteUserRepository implements UserRepository using a SQLite database.
// The users table stores user credentials and metadata. Email addresses
// have a UNIQUE constraint to prevent duplicates.
type SQLiteUserRepository struct {
	db *sqlx.DB
}

// NewSQLiteUserRepository creates a new SQLiteUserRepository backed by
// the given *sqlx.DB connection.
//
// The caller is responsible for configuring the underlying *sql.DB via
// ConfigureSQLiteDB before wrapping it with sqlx.NewDb.
func NewSQLiteUserRepository(db *sqlx.DB) *SQLiteUserRepository {
	return &SQLiteUserRepository{db: db}
}

// Create persists a new user. Returns ErrDuplicateEmail if a user with the
// same email already exists.
func (r *SQLiteUserRepository) Create(ctx context.Context, user model.User) error {
	query := `INSERT INTO users (id, email, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.CreatedAt.UTC(),
	)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

// GetByEmail retrieves a user by email. Returns ErrNotFound if no user
// with the given email exists.
func (r *SQLiteUserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	query := `SELECT id, email, password_hash, role, created_at FROM users WHERE email = ?`
	row := r.db.QueryRowContext(ctx, query, email)

	var user model.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("getting user by email: %w", err)
	}
	return user, nil
}

// GetByID retrieves a user by ID. Returns ErrNotFound if no user
// with the given ID exists.
func (r *SQLiteUserRepository) GetByID(ctx context.Context, id string) (model.User, error) {
	query := `SELECT id, email, password_hash, role, created_at FROM users WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var user model.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("getting user by id: %w", err)
	}
	return user, nil
}

// Update modifies an existing user's mutable fields: email, password_hash,
// and role. Returns ErrNotFound if no user with the given ID exists.
func (r *SQLiteUserRepository) Update(ctx context.Context, user model.User) error {
	query := `UPDATE users SET email = ?, password_hash = ?, role = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query,
		user.Email,
		user.PasswordHash,
		user.Role,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
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

// List returns all users ordered by created_at ascending.
// Returns an empty slice if no users exist.
func (r *SQLiteUserRepository) List(ctx context.Context) ([]model.User, error) {
	query := `SELECT id, email, password_hash, role, created_at FROM users ORDER BY created_at`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []model.User
	for rows.Next() {
		var user model.User
		if err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.PasswordHash,
			&user.Role,
			&user.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating users: %w", err)
	}
	return users, nil
}

// Delete removes a user by ID. Returns ErrNotFound if no user
// with the given ID exists.
func (r *SQLiteUserRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
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

// Count returns the total number of users.
func (r *SQLiteUserRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`
	var count int
	if err := r.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}

// CountByRole returns the number of users with the given role.
func (r *SQLiteUserRepository) CountByRole(ctx context.Context, role string) (int, error) {
	query := `SELECT COUNT(*) FROM users WHERE role = ?`
	var count int
	if err := r.db.QueryRowContext(ctx, query, role).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting users by role: %w", err)
	}
	return count, nil
}

// isUniqueConstraintViolation checks if the error is a SQLite UNIQUE
// constraint violation (error code 19, constraint UNIQUE).
func isUniqueConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	// SQLite error format: "UNIQUE constraint failed: table.column"
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
