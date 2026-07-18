package main

import (
	"database/sql"
	"os"
	"os/exec"
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

// ─── jwtSecret tests ─────────────────────────────────────────────────────

// TestJWTSecret_FromEnv verifies that jwtSecret() returns the value of
// JWT_SECRET when it is set to a valid key (>=16 chars).
func TestJWTSecret_FromEnv(t *testing.T) {
	t.Setenv("JWT_SECRET", "my-strong-secret-key-with-32-chars!")
	secret := jwtSecret()
	if string(secret) != "my-strong-secret-key-with-32-chars!" {
		t.Errorf("got %q, want %q", string(secret), "my-strong-secret-key-with-32-chars!")
	}
}

// TestJWTSecret_NoFallback verifies that jwtSecret() never returns the
// hardcoded "dev-secret" fallback, even when JWT_SECRET is unset.
// The function should generate a random key and log a warning instead.
func TestJWTSecret_NoFallback(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	secret := jwtSecret()
	if string(secret) == "dev-secret" {
		t.Error("jwtSecret() returned hardcoded dev-secret fallback")
	}
}

// TestJWTSecret_MinLength verifies that jwtSecret() always returns a key
// of at least 16 bytes.
func TestJWTSecret_MinLength(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	secret := jwtSecret()
	if len(secret) < 16 {
		t.Errorf("jwtSecret() returned key of length %d, want at least 16", len(secret))
	}
}

// TestJWTSecret_RandomWhenEmpty verifies that two calls to jwtSecret()
// without JWT_SECRET produce different keys (a fresh random key each time).
func TestJWTSecret_RandomWhenEmpty(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	secret1 := jwtSecret()
	secret2 := jwtSecret()
	if len(secret1) == 0 || len(secret2) == 0 {
		t.Fatal("jwtSecret() returned empty key")
	}
	if string(secret1) == string(secret2) {
		t.Error("jwtSecret() returned same key on two calls — should generate random key each time")
	}
}

// TestJWTSecret_NOAUTH_NoBypass verifies that NO_AUTH=1 does NOT bypass
// the minimum key length check. Even with NO_AUTH=1 and an empty JWT_SECRET,
// the returned key must be at least 16 bytes.
func TestJWTSecret_NOAUTH_NoBypass(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("NO_AUTH", "1")
	secret := jwtSecret()
	if len(secret) < 16 {
		t.Errorf("jwtSecret() with NO_AUTH=1 returned key of length %d, want at least 16", len(secret))
	}
}

// TestJWTSecret_ShortSecretFatal verifies that jwtSecret() with a JWT_SECRET
// shorter than 16 characters causes the process to exit with a fatal error.
// Since log.Fatalf calls os.Exit(1), we test this via a subprocess.
func TestJWTSecret_ShortSecretFatal(t *testing.T) {
	// This test runs itself as a subprocess to verify the fatal exit
	// path. When JWT_SECRET_SHORT_FATAL_TEST is set, we call jwtSecret()
	// with a short key (which should call log.Fatalf) and let the process
	// die. The parent process checks the exit code.
	if os.Getenv("JWT_SECRET_SHORT_FATAL_TEST") == "1" {
		_ = os.Setenv("JWT_SECRET", "short")
		jwtSecret()
		os.Exit(0) // should not reach here
	}

	ctx := t.Context()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestJWTSecret_ShortSecretFatal")
	cmd.Env = append(os.Environ(), "JWT_SECRET_SHORT_FATAL_TEST=1")
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("expected non-zero exit from short JWT_SECRET, got 0")
	}
	if !strings.Contains(string(out), "JWT_SECRET must be at least 16 characters") {
		t.Errorf("expected fatal message about minimum length, got: %s", string(out))
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
