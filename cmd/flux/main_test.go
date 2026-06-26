package main

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/jmoiron/sqlx"

	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/repository"
)

// setupTestDB creates an in-memory SQLite DB with migrations applied.
// Uses a shared cache so that other connections (e.g. from seedCmd) can
// access the same in-memory database.
func setupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return sqlx.NewDb(db, "sqlite")
}

// tempFile writes content to a temp file and returns its path.
func tempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "flux-test-")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	_ = f.Close()
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	return f.Name()
}

// setArgs temporarily sets os.Args and restores them after the test.
func setArgs(args ...string) func() {
	old := os.Args
	os.Args = append([]string{"flux"}, args...)
	return func() { os.Args = old }
}

func TestSeedCommand_CreatesAdmin(t *testing.T) {
	sdb := setupTestDB(t)
	pwFile := tempFile(t, "new-password-123")

	defer setArgs("seed", "--email", "admin@flux.dev", "--password-file", pwFile)()

	cfgFile := tempFile(t, "database:\n  path: file::memory:?cache=shared")
	t.Setenv("FLUX_CONFIG", cfgFile)

	err := seedCmd()
	if err != nil {
		t.Fatalf("seedCmd failed: %v", err)
	}

	userRepo := repository.NewSQLiteUserRepository(sdb)
	user, err := userRepo.GetByEmail(t.Context(), "admin@flux.dev")
	if err != nil {
		t.Fatalf("admin should exist after seed: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("role = %q, want %q", user.Role, "admin")
	}
}

func TestSeedCommand_MissingEmail(t *testing.T) {
	setupTestDB(t)
	pwFile := tempFile(t, "password")
	defer setArgs("seed", "--password-file", pwFile)()

	err := seedCmd()
	if err == nil {
		t.Error("expected error for missing --email, got nil")
	}
}

func TestSeedCommand_Idempotent(t *testing.T) {
	sdb := setupTestDB(t)
	pwFile := tempFile(t, "password-123")

	cfgFile := tempFile(t, "database:\n  path: file::memory:?cache=shared")
	t.Setenv("FLUX_CONFIG", cfgFile)

	defer setArgs("seed", "--email", "admin@flux.dev", "--password-file", pwFile)()
	if err := seedCmd(); err != nil {
		t.Fatalf("first seed: %v", err)
	}

	// Second run should succeed (idempotent).
	err := seedCmd()
	if err != nil {
		t.Errorf("second seed should be idempotent, got: %v", err)
	}

	// User should still exist.
	userRepo := repository.NewSQLiteUserRepository(sdb)
	_, err = userRepo.GetByEmail(t.Context(), "admin@flux.dev")
	if err != nil {
		t.Errorf("admin should still exist: %v", err)
	}
}

func TestUserSetPassword_ChangesHash(t *testing.T) {
	sdb := setupTestDB(t)
	pwFile1 := tempFile(t, "old-password")
	pwFile2 := tempFile(t, "new-password-456")

	cfgFile := tempFile(t, "database:\n  path: file::memory:?cache=shared")
	t.Setenv("FLUX_CONFIG", cfgFile)

	// First, seed the admin.
	defer setArgs("seed", "--email", "admin@flux.dev", "--password-file", pwFile1)()
	if err := seedCmd(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Get the original hash.
	userRepo := repository.NewSQLiteUserRepository(sdb)
	user1, err := userRepo.GetByEmail(t.Context(), "admin@flux.dev")
	if err != nil {
		t.Fatalf("get user after seed: %v", err)
	}

	// Change password.
	defer setArgs("user", "set-password", "--email", "admin@flux.dev", "--password-file", pwFile2)()
	if err := userCmd(); err != nil {
		t.Fatalf("set-password: %v", err)
	}

	// Verify hash changed.
	user2, err := userRepo.GetByEmail(t.Context(), "admin@flux.dev")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user1.PasswordHash == user2.PasswordHash {
		t.Error("password hash should have changed")
	}
}

func TestUserSetPassword_UserNotFound(t *testing.T) {
	setupTestDB(t)
	pwFile := tempFile(t, "password")

	defer setArgs("user", "set-password", "--email", "nobody@flux.dev", "--password-file", pwFile)()

	err := userCmd()
	if err == nil {
		t.Error("expected error for non-existent user, got nil")
	}
}

func TestUserAdd_CreatesUser(t *testing.T) {
	sdb := setupTestDB(t)
	pwFile := tempFile(t, "password-123456")
	cfgFile := tempFile(t, "database:\n  path: file::memory:?cache=shared")

	defer setArgs("user", "add", "--email", "test@flux.dev", "--password-file", pwFile)()
	t.Setenv("FLUX_CONFIG", cfgFile)

	err := userCmd()
	if err != nil {
		t.Fatalf("user add: %v", err)
	}

	userRepo := repository.NewSQLiteUserRepository(sdb)
	user, err := userRepo.GetByEmail(t.Context(), "test@flux.dev")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.Role != "user" {
		t.Errorf("role = %q, want %q", user.Role, "user")
	}
}

func TestUserAdd_CreatesAdmin(t *testing.T) {
	sdb := setupTestDB(t)
	pwFile := tempFile(t, "password-123456")
	cfgFile := tempFile(t, "database:\n  path: file::memory:?cache=shared")

	defer setArgs("user", "add", "--email", "admin2@flux.dev", "--password-file", pwFile, "--role", "admin")()
	t.Setenv("FLUX_CONFIG", cfgFile)

	err := userCmd()
	if err != nil {
		t.Fatalf("user add: %v", err)
	}

	userRepo := repository.NewSQLiteUserRepository(sdb)
	user, err := userRepo.GetByEmail(t.Context(), "admin2@flux.dev")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if user.Role != "admin" {
		t.Errorf("role = %q, want %q", user.Role, "admin")
	}
}

func TestUserAdd_MissingEmail(t *testing.T) {
	setupTestDB(t)
	pwFile := tempFile(t, "password-123456")
	cfgFile := tempFile(t, "database:\n  path: file::memory:?cache=shared")

	defer setArgs("user", "add", "--password-file", pwFile)()
	t.Setenv("FLUX_CONFIG", cfgFile)

	err := userCmd()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--email is required") {
		t.Errorf("error = %q, want substring %q", err.Error(), "--email is required")
	}
}

func TestUserAdd_DuplicateEmail(t *testing.T) {
	setupTestDB(t)
	pwFile := tempFile(t, "password-123456")
	cfgFile := tempFile(t, "database:\n  path: file::memory:?cache=shared")

	defer setArgs("user", "add", "--email", "dup@flux.dev", "--password-file", pwFile)()
	t.Setenv("FLUX_CONFIG", cfgFile)

	// First creation should succeed.
	err := userCmd()
	if err != nil {
		t.Fatalf("first user add: %v", err)
	}

	// Second creation with same email should fail.
	err = userCmd()
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want substring %q", err.Error(), "already exists")
	}
}

