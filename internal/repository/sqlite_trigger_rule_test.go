package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// setupTriggerRuleTestDB opens an in-memory SQLite database, configures it for
// SQLite use, runs all migrations, and returns a SQLiteTriggerRuleRepository
// for testing.
func setupTriggerRuleTestDB(t *testing.T) *repository.SQLiteTriggerRuleRepository {
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
	return repository.NewSQLiteTriggerRuleRepository(sdb)
}

func testTriggerRule(id, projectID, label, pipeline string, enabled bool, priority int) model.TriggerRule {
	now := time.Now().UTC().Truncate(time.Second)
	return model.TriggerRule{
		ID:        id,
		ProjectID: projectID,
		Label:     label,
		Pipeline:  pipeline,
		Enabled:   enabled,
		Priority:  priority,
		Event:     model.DefaultEvent,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestSQLiteTriggerRuleRepo_CreateAndListByProject(t *testing.T) {
	repo := setupTriggerRuleTestDB(t)
	ctx := context.Background()

	r1 := testTriggerRule(uuid.New().String(), "proj-1", "bug", "fix", true, 10)
	r2 := testTriggerRule(uuid.New().String(), "proj-1", "feature", "dev", true, 5)
	r3 := testTriggerRule(uuid.New().String(), "proj-2", "bug", "fix", true, 10)

	for _, rule := range []model.TriggerRule{r1, r2, r3} {
		if err := repo.Create(ctx, rule); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
	}

	// List by project 1 — should return 2 rules ordered by priority desc.
	rules, err := repo.ListByProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("ListByProject returned error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].ID != r1.ID {
		t.Errorf("expected first rule to have ID %s (highest priority), got %s", r1.ID, rules[0].ID)
	}
	if rules[1].ID != r2.ID {
		t.Errorf("expected second rule to have ID %s, got %s", r2.ID, rules[1].ID)
	}

	// Verify fields.
	got := rules[0]
	if got.Label != r1.Label {
		t.Errorf("got Label %q, want %q", got.Label, r1.Label)
	}
	if got.Pipeline != r1.Pipeline {
		t.Errorf("got Pipeline %q, want %q", got.Pipeline, r1.Pipeline)
	}
	if got.Enabled != r1.Enabled {
		t.Errorf("got Enabled %v, want %v", got.Enabled, r1.Enabled)
	}
	if got.Priority != r1.Priority {
		t.Errorf("got Priority %d, want %d", got.Priority, r1.Priority)
	}
	if !got.CreatedAt.Equal(r1.CreatedAt) {
		t.Errorf("got CreatedAt %v, want %v", got.CreatedAt, r1.CreatedAt)
	}
}

func TestSQLiteTriggerRuleRepo_ListByProject_Empty(t *testing.T) {
	repo := setupTriggerRuleTestDB(t)
	ctx := context.Background()

	rules, err := repo.ListByProject(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ListByProject returned error: %v", err)
	}
	if rules == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(rules) != 0 {
		t.Errorf("got %d rules, want 0", len(rules))
	}
}

func TestSQLiteTriggerRuleRepo_Update(t *testing.T) {
	repo := setupTriggerRuleTestDB(t)
	ctx := context.Background()

	rule := testTriggerRule(uuid.New().String(), "proj-1", "bug", "fix", true, 10)
	must(t, repo.Create(ctx, rule))

	rule.Label = "critical"
	rule.Pipeline = "hotfix"
	rule.Enabled = false
	rule.Priority = 20
	must(t, repo.Update(ctx, rule))

	rules, err := repo.ListByProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("ListByProject returned error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	got := rules[0]
	if got.Label != "critical" {
		t.Errorf("got Label %q, want %q", got.Label, "critical")
	}
	if got.Pipeline != "hotfix" {
		t.Errorf("got Pipeline %q, want %q", got.Pipeline, "hotfix")
	}
	if got.Enabled != false {
		t.Errorf("got Enabled %v, want false", got.Enabled)
	}
	if got.Priority != 20 {
		t.Errorf("got Priority %d, want 20", got.Priority)
	}
}

func TestSQLiteTriggerRuleRepo_Update_NotFound(t *testing.T) {
	repo := setupTriggerRuleTestDB(t)
	ctx := context.Background()

	rule := testTriggerRule("nonexistent", "proj-1", "bug", "fix", true, 10)
	err := repo.Update(ctx, rule)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSQLiteTriggerRuleRepo_Delete(t *testing.T) {
	repo := setupTriggerRuleTestDB(t)
	ctx := context.Background()

	rule := testTriggerRule(uuid.New().String(), "proj-1", "bug", "fix", true, 10)
	must(t, repo.Create(ctx, rule))

	must(t, repo.Delete(ctx, rule.ID))

	rules, err := repo.ListByProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("ListByProject returned error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after delete, got %d", len(rules))
	}
}

func TestSQLiteTriggerRuleRepo_Delete_NotFound(t *testing.T) {
	repo := setupTriggerRuleTestDB(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
