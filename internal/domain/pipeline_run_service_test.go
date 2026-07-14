package domain_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/decko/flux/internal/adapter/orchestrator"
	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
	"github.com/decko/flux/pkg/authctx"
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

func (r *mockPipelineRunRepo) List(_ context.Context, filter repository.PipelineRunFilter) ([]model.PipelineRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]model.PipelineRun, 0, len(r.store))
	for _, run := range r.store {
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		if filter.ProjectID != "" && run.ProjectID != filter.ProjectID {
			continue
		}
		if filter.TicketID != "" && run.TicketID != filter.TicketID {
			continue
		}
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

func (r *mockPipelineRunRepo) HasActiveRun(_ context.Context, projectID, ticketID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, run := range r.store {
		if run.ProjectID == projectID && run.TicketID == ticketID {
			if run.Status == model.RunStatusPending || run.Status == model.RunStatusRunning {
				return true, nil
			}
		}
	}
	return false, nil
}

// ─── Stub: OrchestratorAdapter ──────────────────────────────────────────────

type stubOrchestrator struct {
	mu           sync.Mutex
	triggeredIDs []string
	canceledIDs  []string
}

func (s *stubOrchestrator) Name() string { return "stub" }

func (s *stubOrchestrator) Trigger(_ context.Context, run model.PipelineRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.triggeredIDs = append(s.triggeredIDs, run.ID)
	return nil
}

func (s *stubOrchestrator) Cancel(_ context.Context, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canceledIDs = append(s.canceledIDs, runID)
	return nil
}

func (s *stubOrchestrator) Status(_ context.Context, _ string) (*model.PipelineRun, error) {
	return nil, nil
}

func (s *stubOrchestrator) Logs(_ context.Context, _ string) (<-chan orchestrator.LogEntry, error) {
	return nil, nil
}

func (s *stubOrchestrator) Health(_ context.Context) error {
	return nil
}

// failingStubOrchestrator is a stub that returns an error from Trigger,
// used to test the failure path where the orchestrator invocation fails.
type failingStubOrchestrator struct {
	mu           sync.Mutex
	triggeredIDs []string
}

func (s *failingStubOrchestrator) Name() string { return "failing-stub" }

func (s *failingStubOrchestrator) Trigger(_ context.Context, run model.PipelineRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.triggeredIDs = append(s.triggeredIDs, run.ID)
	return errors.New("orchestrator: simulated failure")
}

func (s *failingStubOrchestrator) Cancel(_ context.Context, runID string) error { return nil }
func (s *failingStubOrchestrator) Status(_ context.Context, _ string) (*model.PipelineRun, error) {
	return nil, nil
}
func (s *failingStubOrchestrator) Logs(_ context.Context, _ string) (<-chan orchestrator.LogEntry, error) {
	return nil, nil
}
func (s *failingStubOrchestrator) Health(_ context.Context) error { return nil }

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

// ─── Trigger ─────────────────────────────────────────────────────────────────

func TestPipelineRunService_Trigger(t *testing.T) {
	repo := newMockPipelineRunRepo()
	orch := &stubOrchestrator{}
	svc := domain.NewPipelineRunService(repo, domain.WithOrchestrator(orch))
	ctx := context.Background()

	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, svc.Create(ctx, run))

	err := svc.Trigger(ctx, "run-1")
	must(t, err)

	// Verify status was updated to completed (orchestrator succeeded).
	got, err := svc.Get(ctx, "run-1")
	must(t, err)
	if got.Status != model.RunStatusCompleted {
		t.Errorf("got Status %q, want %q", got.Status, model.RunStatusCompleted)
	}
	if got.CompletedAt == nil {
		t.Error("expected CompletedAt to be set on success")
	}

	// Verify adapter.Trigger was called.
	orch.mu.Lock()
	triggered := len(orch.triggeredIDs) == 1 && orch.triggeredIDs[0] == "run-1"
	orch.mu.Unlock()
	if !triggered {
		t.Errorf("expected adapter.Trigger to be called with run-1; calls: %v", orch.triggeredIDs)
	}
}

