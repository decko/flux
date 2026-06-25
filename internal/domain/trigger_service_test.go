package domain

import (
	"context"
	"testing"

	"github.com/decko/flux/internal/model"
)

// stubPipelineRunService is a minimal implementation for TriggerService tests.
type stubPipelineRunService struct {
	createdRuns []model.PipelineRun
	createErr   error
}

func (s *stubPipelineRunService) Create(ctx context.Context, run model.PipelineRun) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.createdRuns = append(s.createdRuns, run)
	return nil
}

// stubProjectRepo returns a fixed project.
type stubProjectRepo struct {
	project model.Project
	err     error
}

func (r *stubProjectRepo) Get(ctx context.Context, id string) (model.Project, error) {
	return r.project, r.err
}

// stubRunRepo for dedup tests.
type stubRunRepo struct {
	hasActive bool
}

func (r *stubRunRepo) HasActiveRun(ctx context.Context, projectID, ticketID string) (bool, error) {
	return r.hasActive, nil
}

func TestTriggerService_CheckAndTrigger_WithTriggerLabel(t *testing.T) {
	projectRepo := &stubProjectRepo{
		project: model.Project{
			ID: "proj-1",
			Pipelines: []model.PipelineConfig{
				{Name: "default"},
			},
		},
	}
	pipelineSvc := &stubPipelineRunService{}
	runRepo := &stubRunRepo{}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"flux/agent", "bug"},
		Status:    model.TicketStatusOpen,
	}

	err := svc.CheckAndTrigger(context.Background(), ticket)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 1 {
		t.Fatalf("expected 1 pipeline run, got %d", len(pipelineSvc.createdRuns))
	}
}

func TestTriggerService_CheckAndTrigger_WithoutTriggerLabel(t *testing.T) {
	projectRepo := &stubProjectRepo{
		project: model.Project{ID: "proj-1"},
	}
	pipelineSvc := &stubPipelineRunService{}
	runRepo := &stubRunRepo{}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"bug"},
	}

	err := svc.CheckAndTrigger(context.Background(), ticket)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 0 {
		t.Errorf("expected 0 pipeline runs, got %d", len(pipelineSvc.createdRuns))
	}
}

func TestTriggerService_CheckAndTrigger_EmptyLabels(t *testing.T) {
	projectRepo := &stubProjectRepo{
		project: model.Project{ID: "proj-1"},
	}
	pipelineSvc := &stubPipelineRunService{}
	runRepo := &stubRunRepo{}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    nil,
	}

	err := svc.CheckAndTrigger(context.Background(), ticket)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 0 {
		t.Errorf("expected 0 pipeline runs, got %d", len(pipelineSvc.createdRuns))
	}
}

func TestTriggerService_CheckAndTrigger_Deduplication(t *testing.T) {
	projectRepo := &stubProjectRepo{
		project: model.Project{
			ID: "proj-1",
			Pipelines: []model.PipelineConfig{
				{Name: "default"},
			},
		},
	}
	pipelineSvc := &stubPipelineRunService{}
	runRepo := &stubRunRepo{hasActive: true} // active run exists
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"flux/agent"},
	}

	err := svc.CheckAndTrigger(context.Background(), ticket)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 0 {
		t.Errorf("expected 0 pipeline runs (dedup), got %d", len(pipelineSvc.createdRuns))
	}
}
