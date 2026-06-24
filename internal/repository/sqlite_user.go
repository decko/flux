package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/decko/flux/internal/model"
)

// SQLiteUserRepository implements UserRepository using a SQLite database.
// The users table stores user credentials and metadata. Email addresses
// have a UNIQUE constraint to prevent duplicates.
type SQLiteUserRepository struct {
	db *sql.DB
}

// NewSQLiteUserRepository creates a new SQLiteUserRepository backed by
// the given *sql.DB connection.
//
// The caller is responsible for configuring the *sql.DB via ConfigureSQLiteDB
// before calling this constructor.
//
// The caller must also ensure the "sqlite3" driver is imported:
//
//	import _ "modernc.org/sqlite"
func NewSQLiteUserRepository(db *sql.DB) *SQLiteUserRepository {
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

// isUniqueConstraintViolation checks if the error is a SQLite UNIQUE
// constraint violation (error code 19, constraint UNIQUE).
func isUniqueConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	// SQLite error format: "UNIQUE constraint failed: table.column"
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