func TestPipelineRunService_Trigger_NotFound(t *testing.T) {
	repo := newMockPipelineRunRepo()
	orch := &stubOrchestrator{}
	svc := domain.NewPipelineRunService(repo, domain.WithOrchestrator(orch))
	ctx := context.Background()

	err := svc.Trigger(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPipelineRunService_Trigger_AlreadyActive(t *testing.T) {
	repo := newMockPipelineRunRepo()
	orch := &stubOrchestrator{}
	svc := domain.NewPipelineRunService(repo, domain.WithOrchestrator(orch))
	ctx := context.Background()

	tests := []struct {
		name   string
		status model.RunStatus
	}{
		{"running", model.RunStatusRunning},
		{"completed", model.RunStatusCompleted},
		{"failed", model.RunStatusFailed},
		{"canceled", model.RunStatusCanceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := testPipelineRun("run-"+tt.name, "proj-1", "ticket-1", tt.status)
			must(t, svc.Create(ctx, run))

			err := svc.Trigger(ctx, run.ID)
			if !errors.Is(err, domain.ErrRunNotPending) {
				t.Fatalf("expected ErrRunNotPending for status %q, got %v", tt.status, err)
			}

			// Verify orchestrator was NOT called.
			orch.mu.Lock()
			calls := len(orch.triggeredIDs)
			orch.mu.Unlock()
			if calls > 0 {
				t.Errorf("orchestrator was called %d times, want 0 for non-pending run", calls)
			}
		})
	}
}

func TestPipelineRunService_Trigger_NoOrchestrator(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo) // no orchestrator
	ctx := context.Background()

	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, svc.Create(ctx, run))

	err := svc.Trigger(ctx, "run-1")
	if err == nil {
		t.Fatal("expected error when orchestrator is not configured, got nil")
	}
}

func TestPipelineRunService_Trigger_OrchFailure(t *testing.T) {
	repo := newMockPipelineRunRepo()
	orch := &failingStubOrchestrator{}
	svc := domain.NewPipelineRunService(repo, domain.WithOrchestrator(orch))
	ctx := context.Background()

	run := testPipelineRun("run-fail", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, svc.Create(ctx, run))

	err := svc.Trigger(ctx, "run-fail")
	if err == nil {
		t.Fatal("expected error from failing orchestrator, got nil")
	}

	// Verify status was updated to failed.
	got, err := svc.Get(ctx, "run-fail")
	must(t, err)
	if got.Status != model.RunStatusFailed {
		t.Errorf("got Status %q, want %q", got.Status, model.RunStatusFailed)
	}
	if got.CompletedAt == nil {
		t.Error("expected CompletedAt to be set on failure")
	}

	// Verify adapter.Trigger was called.
	orch.mu.Lock()
	triggered := len(orch.triggeredIDs) == 1 && orch.triggeredIDs[0] == "run-fail"
	orch.mu.Unlock()
	if !triggered {
		t.Errorf("expected adapter.Trigger to be called; calls: %v", orch.triggeredIDs)
	}
}

// ─── Cancel ──────────────────────────────────────────────────────────────────

func TestPipelineRunService_Cancel(t *testing.T) {
	repo := newMockPipelineRunRepo()
	orch := &stubOrchestrator{}
	svc := domain.NewPipelineRunService(repo, domain.WithOrchestrator(orch))
	ctx := context.Background()

	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusRunning)
	must(t, svc.Create(ctx, run))

	err := svc.Cancel(ctx, "run-1")
	must(t, err)

	// Verify status was updated to canceled.
	got, err := svc.Get(ctx, "run-1")
	must(t, err)
	if got.Status != model.RunStatusCanceled {
		t.Errorf("got Status %q, want %q", got.Status, model.RunStatusCanceled)
	}

	// Verify adapter.Cancel was called.
	orch.mu.Lock()
	canceled := len(orch.canceledIDs) == 1 && orch.canceledIDs[0] == "run-1"
	orch.mu.Unlock()
	if !canceled {
		t.Errorf("expected adapter.Cancel to be called with run-1; calls: %v", orch.canceledIDs)
	}
}

