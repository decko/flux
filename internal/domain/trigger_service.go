package domain

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/decko/flux/internal/model"
)

type triggerRunner interface {
	Create(ctx context.Context, run model.PipelineRun) error
}

type triggerProjectRepo interface {
	Get(ctx context.Context, id string) (model.Project, error)
}

type triggerRunRepo interface {
	HasActiveRun(ctx context.Context, projectID, ticketID string) (bool, error)
}

type triggerRuleRepo interface {
	ListByProject(ctx context.Context, projectID string) ([]model.TriggerRule, error)
}

// TriggerService evaluates tickets against trigger rules and automatically
// creates pipeline runs when conditions match. Rules are loaded from the
// trigger rule repository; if none are configured for a project, the
// hardcoded default (ticket labeled "flux/agent" → default pipeline) is used.
type TriggerService struct {
	pipelineSvc  triggerRunner
	projectRepo  triggerProjectRepo
	runRepo      triggerRunRepo
	triggerRules triggerRuleRepo
	selfUser     string
}

// NewTriggerService creates a new TriggerService.
func NewTriggerService(
	pipelineSvc triggerRunner,
	projectRepo triggerProjectRepo,
	runRepo triggerRunRepo,
	triggerRules triggerRuleRepo,
	selfUser string,
) *TriggerService {
	return &TriggerService{
		pipelineSvc:  pipelineSvc,
		projectRepo:  projectRepo,
		runRepo:      runRepo,
		triggerRules: triggerRules,
		selfUser:     selfUser,
	}
}

// CheckAndTrigger evaluates trigger rules against a ticket. If the ticket
// matches a rule, the resolved pipeline is validated against the project's
// configured pipelines, and if no active run exists, a new pipeline run is
// created.
func (s *TriggerService) CheckAndTrigger(ctx context.Context, ticket model.Ticket) error {
	pipelineName, err := s.matchRules(ctx, ticket)
	if err != nil {
		return fmt.Errorf("trigger service: match rules: %w", err)
	}
	if pipelineName == "" {
		return nil
	}

	// Validate the resolved pipeline exists in project config.
	project, err := s.projectRepo.Get(ctx, ticket.ProjectID)
	if err != nil {
		return fmt.Errorf("trigger service: get project: %w", err)
	}

	var found bool
	for _, p := range project.Pipelines {
		if p.Name == pipelineName {
			found = true
			break
		}
	}
	if !found {
		slog.Info("skipping trigger: pipeline not configured for project",
			"pipeline", pipelineName, "project_id", ticket.ProjectID)
		return nil
	}

	active, err := s.runRepo.HasActiveRun(ctx, ticket.ProjectID, ticket.ID)
	if err != nil {
		return fmt.Errorf("trigger service: check active runs: %w", err)
	}
	if active {
		slog.Info("skipping trigger: active run already exists", "ticket_id", ticket.ID)
		return nil
	}

	run := model.PipelineRun{
		ProjectID:    ticket.ProjectID,
		TicketID:     ticket.ID,
		Orchestrator: "soda",
		Pipeline:     pipelineName,
		Status:       model.RunStatusPending,
		Phases:       []model.PhaseResult{},
	}

	if err := s.pipelineSvc.Create(ctx, run); err != nil {
		return fmt.Errorf("trigger service: create pipeline run: %w", err)
	}

	slog.Info("triggered pipeline run", "ticket_id", ticket.ID, "pipeline", pipelineName)
	return nil
}

// CheckAndTriggerPR is a stub for PR-based triggers.
func (s *TriggerService) CheckAndTriggerPR(ctx context.Context, pr model.PullRequest) error {
	return nil
}

// matchRules queries the trigger rule repository for enabled rules matching
// the ticket's project. Rules are ordered by priority; the highest-priority
// rule whose label matches a ticket label determines the pipeline.
// Falls back to the hardcoded default if no rules exist for the project.
func (s *TriggerService) matchRules(ctx context.Context, ticket model.Ticket) (string, error) {
	rules, err := s.triggerRules.ListByProject(ctx, ticket.ProjectID)
	if err != nil {
		return "", fmt.Errorf("list rules: %w", err)
	}

	// Fallback to hardcoded default if no rules are configured.
	if len(rules) == 0 {
		if hasLabel(ticket.Labels, "flux/agent") {
			return "default", nil
		}
		return "", nil
	}

	// Find the highest-priority enabled rule whose label matches the ticket.
	var bestPipeline string
	bestPriority := -1
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if hasLabel(ticket.Labels, rule.Label) && rule.Priority > bestPriority {
			bestPriority = rule.Priority
			bestPipeline = rule.Pipeline
		}
	}
	return bestPipeline, nil
}

func hasLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}
