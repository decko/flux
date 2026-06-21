// Package scm defines the SCMAdapter interface for reading pull
// requests and reviews from source code management systems.
package scm

import (
	"context"
	"net/http"
	"strings"

	"github.com/decko/flux/internal/adapter"
	"github.com/decko/flux/internal/adapter/github"
	"github.com/decko/flux/internal/model"
)

// SCMAdapter defines the interface for reading pull requests and
// reviews from source code management systems (GitHub, GitLab, etc.).
// All methods accept a context for cancellation and timeout propagation.
type SCMAdapter interface {
	// Name returns a human-readable name for this adapter (e.g. "github", "gitlab").
	Name() string

	// ListPullRequests returns all pull requests for the given project.
	// The Reviews field of each PullRequest is not populated; use
	// ListReviews to retrieve review data for a specific pull request.
	ListPullRequests(ctx context.Context, projectID string) ([]model.PullRequest, error)

	// GetPullRequest retrieves a single pull request by its external ID.
	GetPullRequest(ctx context.Context, projectID, externalID string) (*model.PullRequest, error)

	// ListReviews returns all reviews for a specific pull request.
	ListReviews(ctx context.Context, projectID, externalID string) ([]model.Review, error)

	// Health checks whether the external source is reachable and responsive.
	Health(ctx context.Context) error
}

// StubSCMAdapter is a no-op stub that satisfies SCMAdapter.
// Each method returns zero values or ErrNotImplemented.
type StubSCMAdapter struct{}

func (s *StubSCMAdapter) Name() string { return "test-stub" }

func (s *StubSCMAdapter) ListPullRequests(_ context.Context, _ string) ([]model.PullRequest, error) {
	return nil, adapter.ErrNotImplemented
}

func (s *StubSCMAdapter) GetPullRequest(_ context.Context, _, _ string) (*model.PullRequest, error) {
	return nil, adapter.ErrNotImplemented
}

func (s *StubSCMAdapter) ListReviews(_ context.Context, _, _ string) ([]model.Review, error) {
	return nil, adapter.ErrNotImplemented
}

func (s *StubSCMAdapter) Health(_ context.Context) error {
	return adapter.ErrNotImplemented
}

// Compile-time check: StubSCMAdapter satisfies SCMAdapter.
var _ SCMAdapter = (*StubSCMAdapter)(nil)

// GitHubSCMAdapter implements SCMAdapter for the GitHub REST API v3.
type GitHubSCMAdapter struct {
	owner    string
	repo     string
	ghClient *github.Client
	baseURL  string
}

// GitHubSCMOption configures a GitHubSCMAdapter.
type GitHubSCMOption func(*GitHubSCMAdapter)

// WithBaseURL sets the base URL for the GitHub API. Used for testing with
// httptest.Server or GitHub Enterprise.
func WithBaseURL(baseURL string) GitHubSCMOption {
	return func(a *GitHubSCMAdapter) {
		a.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// NewGitHubAdapter creates a new GitHubSCMAdapter. If httpClient is nil,
// http.DefaultClient is used. The default base URL is https://api.github.com.
// Functional options (e.g. WithBaseURL) can be provided to override defaults.
func NewGitHubAdapter(owner, repo, token string, httpClient *http.Client, opts ...GitHubSCMOption) *GitHubSCMAdapter {
	a := &GitHubSCMAdapter{
		owner:    owner,
		repo:     repo,
		ghClient: github.NewClient(token, httpClient),
		baseURL:  "https://api.github.com",
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Name returns "github".
func (a *GitHubSCMAdapter) Name() string { return "github" }
