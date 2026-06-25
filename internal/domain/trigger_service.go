package domain

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/decko/flux/internal/config"
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

// TriggerService evaluates tickets against trigger rules and automatically
// creates pipeline runs when conditions match. Rules are loaded from
// configuration via WithTriggerRules; if none are configured, the hardcoded
// default (ticket labeled "flux/agent" → default pipeline) is used.
type TriggerService struct {
	pipelineSvc triggerRunner
	projectRepo triggerProjectRepo
	runRepo     triggerRunRepo
	selfUser    string
	rules       []config.TriggerRule
}

// TriggerServiceOption configures a TriggerService.
type TriggerServiceOption func(*TriggerService)

// WithTriggerRules sets the trigger rules from configuration.
func WithTriggerRules(rules []config.TriggerRule) TriggerServiceOption {
	return func(s *TriggerService) {
		s.rules = rules
	}
}

// NewTriggerService creates a new TriggerService.
func NewTriggerService(
	pipelineSvc triggerRunner,
	projectRepo triggerProjectRepo,
	runRepo triggerRunRepo,
	selfUser string,
	opts ...TriggerServiceOption,
) *TriggerService {
	s := &TriggerService{
		pipelineSvc: pipelineSvc,
		projectRepo: projectRepo,
		runRepo:     runRepo,
		selfUser:    selfUser,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CheckAndTrigger evaluates trigger rules against a ticket. If the ticket
// matches a rule and no active run exists, a new pipeline run is created.
func (s *TriggerService) CheckAndTrigger(ctx context.Context, ticket model.Ticket) error {
	pipelineName := s.matchRules(ticket)
	if pipelineName == "" {
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

// matchRules evaluates configured rules against a ticket. Falls back to
// the hardcoded default if no rules are configured.
func (s *TriggerService) matchRules(ticket model.Ticket) string {
	if len(s.rules) == 0 {
		if hasLabel(ticket.Labels, "flux/agent") {
			return "default"
		}
		return ""
	}

	for _, rule := range s.rules {
		if rule.Event != "ticket.labeled" {
			continue
		}
		if !hasAllLabels(ticket.Labels, rule.Labels) {
			continue
		}
		if rule.Pipeline != "" {
			return rule.Pipeline
		}
		return "default"
	}
	return ""
}

func hasLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

func hasAllLabels(ticketLabels, required []string) bool {
	for _, r := range required {
		if !hasLabel(ticketLabels, r) {
			return false
		}
	}
	return true
}
