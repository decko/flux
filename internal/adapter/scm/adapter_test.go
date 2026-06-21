package scm

import (
	"context"
	"errors"
	"testing"

	"github.com/decko/flux/internal/adapter"
	"github.com/decko/flux/internal/model"
)

// StubSCMAdapter is a no-op stub that satisfies SCMAdapter.
// Each method returns zero values or ErrNotImplemented.
type StubSCMAdapter struct{}

func (s *StubSCMAdapter) Name() string { return "test-stub" }

func (s *StubSCMAdapter) ListPullRequests(ctx context.Context, projectID string) ([]model.PullRequest, error) {
	return nil, adapter.ErrNotImplemented
}

func (s *StubSCMAdapter) GetPullRequest(ctx context.Context, projectID, externalID string) (*model.PullRequest, error) {
	return nil, adapter.ErrNotImplemented
}

func (s *StubSCMAdapter) ListReviews(ctx context.Context, projectID, externalID string) ([]model.Review, error) {
	return nil, adapter.ErrNotImplemented
}

func (s *StubSCMAdapter) Health(ctx context.Context) error {
	return adapter.ErrNotImplemented
}

// Compile-time check: StubSCMAdapter satisfies SCMAdapter.
var _ SCMAdapter = (*StubSCMAdapter)(nil)

func TestStubSatisfiesSCMAdapter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T, a SCMAdapter)
	}{
		{
			name: "Name",
			run: func(t *testing.T, a SCMAdapter) {
				got := a.Name()
				if got != "test-stub" {
					t.Errorf("Name() = %q, want %q", got, "test-stub")
				}
			},
		},
		{
			name: "ListPullRequests returns ErrNotImplemented",
			run: func(t *testing.T, a SCMAdapter) {
				prs, err := a.ListPullRequests(context.Background(), "proj-1")
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Fatalf("ListPullRequests err = %v, want %v", err, adapter.ErrNotImplemented)
				}
				if len(prs) != 0 {
					t.Errorf("ListPullRequests returned %d PRs, want 0", len(prs))
				}
			},
		},
		{
			name: "GetPullRequest returns ErrNotImplemented",
			run: func(t *testing.T, a SCMAdapter) {
				got, err := a.GetPullRequest(context.Background(), "proj-1", "ext-1")
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Fatalf("GetPullRequest err = %v, want %v", err, adapter.ErrNotImplemented)
				}
				if got != nil {
					t.Errorf("GetPullRequest returned non-nil PR: %v", got)
				}
			},
		},
		{
			name: "ListReviews returns ErrNotImplemented",
			run: func(t *testing.T, a SCMAdapter) {
				reviews, err := a.ListReviews(context.Background(), "proj-1", "ext-1")
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Fatalf("ListReviews err = %v, want %v", err, adapter.ErrNotImplemented)
				}
				if len(reviews) != 0 {
					t.Errorf("ListReviews returned %d reviews, want 0", len(reviews))
				}
			},
		},
		{
			name: "Health returns ErrNotImplemented",
			run: func(t *testing.T, a SCMAdapter) {
				err := a.Health(context.Background())
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Fatalf("Health err = %v, want %v", err, adapter.ErrNotImplemented)
				}
			},
		},
	}

	adapter := &StubSCMAdapter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t, adapter)
		})
	}
}
