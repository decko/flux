package domain

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/decko/flux/internal/model"
)

// triggerRunner is the subset of PipelineRunService used by TriggerService
// to create and trigger pipeline runs automatically.
type triggerRunner interface {
	Create(ctx context.Context, run model.PipelineRun) error
}

// triggerProjectRepo is the subset of ProjectRepository used by TriggerService
// to look up pipeline configuration for a project.
type triggerProjectRepo interface {
	Get(ctx context.Context, id string) (model.Project, error)
}

// TriggerService evaluates tickets against trigger rules and automatically
// creates pipeline runs when conditions match. Currently uses hardcoded rules:
// a ticket labeled "flux/agent" triggers the project's default pipeline.
type TriggerService struct {
	pipelineSvc triggerRunner
	projectRepo triggerProjectRepo
	selfUser    string
}

// NewTriggerService creates a new TriggerService.
func NewTriggerService(
	pipelineSvc triggerRunner,
	projectRepo triggerProjectRepo,
	selfUser string,
) *TriggerService {
	return &TriggerService{
		pipelineSvc: pipelineSvc,
		projectRepo: projectRepo,
		selfUser:    selfUser,
	}
}

// CheckAndTrigger evaluates trigger rules against a ticket. If the ticket
// matches a rule, a new pipeline run is created and triggered.
func (s *TriggerService) CheckAndTrigger(ctx context.Context, ticket model.Ticket) error {
	// Rule: ticket labeled "flux/agent" triggers the project's default pipeline.
	if !hasLabel(ticket.Labels, "flux/agent") {
		return nil
	}

	project, err := s.projectRepo.Get(ctx, ticket.ProjectID)
	if err != nil {
		return fmt.Errorf("trigger service: get project %s: %w", ticket.ProjectID, err)
	}

	run := model.PipelineRun{
		ProjectID:     ticket.ProjectID,
		TicketID:      ticket.ID,
		Orchestrator:  "soda",
		Pipeline:      resolveDefaultPipeline(project.Pipelines),
		Status:        model.RunStatusPending,
		Phases:        []model.PhaseResult{},
	}

	if err := s.pipelineSvc.Create(ctx, run); err != nil {
		return fmt.Errorf("trigger service: create pipeline run: %w", err)
	}

	slog.Info("triggered pipeline run", "ticket_id", ticket.ID, "project_id", ticket.ProjectID)
	return nil
}

// CheckAndTriggerPR is a stub for PR-based triggers. Returns nil (no-op for now).
func (s *TriggerService) CheckAndTriggerPR(ctx context.Context, pr model.PullRequest) error {
	return nil
}

// hasLabel returns true if the labels slice contains the target label.
func hasLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

// resolveDefaultPipeline returns the first pipeline name from the config,
// or "default" if none are configured.
func resolveDefaultPipeline(pipelines []model.PipelineConfig) string {
	if len(pipelines) > 0 {
		return pipelines[0].Name
	}
	return "default"
}
