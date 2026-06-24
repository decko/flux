package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ProjectService provides business logic for project CRUD operations.
// It validates inputs before delegating to the repository and wraps
// repository errors with additional context.
type ProjectService struct {
	repo  repository.ProjectRepository
	audit *AuditService
}

// ProjectServiceOption configures a ProjectService.
type ProjectServiceOption func(*ProjectService)

// WithAuditService sets the audit service for recording project events.
func WithAuditService(audit *AuditService) ProjectServiceOption {
	return func(s *ProjectService) {
		s.audit = audit
	}
}

// NewProjectService creates a new ProjectService backed by the given repository.
func NewProjectService(repo repository.ProjectRepository, opts ...ProjectServiceOption) *ProjectService {
	s := &ProjectService{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
	if s.audit != nil {
		if err := s.audit.Record(ctx, "project.created", "project", p.ID, marshalProject(p)); err != nil {
			return fmt.Errorf("create project: %w", err)
		}
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
	if s.audit != nil {
		if err := s.audit.Record(ctx, "project.updated", "project", p.ID, marshalProject(p)); err != nil {
			return fmt.Errorf("update project: %w", err)
		}
	}
	return nil
}

// Delete removes a project by ID.
// Returns ErrNotFound if the project does not exist.
func (s *ProjectService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.Record(ctx, "project.deleted", "project", id, ""); err != nil {
			return fmt.Errorf("delete project: %w", err)
		}
	}
	return nil
}

// marshalProject serializes a project to a JSON string.
// If marshaling fails, it returns an empty string.
func marshalProject(p model.Project) string {
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(b)
}
