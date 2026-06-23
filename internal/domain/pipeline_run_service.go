package domain

import (
	"context"
	"fmt"

	"github.com/decko/flux/internal/adapter/orchestrator"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// PipelineRunService provides business logic for pipeline run CRUD operations.
// It validates inputs before delegating to the repository and wraps
// repository errors with additional context.
// Pipeline runs are immutable records; there is no Delete method.
type PipelineRunService struct {
	repo         repository.PipelineRunRepository
	orchestrator *orchestrator.OrchestratorAdapter
}

// PipelineRunServiceOption configures a PipelineRunService.
type PipelineRunServiceOption func(*PipelineRunService)

// WithOrchestrator sets the orchestrator adapter for trigger and cancel operations.
func WithOrchestrator(adapter orchestrator.OrchestratorAdapter) PipelineRunServiceOption {
	return func(s *PipelineRunService) {
		s.orchestrator = &adapter
	}
}

// NewPipelineRunService creates a new PipelineRunService backed by the given repository.
func NewPipelineRunService(repo repository.PipelineRunRepository, opts ...PipelineRunServiceOption) *PipelineRunService {
	s := &PipelineRunService{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create validates the pipeline run and persists it.
// Returns validation errors directly; wraps repository errors.
func (s *PipelineRunService) Create(ctx context.Context, run model.PipelineRun) error {
	if err := run.Validate(); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, run); err != nil {
		return fmt.Errorf("create pipeline run: %w", err)
	}
	return nil
}

// Get retrieves a pipeline run by ID.
// Returns ErrNotFound if the pipeline run does not exist.
func (s *PipelineRunService) Get(ctx context.Context, id string) (model.PipelineRun, error) {
	run, err := s.repo.Get(ctx, id)
	if err != nil {
		return model.PipelineRun{}, fmt.Errorf("get pipeline run: %w", err)
	}
	return run, nil
}

// List returns all pipeline runs matching the given filter criteria.
func (s *PipelineRunService) List(ctx context.Context, filter repository.PipelineRunFilter) ([]model.PipelineRun, error) {
	runs, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list pipeline runs: %w", err)
	}
	return runs, nil
}

// Update validates the pipeline run and modifies it in the store.
// Returns validation errors directly; wraps repository errors.
// Returns ErrNotFound if the pipeline run does not exist.
func (s *PipelineRunService) Update(ctx context.Context, run model.PipelineRun) error {
	if err := run.Validate(); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, run); err != nil {
		return fmt.Errorf("update pipeline run: %w", err)
	}
	return nil
}

// Trigger initiates execution of a pipeline run by notifying the orchestrator.
// It fetches the run by ID, delegates to the orchestrator's Trigger method,
// sets the run status to running, and persists the update.
// Returns ErrNotFound if the pipeline run does not exist.
// Returns an error if no orchestrator adapter is configured.
func (s *PipelineRunService) Trigger(ctx context.Context, runID string) error {
	return s.TriggerWithTicketID(ctx, runID, "")
}

// TriggerWithTicketID starts execution of a pipeline run, using the given external
// ticket ID when passing to the orchestrator (soda expects a GitHub issue number).
func (s *PipelineRunService) TriggerWithTicketID(ctx context.Context, runID, externalTicketID string) error {
	if s.orchestrator == nil {
		return fmt.Errorf("orchestrator not configured")
	}
	run, err := s.repo.Get(ctx, runID)
	if err != nil {
		return fmt.Errorf("trigger pipeline run: %w", err)
	}
	// Use the external ticket ID if provided (soda expects GitHub issue numbers).
	if externalTicketID != "" {
		run.TicketID = externalTicketID
	}
	if err := (*s.orchestrator).Trigger(ctx, run); err != nil {
		return fmt.Errorf("trigger pipeline run: %w", err)
	}
	run.Status = model.RunStatusRunning
	if err := s.repo.Update(ctx, run); err != nil {
		return fmt.Errorf("trigger pipeline run: %w", err)
	}
	return nil
}

// Cancel stops execution of a pipeline run by notifying the orchestrator.
// It fetches the run by ID, delegates to the orchestrator's Cancel method,
// sets the run status to canceled, and persists the update.
// Returns ErrNotFound if the pipeline run does not exist.
// Returns an error if no orchestrator adapter is configured.
func (s *PipelineRunService) Cancel(ctx context.Context, runID string) error {
	if s.orchestrator == nil {
		return fmt.Errorf("orchestrator not configured")
	}
	run, err := s.repo.Get(ctx, runID)
	if err != nil {
		return fmt.Errorf("cancel pipeline run: %w", err)
	}
	if err := (*s.orchestrator).Cancel(ctx, runID); err != nil {
		return fmt.Errorf("cancel pipeline run: %w", err)
	}
	run.Status = model.RunStatusCanceled
	if err := s.repo.Update(ctx, run); err != nil {
		return fmt.Errorf("cancel pipeline run: %w", err)
	}
	return nil
}
