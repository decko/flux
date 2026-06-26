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

// stubTriggerRuleRepo for trigger rule tests.
type stubTriggerRuleRepo struct {
	rules []model.TriggerRule
	err   error
}

func (r *stubTriggerRuleRepo) ListByProject(ctx context.Context, projectID string) ([]model.TriggerRule, error) {
	if r.err != nil {
		return nil, r.err
	}
	// Filter by project for realism, though tests typically set up per-test.
	var matched []model.TriggerRule
	for _, rule := range r.rules {
		if rule.ProjectID == projectID {
			matched = append(matched, rule)
		}
	}
	if matched == nil {
		return []model.TriggerRule{}, nil
	}
	return matched, nil
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
	// No DB rules — fallback to hardcoded "flux/agent" → "default".
	ruleRepo := &stubTriggerRuleRepo{}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, ruleRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"flux/agent", "bug"},
		Status:    model.TicketStatusOpen,
	}

	err := svc.CheckAndTrigger(context.Background(), ticket, model.DefaultEvent)
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
	ruleRepo := &stubTriggerRuleRepo{}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, ruleRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"bug"},
	}

	err := svc.CheckAndTrigger(context.Background(), ticket, model.DefaultEvent)
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
	ruleRepo := &stubTriggerRuleRepo{}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, ruleRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    nil,
	}

	err := svc.CheckAndTrigger(context.Background(), ticket, model.DefaultEvent)
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
	ruleRepo := &stubTriggerRuleRepo{}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, ruleRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"flux/agent"},
	}

	err := svc.CheckAndTrigger(context.Background(), ticket, model.DefaultEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 0 {
		t.Errorf("expected 0 pipeline runs (dedup), got %d", len(pipelineSvc.createdRuns))
	}
}

func TestTriggerService_CheckAndTrigger_DBRuleMatch(t *testing.T) {
	projectRepo := &stubProjectRepo{
		project: model.Project{
			ID: "proj-1",
			Pipelines: []model.PipelineConfig{
				{Name: "fix-pipeline"},
				{Name: "dev-pipeline"},
			},
		},
	}
	pipelineSvc := &stubPipelineRunService{}
	runRepo := &stubRunRepo{}
	ruleRepo := &stubTriggerRuleRepo{
		rules: []model.TriggerRule{
			{ProjectID: "proj-1", Label: "bug", Pipeline: "fix-pipeline", Enabled: true, Priority: 10},
			{ProjectID: "proj-1", Label: "feature", Pipeline: "dev-pipeline", Enabled: true, Priority: 5},
		},
	}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, ruleRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"bug", "critical"},
	}

	err := svc.CheckAndTrigger(context.Background(), ticket, model.DefaultEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 1 {
		t.Fatalf("expected 1 pipeline run, got %d", len(pipelineSvc.createdRuns))
	}
	if pipelineSvc.createdRuns[0].Pipeline != "fix-pipeline" {
		t.Errorf("expected pipeline %q, got %q", "fix-pipeline", pipelineSvc.createdRuns[0].Pipeline)
	}
}

func TestTriggerService_CheckAndTrigger_DBRulePriority(t *testing.T) {
	projectRepo := &stubProjectRepo{
		project: model.Project{
			ID: "proj-1",
			Pipelines: []model.PipelineConfig{
				{Name: "high-priority"},
				{Name: "low-priority"},
			},
		},
	}
	pipelineSvc := &stubPipelineRunService{}
	runRepo := &stubRunRepo{}
	// Both rules match the ticket label "urgent"; highest priority wins.
	ruleRepo := &stubTriggerRuleRepo{
		rules: []model.TriggerRule{
			{ProjectID: "proj-1", Label: "urgent", Pipeline: "low-priority", Enabled: true, Priority: 1},
			{ProjectID: "proj-1", Label: "urgent", Pipeline: "high-priority", Enabled: true, Priority: 100},
		},
	}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, ruleRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"urgent"},
	}

	err := svc.CheckAndTrigger(context.Background(), ticket, model.DefaultEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 1 {
		t.Fatalf("expected 1 pipeline run, got %d", len(pipelineSvc.createdRuns))
	}
	if pipelineSvc.createdRuns[0].Pipeline != "high-priority" {
		t.Errorf("expected highest-priority pipeline %q, got %q", "high-priority", pipelineSvc.createdRuns[0].Pipeline)
	}
}

