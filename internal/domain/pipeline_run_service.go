package domain

import (
	"context"
	"fmt"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// PipelineRunService provides business logic for pipeline run CRUD operations.
// It validates inputs before delegating to the repository and wraps
// repository errors with additional context.
// Pipeline runs are immutable records; there is no Delete method.
type PipelineRunService struct {
	repo repository.PipelineRunRepository
}

// NewPipelineRunService creates a new PipelineRunService backed by the given repository.
func NewPipelineRunService(repo repository.PipelineRunRepository) *PipelineRunService {
	return &PipelineRunService{repo: repo}
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
