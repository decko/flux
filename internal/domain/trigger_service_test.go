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
	svc := NewTriggerService(pipelineSvc, projectRepo, "flux-bot")

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
	if pipelineSvc.createdRuns[0].ProjectID != "proj-1" {
		t.Errorf("run.ProjectID = %q, want %q", pipelineSvc.createdRuns[0].ProjectID, "proj-1")
	}
	if pipelineSvc.createdRuns[0].TicketID != "ticket-1" {
		t.Errorf("run.TicketID = %q, want %q", pipelineSvc.createdRuns[0].TicketID, "ticket-1")
	}
}

func TestTriggerService_CheckAndTrigger_WithoutTriggerLabel(t *testing.T) {
	projectRepo := &stubProjectRepo{
		project: model.Project{ID: "proj-1"},
	}
	pipelineSvc := &stubPipelineRunService{}
	svc := NewTriggerService(pipelineSvc, projectRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"bug"},
		Status:    model.TicketStatusOpen,
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
	svc := NewTriggerService(pipelineSvc, projectRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    nil,
		Status:    model.TicketStatusOpen,
	}

	err := svc.CheckAndTrigger(context.Background(), ticket)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 0 {
		t.Errorf("expected 0 pipeline runs, got %d", len(pipelineSvc.createdRuns))
	}
}