func TestTriggerService_CheckAndTrigger_PipelineNotConfigured(t *testing.T) {
	projectRepo := &stubProjectRepo{
		project: model.Project{
			ID:        "proj-1",
			Pipelines: []model.PipelineConfig{}, // no pipelines configured
		},
	}
	pipelineSvc := &stubPipelineRunService{}
	runRepo := &stubRunRepo{}
	ruleRepo := &stubTriggerRuleRepo{
		rules: []model.TriggerRule{
			{ProjectID: "proj-1", Label: "bug", Pipeline: "fix-pipeline", Enabled: true, Priority: 10},
		},
	}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, ruleRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"bug"},
	}

	err := svc.CheckAndTrigger(context.Background(), ticket, model.DefaultEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 0 {
		t.Errorf("expected 0 pipeline runs (pipeline not configured), got %d", len(pipelineSvc.createdRuns))
	}
}

func TestTriggerService_CheckAndTrigger_EventFiltering(t *testing.T) {
	projectRepo := &stubProjectRepo{
		project: model.Project{
			ID: "proj-1",
			Pipelines: []model.PipelineConfig{
				{Name: "labeled-pipeline"},
				{Name: "pr-pipeline"},
			},
		},
	}
	pipelineSvc := &stubPipelineRunService{}
	runRepo := &stubRunRepo{}
	ruleRepo := &stubTriggerRuleRepo{
		rules: []model.TriggerRule{
			{ProjectID: "proj-1", Label: "bug", Pipeline: "labeled-pipeline", Enabled: true, Priority: 10, Event: "ticket.labeled"},
			{ProjectID: "proj-1", Label: "bug", Pipeline: "pr-pipeline", Enabled: true, Priority: 5, Event: "pull_request"},
		},
	}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, ruleRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"bug"},
	}

	// Only the ticket.labeled rule should match.
	err := svc.CheckAndTrigger(context.Background(), ticket, "ticket.labeled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 1 {
		t.Fatalf("expected 1 pipeline run, got %d", len(pipelineSvc.createdRuns))
	}
	if pipelineSvc.createdRuns[0].Pipeline != "labeled-pipeline" {
		t.Errorf("expected pipeline %q, got %q", "labeled-pipeline", pipelineSvc.createdRuns[0].Pipeline)
	}
}

func TestTriggerService_CheckAndTrigger_BackwardCompatEmptyEvent(t *testing.T) {
	projectRepo := &stubProjectRepo{
		project: model.Project{
			ID: "proj-1",
			Pipelines: []model.PipelineConfig{
				{Name: "legacy-pipeline"},
			},
		},
	}
	pipelineSvc := &stubPipelineRunService{}
	runRepo := &stubRunRepo{}
	// Rule with empty event — should match any event type.
	ruleRepo := &stubTriggerRuleRepo{
		rules: []model.TriggerRule{
			{ProjectID: "proj-1", Label: "bug", Pipeline: "legacy-pipeline", Enabled: true, Priority: 10, Event: ""},
		},
	}
	svc := NewTriggerService(pipelineSvc, projectRepo, runRepo, ruleRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-1",
		ProjectID: "proj-1",
		Labels:    []string{"bug"},
	}

	// Should match even with a non-default event type.
	err := svc.CheckAndTrigger(context.Background(), ticket, "pull_request")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelineSvc.createdRuns) != 1 {
		t.Fatalf("expected 1 pipeline run, got %d", len(pipelineSvc.createdRuns))
	}
}