func TestPipelineRunService_Cancel_NotFound(t *testing.T) {
	repo := newMockPipelineRunRepo()
	orch := &stubOrchestrator{}
	svc := domain.NewPipelineRunService(repo, domain.WithOrchestrator(orch))
	ctx := context.Background()

	err := svc.Cancel(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── Audit Integration Tests ─────────────────────────────────────────────────

func TestPipelineRunService_Create_AuditRecorded(t *testing.T) {
	auditRepo := setupAuditDB(t)
	auditSvc := domain.NewAuditService(auditRepo)
	runRepo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(runRepo, domain.WithPipelineRunAuditService(auditSvc))
	ctx := authctx.WithUserID(context.Background(), "test-user")

	run := testPipelineRun("run-audit-1", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, svc.Create(ctx, run))

	events, err := auditRepo.List(context.Background(), repository.AuditFilter{})
	must(t, err)
	if len(events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(events))
	}
	if events[0].Action != model.AuditAction("pipeline_run.created") {
		t.Errorf("Action = %q, want %q", events[0].Action, "pipeline_run.created")
	}
	if events[0].ResourceID != run.ID {
		t.Errorf("ResourceID = %q, want %q", events[0].ResourceID, run.ID)
	}
	if events[0].ActorID != "test-user" {
		t.Errorf("ActorID = %q, want %q", events[0].ActorID, "test-user")
	}
}

func TestPipelineRunService_Update_AuditRecorded(t *testing.T) {
	auditRepo := setupAuditDB(t)
	auditSvc := domain.NewAuditService(auditRepo)
	runRepo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(runRepo, domain.WithPipelineRunAuditService(auditSvc))
	ctx := authctx.WithUserID(context.Background(), "test-user")

	run := testPipelineRun("run-audit-2", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, svc.Create(ctx, run))

	run.Status = model.RunStatusRunning
	must(t, svc.Update(ctx, run))

	events, err := auditRepo.List(context.Background(), repository.AuditFilter{})
	must(t, err)
	if len(events) != 2 {
		t.Fatalf("got %d audit events, want 2 (create + update)", len(events))
	}
	if events[0].Action != model.AuditAction("pipeline_run.updated") {
		t.Errorf("Action = %q, want %q", events[0].Action, "pipeline_run.updated")
	}
	if events[0].ResourceID != run.ID {
		t.Errorf("ResourceID = %q, want %q", events[0].ResourceID, run.ID)
	}
}

func TestPipelineRunService_Trigger_AuditRecorded(t *testing.T) {
	auditRepo := setupAuditDB(t)
	auditSvc := domain.NewAuditService(auditRepo)
	runRepo := newMockPipelineRunRepo()
	orch := &stubOrchestrator{}
	svc := domain.NewPipelineRunService(runRepo,
		domain.WithOrchestrator(orch),
		domain.WithPipelineRunAuditService(auditSvc))
	ctx := authctx.WithUserID(context.Background(), "test-user")

	run := testPipelineRun("run-audit-3", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, svc.Create(ctx, run))

	must(t, svc.Trigger(ctx, run.ID))

	events, err := auditRepo.List(context.Background(), repository.AuditFilter{})
	must(t, err)
	if len(events) != 2 {
		t.Fatalf("got %d audit events, want 2 (create + trigger)", len(events))
	}
	if events[0].Action != model.AuditAction("pipeline_run.triggered") {
		t.Errorf("Action = %q, want %q", events[0].Action, "pipeline_run.triggered")
	}
	if events[0].ResourceID != run.ID {
		t.Errorf("ResourceID = %q, want %q", events[0].ResourceID, run.ID)
	}
}

func TestPipelineRunService_Cancel_AuditRecorded(t *testing.T) {
	auditRepo := setupAuditDB(t)
	auditSvc := domain.NewAuditService(auditRepo)
	runRepo := newMockPipelineRunRepo()
	orch := &stubOrchestrator{}
	svc := domain.NewPipelineRunService(runRepo,
		domain.WithOrchestrator(orch),
		domain.WithPipelineRunAuditService(auditSvc))
	ctx := authctx.WithUserID(context.Background(), "test-user")

	run := testPipelineRun("run-audit-4", "proj-1", "ticket-1", model.RunStatusRunning)
	must(t, svc.Create(ctx, run))

	must(t, svc.Cancel(ctx, run.ID))

	events, err := auditRepo.List(context.Background(), repository.AuditFilter{})
	must(t, err)
	if len(events) != 2 {
		t.Fatalf("got %d audit events, want 2 (create + cancel)", len(events))
	}
	if events[0].Action != model.AuditAction("pipeline_run.canceled") {
		t.Errorf("Action = %q, want %q", events[0].Action, "pipeline_run.canceled")
	}
	if events[0].ResourceID != run.ID {
		t.Errorf("ResourceID = %q, want %q", events[0].ResourceID, run.ID)
	}
}

func TestPipelineRunService_AuditNil(t *testing.T) {
	runRepo := newMockPipelineRunRepo()
	orch := &stubOrchestrator{}
	svc := domain.NewPipelineRunService(runRepo, domain.WithOrchestrator(orch)) // no audit
	ctx := authctx.WithUserID(context.Background(), "test-user")

	run := testPipelineRun("run-noaudit", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, svc.Create(ctx, run))

	got, err := svc.Get(ctx, "run-noaudit")
	must(t, err)
	if got.ID != run.ID {
		t.Errorf("got ID %q, want %q", got.ID, run.ID)
	}

	must(t, svc.Trigger(ctx, "run-noaudit"))
	must(t, svc.Cancel(ctx, "run-noaudit"))
}

// ─── Reconciler ───────────────────────────────────────────────────────────────

func TestPipelineRunService_ReconcileStrandedRuns(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo)
	ctx := context.Background()
	now := time.Now().UTC()

	// Create runs with various statuses and ages.
	oldRunning := testPipelineRun("old-running", "proj-1", "t-1", model.RunStatusRunning)
	oldRunning.StartedAt = now.Add(-2 * time.Hour) // 2 hours ago — stranded
	recentRunning := testPipelineRun("recent-running", "proj-1", "t-2", model.RunStatusRunning)
	recentRunning.StartedAt = now.Add(-10 * time.Minute) // 10 min ago — still recent
	completed := testPipelineRun("completed", "proj-1", "t-3", model.RunStatusCompleted)
	pending := testPipelineRun("pending", "proj-1", "t-4", model.RunStatusPending)

	for _, run := range []model.PipelineRun{oldRunning, recentRunning, completed, pending} {
		// Bypass the mock's Create (which would set status) by directly inserting.
		repo.mu.Lock()
		repo.store[run.ID] = run
		repo.mu.Unlock()
	}

	// Reconcile with a 1-hour threshold — only oldRunning should be affected.
	count, err := svc.ReconcileStrandedRuns(ctx, 1*time.Hour)
	must(t, err)
	if count != 1 {
		t.Fatalf("expected 1 reconciled run, got %d", count)
	}

	// Verify oldRunning was transitioned to failed.
	got, err := svc.Get(ctx, "old-running")
	must(t, err)
	if got.Status != model.RunStatusFailed {
		t.Errorf("got Status %q, want %q", got.Status, model.RunStatusFailed)
	}
	if got.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}

	// Verify recentRunning was NOT touched.
	got2, err := svc.Get(ctx, "recent-running")
	must(t, err)
	if got2.Status != model.RunStatusRunning {
		t.Errorf("recent run should still be running, got %q", got2.Status)
	}
}

func TestPipelineRunService_ReconcileStrandedRuns_Empty(t *testing.T) {
	repo := newMockPipelineRunRepo()
	svc := domain.NewPipelineRunService(repo)
	ctx := context.Background()

	count, err := svc.ReconcileStrandedRuns(ctx, 1*time.Hour)
	must(t, err)
	if count != 0 {
		t.Errorf("expected 0 reconciled runs, got %d", count)
	}
}
