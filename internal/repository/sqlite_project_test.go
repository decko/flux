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

// ─── Setup ─────────────────────────────────────────────────────────────────

// setupTestDB opens an in-memory SQLite database, configures it for SQLite
// use (pool + WAL), creates the projects table via migration, and returns a
// SQLiteProjectRepository for testing.
func setupTestDB(t *testing.T) (*sqlx.DB, *repository.SQLiteProjectRepository) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}

	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlite")
	repo := repository.NewSQLiteProjectRepository(sdb)
	return sdb, repo
}

// ─── Create ────────────────────────────────────────────────────────────────

func TestSQLiteProjectRepo_Create(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()
	p := testProject("proj-1", "test-project")

	err := repo.Create(ctx, p)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestSQLiteProjectRepo_Create_DuplicateID(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()
	p := testProject("proj-1", "test-project")

	must(t, repo.Create(ctx, p))

	err := repo.Create(ctx, p)
	if err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
}

// ─── Get ───────────────────────────────────────────────────────────────────

func TestSQLiteProjectRepo_Get(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()
	p := testProject("proj-1", "get-test")

	must(t, repo.Create(ctx, p))

	got, err := repo.Get(ctx, "proj-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if got.ID != p.ID {
		t.Errorf("got ID %q, want %q", got.ID, p.ID)
	}
	if got.Name != p.Name {
		t.Errorf("got Name %q, want %q", got.Name, p.Name)
	}
	if got.RepoURL != p.RepoURL {
		t.Errorf("got RepoURL %q, want %q", got.RepoURL, p.RepoURL)
	}
	if !got.CreatedAt.Equal(p.CreatedAt) {
		t.Errorf("got CreatedAt %v, want %v", got.CreatedAt, p.CreatedAt)
	}
	if !got.UpdatedAt.Equal(p.UpdatedAt) {
		t.Errorf("got UpdatedAt %v, want %v", got.UpdatedAt, p.UpdatedAt)
	}
}

func TestSQLiteProjectRepo_Get_NotFound(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── List ──────────────────────────────────────────────────────────────────

func TestSQLiteProjectRepo_List(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()

	projects := []model.Project{
		testProject("p1", "project-a"),
		testProject("p2", "project-b"),
		testProject("p3", "project-c"),
	}
	for _, p := range projects {
		must(t, repo.Create(ctx, p))
	}

	result, err := repo.List(ctx, repository.ProjectFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != len(projects) {
		t.Errorf("got %d projects, want %d", len(result), len(projects))
	}
}

func TestSQLiteProjectRepo_List_Empty(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()

	result, err := repo.List(ctx, repository.ProjectFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("got %d projects, want 0", len(result))
	}
}

// ─── Update ────────────────────────────────────────────────────────────────

func TestSQLiteProjectRepo_Update(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()
	p := testProject("proj-1", "original")
	must(t, repo.Create(ctx, p))

	p.Name = "updated"
	p.RepoURL = "https://github.com/example/updated"
	must(t, repo.Update(ctx, p))

	got, err := repo.Get(ctx, "proj-1")
	if err != nil {
		t.Fatalf("Get after update returned error: %v", err)
	}
	if got.Name != "updated" {
		t.Errorf("got Name %q, want %q", got.Name, "updated")
	}
	if got.RepoURL != "https://github.com/example/updated" {
		t.Errorf("got RepoURL %q, want %q", got.RepoURL, "https://github.com/example/updated")
	}
}

func TestSQLiteProjectRepo_Update_NotFound(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()
	p := testProject("nonexistent", "ghost")

	err := repo.Update(ctx, p)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── Delete ────────────────────────────────────────────────────────────────

func TestSQLiteProjectRepo_Delete(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()
	p := testProject("proj-1", "delete-me")
	must(t, repo.Create(ctx, p))

	must(t, repo.Delete(ctx, "proj-1"))

	_, err := repo.Get(ctx, "proj-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSQLiteProjectRepo_Delete_NotFound(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── JSON Round Trip ───────────────────────────────────────────────────────

func TestSQLiteProjectRepo_JSONRoundTrip(t *testing.T) {
	_, repo := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	p := model.Project{
		ID:      "proj-full",
		Name:    "full-project",
		RepoURL: "https://github.com/example/full",
		Definition: model.ProjectDefinition{
			Language:     "Go",
			Framework:    "chi",
			Conventions:  []string{"conventional-commits", "tdd"},
			Architecture: "hexagonal",
		},
		Adapters: []model.AdapterConfig{
			{Type: "github", Config: map[string]string{"token": "env:GITHUB_TOKEN"}},
			{Type: "jira", Config: map[string]string{"url": "https://jira.example.com"}},
		},
		Pipelines: []model.PipelineConfig{
			{Type: "dev-loop", Name: "dev", Config: map[string]string{"max_retries": "3"}},
			{Type: "review", Name: "review", Config: map[string]string{"agents": "2"}},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	must(t, repo.Create(ctx, p))

	got, err := repo.Get(ctx, "proj-full")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	// Verify Definition nested fields.
	if got.Definition.Language != p.Definition.Language {
		t.Errorf("got Language %q, want %q", got.Definition.Language, p.Definition.Language)
	}
	if got.Definition.Framework != p.Definition.Framework {
		t.Errorf("got Framework %q, want %q", got.Definition.Framework, p.Definition.Framework)
	}
	if len(got.Definition.Conventions) != len(p.Definition.Conventions) {
		t.Errorf("got %d conventions, want %d", len(got.Definition.Conventions), len(p.Definition.Conventions))
	} else {
		for i, want := range p.Definition.Conventions {
			if got.Definition.Conventions[i] != want {
				t.Errorf("convention[%d]: got %q, want %q", i, got.Definition.Conventions[i], want)
			}
		}
	}
	if got.Definition.Architecture != p.Definition.Architecture {
		t.Errorf("got Architecture %q, want %q", got.Definition.Architecture, p.Definition.Architecture)
	}

	// Verify Adapters.
	if len(got.Adapters) != len(p.Adapters) {
		t.Errorf("got %d adapters, want %d", len(got.Adapters), len(p.Adapters))
	} else {
		for i, want := range p.Adapters {
			if got.Adapters[i].Type != want.Type {
				t.Errorf("adapter[%d] Type: got %q, want %q", i, got.Adapters[i].Type, want.Type)
			}
			for k, v := range want.Config {
				if got.Adapters[i].Config[k] != v {
					t.Errorf("adapter[%d] Config[%q]: got %q, want %q", i, k, got.Adapters[i].Config[k], v)
				}
			}
		}
	}

	// Verify Pipelines.
	if len(got.Pipelines) != len(p.Pipelines) {
		t.Errorf("got %d pipelines, want %d", len(got.Pipelines), len(p.Pipelines))
	} else {
		for i, want := range p.Pipelines {
			if got.Pipelines[i].Type != want.Type {
				t.Errorf("pipeline[%d] Type: got %q, want %q", i, got.Pipelines[i].Type, want.Type)
			}
			if got.Pipelines[i].Name != want.Name {
				t.Errorf("pipeline[%d] Name: got %q, want %q", i, got.Pipelines[i].Name, want.Name)
			}
			for k, v := range want.Config {
				if got.Pipelines[i].Config[k] != v {
					t.Errorf("pipeline[%d] Config[%q]: got %q, want %q", i, k, got.Pipelines[i].Config[k], v)
				}
			}
		}
	}
}
