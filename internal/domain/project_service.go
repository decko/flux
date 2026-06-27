package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// WebhookUpdater is the subset of GitHub webhook operations needed by
// ProjectService for webhook secret rotation. It allows callers to provide
// custom implementations for testing or alternative GitHub API backends.
type WebhookUpdater interface {
	UpdateWebhook(ctx context.Context, installationID int, owner, repo string, webhookID int, webhookURL, secret string) error
}

// ProjectService provides business logic for project CRUD operations.
// It validates inputs before delegating to the repository and wraps
// repository errors with additional context.
type ProjectService struct {
	repo       repository.ProjectRepository
	audit      *AuditService
	secretRepo webhookSecretRepo
	webhookUpd WebhookUpdater
}

// ProjectServiceOption configures a ProjectService.
type ProjectServiceOption func(*ProjectService)

// WithAuditService sets the audit service for recording project events.
func WithAuditService(audit *AuditService) ProjectServiceOption {
	return func(s *ProjectService) {
		s.audit = audit
	}
}

// WithSecretRepo sets the webhook secret repository for managing webhook secrets.
func WithSecretRepo(repo webhookSecretRepo) ProjectServiceOption {
	return func(s *ProjectService) {
		s.secretRepo = repo
	}
}

// WithWebhookUpdater sets the webhook updater for managing GitHub webhooks.
func WithWebhookUpdater(upd WebhookUpdater) ProjectServiceOption {
	return func(s *ProjectService) {
		s.webhookUpd = upd
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

// ErrNoGitHubAdapter is returned when a project has no GitHub adapter configured.
var ErrNoGitHubAdapter = fmt.Errorf("no GitHub adapter configured")

// ErrNoWebhookRegistered is returned when a project has no webhook ID.
var ErrNoWebhookRegistered = fmt.Errorf("no webhook registered")

// ErrWebhookNotConfigured is returned when the GitHub App or secret repo is
// not configured on the service.
var ErrWebhookNotConfigured = fmt.Errorf("webhook management not configured")

// ErrWebhookURLNotSet is returned when FLUX_WEBHOOK_URL is not set.
var ErrWebhookURLNotSet = fmt.Errorf("FLUX_WEBHOOK_URL not set")

// RotateWebhookSecret generates a new webhook secret, updates the GitHub
// webhook, stores the new secret, and records an audit event.
//
// It returns an error if the GitHub App or webhook secret repository is not
// configured on the service, if the project does not exist, if no webhook is
// registered, or if the GitHub API call fails.
func (s *ProjectService) RotateWebhookSecret(ctx context.Context, projectID string) error {
	if s.webhookUpd == nil || s.secretRepo == nil {
		return ErrWebhookNotConfigured
	}

	project, err := s.repo.Get(ctx, projectID)
	if err != nil {
		return fmt.Errorf("rotate webhook secret: %w", err)
	}

	webhookURL := os.Getenv("FLUX_WEBHOOK_URL")
	if webhookURL == "" {
		return ErrWebhookURLNotSet
	}
	if !strings.HasSuffix(webhookURL, "/api/v1/webhooks/github") {
		webhookURL = strings.TrimRight(webhookURL, "/") + "/api/v1/webhooks/github"
	}

	if project.WebhookID == 0 {
		return fmt.Errorf("rotate webhook secret: %w", ErrNoWebhookRegistered)
	}

	owner, repoName := getOwnerAndRepo(project)
	if owner == "" || repoName == "" {
		return fmt.Errorf("rotate webhook secret: %w", ErrNoGitHubAdapter)
	}

	secret, err := generateSecret()
	if err != nil {
		return fmt.Errorf("rotate webhook secret: %w", err)
	}

	if err := s.webhookUpd.UpdateWebhook(ctx, project.InstallationID, owner, repoName, project.WebhookID, webhookURL, secret); err != nil {
		return fmt.Errorf("rotate webhook secret: %w", err)
	}

	if err := s.secretRepo.Set(ctx, project.RepoURL, secret); err != nil {
		slog.Warn("webhook secret rotated on GitHub but failed to store locally; project may be out of sync",
			"project_id", projectID, "error", err)
		return fmt.Errorf("rotate webhook secret: store secret: %w", err)
	}

	if s.audit != nil {
		if err := s.audit.Record(ctx, model.AuditActionWebhookSecretRotated, "project", projectID, ""); err != nil {
			slog.Error("rotate webhook secret: record audit event", "error", err)
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
