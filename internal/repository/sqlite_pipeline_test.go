package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Setup ─────────────────────────────────────────────────────────────────

// setupPipelineTestDB opens an in-memory SQLite database, configures it for
// SQLite use (pool + WAL), creates the pipeline_runs table via migration, and
// returns a SQLitePipelineRunRepository for testing.
func setupPipelineTestDB(t *testing.T) (*sql.DB, *repository.SQLitePipelineRunRepository) {
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

	repo := repository.NewSQLitePipelineRunRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to run migration: %v", err)
	}
	return db, repo
}

// ─── Create ────────────────────────────────────────────────────────────────

func TestSQLitePipelineRepo_Create(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)

	err := repo.Create(ctx, run)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestSQLitePipelineRepo_Create_DuplicateID(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)

	must(t, repo.Create(ctx, run))

	err := repo.Create(ctx, run)
	if err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
}

// ─── Get ───────────────────────────────────────────────────────────────────

func TestSQLitePipelineRepo_Get(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)

	must(t, repo.Create(ctx, run))

	got, err := repo.Get(ctx, "run-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if got.ID != run.ID {
		t.Errorf("got ID %q, want %q", got.ID, run.ID)
	}
	if got.ProjectID != run.ProjectID {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, run.ProjectID)
	}
	if got.TicketID != run.TicketID {
		t.Errorf("got TicketID %q, want %q", got.TicketID, run.TicketID)
	}
	if got.Orchestrator != run.Orchestrator {
		t.Errorf("got Orchestrator %q, want %q", got.Orchestrator, run.Orchestrator)
	}
	if got.Pipeline != run.Pipeline {
		t.Errorf("got Pipeline %q, want %q", got.Pipeline, run.Pipeline)
	}
	if got.Status != run.Status {
		t.Errorf("got Status %q, want %q", got.Status, run.Status)
	}
}

func TestSQLitePipelineRepo_Get_NotFound(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── List ──────────────────────────────────────────────────────────────────

func TestSQLitePipelineRepo_List(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()

	runs := []model.PipelineRun{
		testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending),
		testPipelineRun("run-2", "proj-1", "ticket-1", model.RunStatusRunning),
		testPipelineRun("run-3", "proj-2", "ticket-2", model.RunStatusCompleted),
	}
	for _, run := range runs {
		must(t, repo.Create(ctx, run))
	}

	result, err := repo.List(ctx, repository.PipelineRunFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != len(runs) {
		t.Errorf("got %d pipeline runs, want %d", len(result), len(runs))
	}
}

func TestSQLitePipelineRepo_List_Empty(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()

	result, err := repo.List(ctx, repository.PipelineRunFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("got %d pipeline runs, want 0", len(result))
	}
}

func TestSQLitePipelineRepo_List_FilterByProject(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()

	runs := []model.PipelineRun{
		testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending),
		testPipelineRun("run-2", "proj-1", "ticket-1", model.RunStatusRunning),
		testPipelineRun("run-3", "proj-2", "ticket-2", model.RunStatusCompleted),
	}
	for _, run := range runs {
		must(t, repo.Create(ctx, run))
	}

	result, err := repo.List(ctx, repository.PipelineRunFilter{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d pipeline runs, want 2", len(result))
	}
}

func TestSQLitePipelineRepo_List_FilterByTicket(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()

	runs := []model.PipelineRun{
		testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending),
		testPipelineRun("run-2", "proj-1", "ticket-1", model.RunStatusRunning),
		testPipelineRun("run-3", "proj-1", "ticket-2", model.RunStatusCompleted),
	}
	for _, run := range runs {
		must(t, repo.Create(ctx, run))
	}

	result, err := repo.List(ctx, repository.PipelineRunFilter{TicketID: "ticket-1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d pipeline runs, want 2", len(result))
	}
}

func TestSQLitePipelineRepo_List_FilterByStatus(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()

	runs := []model.PipelineRun{
		testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending),
		testPipelineRun("run-2", "proj-1", "ticket-1", model.RunStatusRunning),
		testPipelineRun("run-3", "proj-2", "ticket-2", model.RunStatusCompleted),
	}
	for _, run := range runs {
		must(t, repo.Create(ctx, run))
	}

	result, err := repo.List(ctx, repository.PipelineRunFilter{Status: model.RunStatusCompleted})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d pipeline runs, want 1", len(result))
	}
	if result[0].ID != "run-3" {
		t.Errorf("got run ID %q, want %q", result[0].ID, "run-3")
	}
}

// ─── Update ────────────────────────────────────────────────────────────────

func TestSQLitePipelineRepo_Update(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, repo.Create(ctx, run))

	run.Status = model.RunStatusRunning
	must(t, repo.Update(ctx, run))

	got, err := repo.Get(ctx, "run-1")
	if err != nil {
		t.Fatalf("Get after update returned error: %v", err)
	}
	if got.Status != model.RunStatusRunning {
		t.Errorf("got Status %q, want %q", got.Status, model.RunStatusRunning)
	}
}

