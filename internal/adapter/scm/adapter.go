// Package scm defines the SCMAdapter interface for reading pull
// requests and reviews from source code management systems.
package scm

import (
	"context"

	"github.com/decko/flux/internal/model"
)

// SCMAdapter defines the interface for reading pull requests and
// reviews from source code management systems (GitHub, GitLab, etc.).
// All methods accept a context for cancellation and timeout propagation.
type SCMAdapter interface {
	// Name returns a human-readable name for this adapter (e.g. "github", "gitlab").
	Name() string

	// ListPullRequests returns all pull requests for the given project.
	ListPullRequests(ctx context.Context, projectID string) ([]model.PullRequest, error)

	// GetPullRequest retrieves a single pull request by its external ID.
	GetPullRequest(ctx context.Context, projectID, externalID string) (*model.PullRequest, error)

	// ListReviews returns all reviews for a specific pull request.
	ListReviews(ctx context.Context, projectID, externalID string) ([]model.Review, error)

	// Health checks whether the external source is reachable and responsive.
	Health(ctx context.Context) error
}
