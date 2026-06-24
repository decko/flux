package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// PullRequestService provides business logic for pull request CRUD operations.
// It validates inputs before delegating to the repository and wraps
// repository errors with additional context.
type PullRequestService struct {
	repo  repository.PullRequestRepository
	audit *AuditService
}

// PullRequestServiceOption configures a PullRequestService.
type PullRequestServiceOption func(*PullRequestService)

// WithPullRequestAuditService sets the audit service for recording pull
// request events.
func WithPullRequestAuditService(audit *AuditService) PullRequestServiceOption {
	return func(s *PullRequestService) {
		s.audit = audit
	}
}

// NewPullRequestService creates a new PullRequestService backed by the given
// repository.
func NewPullRequestService(repo repository.PullRequestRepository, opts ...PullRequestServiceOption) *PullRequestService {
	s := &PullRequestService{repo: repo}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Create validates the pull request and persists it.
// Returns validation errors directly; wraps repository errors.
func (s *PullRequestService) Create(ctx context.Context, pr model.PullRequest) error {
	if err := pr.Validate(); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, pr); err != nil {
		return fmt.Errorf("create pull request: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.Record(ctx, "pull_request.created", "pull_request", pr.ID, marshalPullRequest(pr)); err != nil {
			return fmt.Errorf("create pull request: %w", err)
		}
	}
	return nil
}

// Get retrieves a pull request by ID.
// Returns ErrNotFound if the pull request does not exist.
func (s *PullRequestService) Get(ctx context.Context, id string) (model.PullRequest, error) {
	pr, err := s.repo.Get(ctx, id)
	if err != nil {
		return model.PullRequest{}, fmt.Errorf("get pull request: %w", err)
	}
	return pr, nil
}

// List returns all pull requests matching the given filter criteria.
func (s *PullRequestService) List(ctx context.Context, filter repository.PullRequestFilter) ([]model.PullRequest, error) {
	prs, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list pull requests: %w", err)
	}
	return prs, nil
}

// Update validates the pull request and modifies it in the store.
// Returns validation errors directly; wraps repository errors.
// Returns ErrNotFound if the pull request does not exist.
func (s *PullRequestService) Update(ctx context.Context, pr model.PullRequest) error {
	if err := pr.Validate(); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, pr); err != nil {
		return fmt.Errorf("update pull request: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.Record(ctx, "pull_request.updated", "pull_request", pr.ID, marshalPullRequest(pr)); err != nil {
			return fmt.Errorf("update pull request: %w", err)
		}
	}
	return nil
}

// Delete removes a pull request by ID.
// Returns ErrNotFound if the pull request does not exist.
func (s *PullRequestService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete pull request: %w", err)
	}
	if s.audit != nil {
		if err := s.audit.Record(ctx, "pull_request.deleted", "pull_request", id, ""); err != nil {
			return fmt.Errorf("delete pull request: %w", err)
		}
	}
	return nil
}

// marshalPullRequest serializes a pull request to a JSON string.
// If marshaling fails, it returns an empty string.
func marshalPullRequest(pr model.PullRequest) string {
	b, err := json.Marshal(pr)
	if err != nil {
		return ""
	}
	return string(b)
}
