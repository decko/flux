package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

func setupUserTestDB(t *testing.T) *repository.SQLiteUserRepository {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}

	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlite")
	repo := repository.NewSQLiteUserRepository(sdb)
	return repo
}

func testUser(id, email, role string) model.User {
	return model.User{
		ID:           id,
		Email:        email,
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
		Role:         role,
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
	}
}

func TestSQLiteUserRepo_Create(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()
	u := testUser("user-1", "user@example.com", "admin")

	err := repo.Create(ctx, u)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestSQLiteUserRepo_Create_DuplicateEmail(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()
	u1 := testUser("user-1", "same@example.com", "admin")
	u2 := testUser("user-2", "same@example.com", "user")

	must(t, repo.Create(ctx, u1))

	err := repo.Create(ctx, u2)
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}
}

func TestSQLiteUserRepo_GetByEmail(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()
	u := testUser("user-1", "user@example.com", "admin")
	must(t, repo.Create(ctx, u))

	got, err := repo.GetByEmail(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("GetByEmail returned error: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("got ID %q, want %q", got.ID, u.ID)
	}
	if got.Email != u.Email {
		t.Errorf("got Email %q, want %q", got.Email, u.Email)
	}
	if got.Role != u.Role {
		t.Errorf("got Role %q, want %q", got.Role, u.Role)
	}
	if got.CreatedAt != u.CreatedAt {
		t.Errorf("got CreatedAt %v, want %v", got.CreatedAt, u.CreatedAt)
	}
}

func TestSQLiteUserRepo_GetByEmail_NotFound(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSQLiteUserRepo_GetByID(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()
	u := testUser("user-1", "user@example.com", "admin")
	must(t, repo.Create(ctx, u))

	got, err := repo.GetByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("got ID %q, want %q", got.ID, u.ID)
	}
	if got.Email != u.Email {
		t.Errorf("got Email %q, want %q", got.Email, u.Email)
	}
}

func TestSQLiteUserRepo_GetByID_NotFound(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSQLiteUserRepo_List(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()

	users := []model.User{
		testUser("u1", "alice@example.com", "admin"),
		testUser("u2", "bob@example.com", "user"),
		testUser("u3", "carol@example.com", "user"),
	}
	for _, u := range users {
		must(t, repo.Create(ctx, u))
	}

	// List is not implemented yet — this will fail (RED).
	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got) != len(users) {
		t.Errorf("got %d users, want %d", len(got), len(users))
	}
}

func TestSQLiteUserRepo_Delete(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()

	u := testUser("user-1", "delete@example.com", "user")
	must(t, repo.Create(ctx, u))

	// Delete is not implemented yet — this will fail (RED).
	must(t, repo.Delete(ctx, "user-1"))

	_, err := repo.GetByID(ctx, "user-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSQLiteUserRepo_Delete_NotFound(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSQLiteUserRepo_Count(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()

	users := []model.User{
		testUser("u1", "alice@example.com", "admin"),
		testUser("u2", "bob@example.com", "user"),
	}
	for _, u := range users {
		must(t, repo.Create(ctx, u))
	}

	// Count is not implemented yet — this will fail (RED).
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if count != 2 {
		t.Errorf("got count %d, want %d", count, 2)
	}
}

func TestSQLiteUserRepo_CountByRole(t *testing.T) {
	repo := setupUserTestDB(t)
	ctx := context.Background()

	users := []model.User{
		testUser("u1", "admin1@example.com", "admin"),
		testUser("u2", "admin2@example.com", "admin"),
		testUser("u3", "user1@example.com", "user"),
	}
	for _, u := range users {
		must(t, repo.Create(ctx, u))
	}

	// CountByRole is not implemented yet — this will fail (RED).
	adminCount, err := repo.CountByRole(ctx, "admin")
	if err != nil {
		t.Fatalf("CountByRole(admin) returned error: %v", err)
	}
	if adminCount != 2 {
		t.Errorf("got admin count %d, want %d", adminCount, 2)
	}

	userCount, err := repo.CountByRole(ctx, "user")
	if err != nil {
		t.Fatalf("CountByRole(user) returned error: %v", err)
	}
	if userCount != 1 {
		t.Errorf("got user count %d, want %d", userCount, 1)
	}
}