func TestSQLitePipelineRepo_Update_NotFound(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()
	run := testPipelineRun("nonexistent", "proj-1", "ticket-1", model.RunStatusPending)

	err := repo.Update(ctx, run)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── JSON Round Trip ───────────────────────────────────────────────────────

func TestSQLitePipelineRepo_JSONRoundTrip(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	completedAt := now.Add(5 * time.Minute).UTC().Truncate(time.Second)

	cost := model.CostBreakdown{
		Total:    0.42,
		Currency: "USD",
		ByPhase:  map[string]float64{"dev-loop": 0.15, "review": 0.27},
	}

	run := model.PipelineRun{
		ID:           "run-full",
		ProjectID:    "proj-1",
		TicketID:     "ticket-1",
		Orchestrator: "soda",
		Pipeline:     "dev-loop",
		Status:       model.RunStatusCompleted,
		Phases: []model.PhaseResult{
			{Name: "plan", Status: model.RunStatusCompleted, Duration: 5000000000, Output: "plan output", StartedAt: now},
			{Name: "code", Status: model.RunStatusCompleted, Duration: 15000000000, Output: "code output", Error: "", StartedAt: now.Add(time.Second)},
		},
		StartedAt:   now,
		CompletedAt: &completedAt,
		Cost:        &cost,
	}

	must(t, repo.Create(ctx, run))

	got, err := repo.Get(ctx, "run-full")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	// Verify scalar fields.
	if got.ID != run.ID {
		t.Errorf("got ID %q, want %q", got.ID, run.ID)
	}
	if got.ProjectID != run.ProjectID {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, run.ProjectID)
	}
	if got.TicketID != run.TicketID {
		t.Errorf("got TicketID %q, want %q", got.TicketID, run.TicketID)
	}
	if got.Orchestrator != run.Orchestrator {
		t.Errorf("got Orchestrator %q, want %q", got.Orchestrator, run.Orchestrator)
	}
	if got.Pipeline != run.Pipeline {
		t.Errorf("got Pipeline %q, want %q", got.Pipeline, run.Pipeline)
	}
	if got.Status != run.Status {
		t.Errorf("got Status %q, want %q", got.Status, run.Status)
	}
	if !got.StartedAt.Equal(run.StartedAt) {
		t.Errorf("got StartedAt %v, want %v", got.StartedAt, run.StartedAt)
	}

	// Verify CompletedAt.
	if got.CompletedAt == nil {
		t.Fatal("expected non-nil CompletedAt, got nil")
	}
	if !got.CompletedAt.Equal(*run.CompletedAt) {
		t.Errorf("got CompletedAt %v, want %v", got.CompletedAt, *run.CompletedAt)
	}

	// Verify Cost.
	if got.Cost == nil {
		t.Fatal("expected non-nil Cost, got nil")
	}
	if got.Cost.Total != run.Cost.Total {
		t.Errorf("got Cost.Total %f, want %f", got.Cost.Total, run.Cost.Total)
	}
	if got.Cost.Currency != run.Cost.Currency {
		t.Errorf("got Cost.Currency %q, want %q", got.Cost.Currency, run.Cost.Currency)
	}
	if len(got.Cost.ByPhase) != len(run.Cost.ByPhase) {
		t.Errorf("got %d cost phases, want %d", len(got.Cost.ByPhase), len(run.Cost.ByPhase))
	} else {
		for phase, want := range run.Cost.ByPhase {
			gotVal, ok := got.Cost.ByPhase[phase]
			if !ok {
				t.Errorf("missing cost phase %q", phase)
				continue
			}
			if gotVal != want {
				t.Errorf("cost phase %q: got %f, want %f", phase, gotVal, want)
			}
		}
	}

	// Verify Phases.
	if len(got.Phases) != len(run.Phases) {
		t.Fatalf("got %d phases, want %d", len(got.Phases), len(run.Phases))
	}
	for i, want := range run.Phases {
		if got.Phases[i].Name != want.Name {
			t.Errorf("phases[%d] Name: got %q, want %q", i, got.Phases[i].Name, want.Name)
		}
		if got.Phases[i].Status != want.Status {
			t.Errorf("phases[%d] Status: got %q, want %q", i, got.Phases[i].Status, want.Status)
		}
		if got.Phases[i].Duration != want.Duration {
			t.Errorf("phases[%d] Duration: got %v, want %v", i, got.Phases[i].Duration, want.Duration)
		}
		if got.Phases[i].Output != want.Output {
			t.Errorf("phases[%d] Output: got %q, want %q", i, got.Phases[i].Output, want.Output)
		}
		if got.Phases[i].Error != want.Error {
			t.Errorf("phases[%d] Error: got %q, want %q", i, got.Phases[i].Error, want.Error)
		}
		if !got.Phases[i].StartedAt.Equal(want.StartedAt) {
			t.Errorf("phases[%d] StartedAt: got %v, want %v", i, got.Phases[i].StartedAt, want.StartedAt)
		}
	}
}

// ─── Nil Optionals ─────────────────────────────────────────────────────────

func TestSQLitePipelineRepo_NilOptionals(t *testing.T) {
	_, repo := setupPipelineTestDB(t)
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, repo.Create(ctx, run))

	got, err := repo.Get(ctx, "run-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if got.CompletedAt != nil {
		t.Errorf("expected nil CompletedAt, got %v", got.CompletedAt)
	}
	if got.Cost != nil {
		t.Errorf("expected nil Cost, got %+v", got.Cost)
	}
	if len(got.Phases) != 0 {
		t.Errorf("expected empty Phases, got %d", len(got.Phases))
	}
}
