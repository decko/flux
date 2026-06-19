package domain_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Mock: PipelineRunRepository ────────────────────────────────────────────
// Note: no Delete method — matches the PipelineRunRepository interface.

type mockPipelineRunRepo struct {
	mu    sync.Mutex
	store map[string]model.PipelineRun
}

func newMockPipelineRunRepo() *mockPipelineRunRepo {
	return &mockPipelineRunRepo{store: make(map[string]model.PipelineRun)}
}

func (r *mockPipelineRunRepo) Create(_ context.Context, run model.PipelineRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[run.ID]; exists {
		return errors.New("already exists")
	}
	r.store[run.ID] = run
	return nil
}

func (r *mockPipelineRunRepo) Get(_ context.Context, id string) (model.PipelineRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, exists := r.store[id]
	if !exists {
		return model.PipelineRun{}, repository.ErrNotFound
	}
	return run, nil
}

func (r *mockPipelineRunRepo) List(_ context.Context, _ repository.PipelineRunFilter) ([]model.PipelineRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]model.PipelineRun, 0, len(r.store))
	for _, run := range r.store {
		result = append(result, run)
	}
	return result, nil
}

func (r *mockPipelineRunRepo) Update(_ context.Context, run model.PipelineRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[run.ID]; !exists {
		return repository.ErrNotFound
	}
	r.store[run.ID] = run
	return nil
}

// ─── Test Helper ────────────────────────────────────────────────────────────

func testPipelineRun(id, projectID, ticketID string, status model.RunStatus) model.PipelineRun {
	now := time.Now().UTC().Truncate(time.Second)
	return model.PipelineRun{
		ID:           id,
		ProjectID:    projectID,
		TicketID:     ticketID,
		Orchestrator: "soda",
		Pipeline:     "dev-loop",
		Status:       status,
		Phases:       []model.PhaseResult{},
		StartedAt:    now,
		CompletedAt:  nil,
		Cost:         nil,
	}
}

// ─── PipelineRunService Tests ────────────────────────────────────────────────

func TestPipelineRunService_Create(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo)
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)

	err := svc.Create(ctx, run)
	must(t, err)

	// Verify it was stored in the repo.
	got, err := repo.Get(ctx, "run-1")
	must(t, err)
	if got.ID != run.ID {
		t.Errorf("got ID %q, want %q", got.ID, run.ID)
	}
	if got.TicketID != run.TicketID {
		t.Errorf("got TicketID %q, want %q", got.TicketID, run.TicketID)
	}
}

func TestPipelineRunService_Create_Invalid(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo)
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "", model.RunStatusPending) // missing TicketID

	err := svc.Create(ctx, run)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatal("expected validation error, not ErrNotFound")
	}

	// Verify the mock was NOT called (run should not be stored).
	_, getErr := repo.Get(ctx, "run-1")
	if !errors.Is(getErr, repository.ErrNotFound) {
		t.Fatal("run was stored in repo despite validation failure")
	}
}

func TestPipelineRunService_Get(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo)
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, svc.Create(ctx, run))

	got, err := svc.Get(ctx, "run-1")
	must(t, err)
	if got.ID != run.ID {
		t.Errorf("got ID %q, want %q", got.ID, run.ID)
	}
	if got.ProjectID != run.ProjectID {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, run.ProjectID)
	}
	if got.TicketID != run.TicketID {
		t.Errorf("got TicketID %q, want %q", got.TicketID, run.TicketID)
	}
	if got.Status != run.Status {
		t.Errorf("got Status %q, want %q", got.Status, run.Status)
	}
}

func TestPipelineRunService_Get_NotFound(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo)
	ctx := context.Background()

	_, err := svc.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPipelineRunService_List(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo)
	ctx := context.Background()

	runs := []model.PipelineRun{
		testPipelineRun("r1", "proj-a", "ticket-1", model.RunStatusPending),
		testPipelineRun("r2", "proj-b", "ticket-2", model.RunStatusRunning),
		testPipelineRun("r3", "proj-c", "ticket-3", model.RunStatusCompleted),
	}
	for _, run := range runs {
		must(t, svc.Create(ctx, run))
	}

	result, err := svc.List(ctx, repository.PipelineRunFilter{})
	must(t, err)
	if len(result) != len(runs) {
		t.Fatalf("got %d runs, want %d", len(result), len(runs))
	}

	// Verify all IDs are present.
	ids := make(map[string]bool)
	for _, run := range result {
		ids[run.ID] = true
	}
	for _, run := range runs {
		if !ids[run.ID] {
			t.Errorf("missing run %q in results", run.ID)
		}
	}
}

func TestPipelineRunService_Update(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo)
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, svc.Create(ctx, run))

	run.Status = model.RunStatusRunning
	must(t, svc.Update(ctx, run))

	got, err := svc.Get(ctx, "run-1")
	must(t, err)
	if got.Status != model.RunStatusRunning {
		t.Errorf("got Status %q, want %q", got.Status, model.RunStatusRunning)
	}
}

func TestPipelineRunService_Update_Invalid(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo)
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, svc.Create(ctx, run))

	run.Pipeline = "" // invalid
	err := svc.Update(ctx, run)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatal("expected validation error, not ErrNotFound")
	}

	// Verify the run was NOT modified in the store.
	got, getErr := repo.Get(ctx, "run-1")
	must(t, getErr)
	if got.Pipeline != "dev-loop" {
		t.Errorf("pipeline field changed despite validation failure: got %q, want %q", got.Pipeline, "dev-loop")
	}
}

func TestPipelineRunService_Update_NotFound(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo)
	ctx := context.Background()
	run := testPipelineRun("nonexistent", "proj-1", "ticket-1", model.RunStatusPending)

	err := svc.Update(ctx, run)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
