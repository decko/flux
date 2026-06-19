package domain

import (
	"context"
	"fmt"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ProjectService provides business logic for project CRUD operations.
// It validates inputs before delegating to the repository and wraps
// repository errors with additional context.
type ProjectService struct {
	repo repository.ProjectRepository
}

// NewProjectService creates a new ProjectService backed by the given repository.
func NewProjectService(repo repository.ProjectRepository) *ProjectService {
	return &ProjectService{repo: repo}
}

// Create validates the project and persists it.
// Returns validation errors directly; wraps repository errors.
func (s *ProjectService) Create(ctx context.Context, p model.Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

// Get retrieves a project by ID.
// Returns ErrNotFound if the project does not exist.
func (s *ProjectService) Get(ctx context.Context, id string) (model.Project, error) {
	p, err := s.repo.Get(ctx, id)
	if err != nil {
		return model.Project{}, fmt.Errorf("get project: %w", err)
	}
	return p, nil
}

// List returns all projects matching the given filter criteria.
func (s *ProjectService) List(ctx context.Context, filter repository.ProjectFilter) ([]model.Project, error) {
	projects, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return projects, nil
}

// Update validates the project and modifies it in the store.
// Returns validation errors directly; wraps repository errors.
// Returns ErrNotFound if the project does not exist.
func (s *ProjectService) Update(ctx context.Context, p model.Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

// Delete removes a project by ID.
// Returns ErrNotFound if the project does not exist.
func (s *ProjectService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}