func TestUserAdd_InvalidRole(t *testing.T) {
	setupTestDB(t)
	pwFile := tempFile(t, "password-123456")
	cfgFile := tempFile(t, "database:\n  path: file::memory:?cache=shared")

	defer setArgs("user", "add", "--email", "badrole@flux.dev", "--password-file", pwFile, "--role", "superadmin")()
	t.Setenv("FLUX_CONFIG", cfgFile)

	err := userCmd()
	if err == nil {
		t.Fatal("expected error for invalid role, got nil")
	}
	if !strings.Contains(err.Error(), "invalid role") {
		t.Errorf("error = %q, want substring %q", err.Error(), "invalid role")
	}
}

func TestUserAdd_ShortPassword(t *testing.T) {
	setupTestDB(t)
	pwFile := tempFile(t, "short")
	cfgFile := tempFile(t, "database:\n  path: file::memory:?cache=shared")

	defer setArgs("user", "add", "--email", "shortpw@flux.dev", "--password-file", pwFile)()
	t.Setenv("FLUX_CONFIG", cfgFile)

	err := userCmd()
	if err == nil {
		t.Fatal("expected error for short password, got nil")
	}
	if !strings.Contains(err.Error(), "password must be at least 12 characters") {
		t.Errorf("error = %q, want substring %q", err.Error(), "password must be at least 12 characters")
	}
}
