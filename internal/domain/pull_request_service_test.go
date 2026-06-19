package domain_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Mock: PullRequestRepository ───────────────────────────────────────────

type mockPullRequestRepo struct {
	mu    sync.Mutex
	store map[string]model.PullRequest
}

func newMockPullRequestRepo() *mockPullRequestRepo {
	return &mockPullRequestRepo{store: make(map[string]model.PullRequest)}
}

func (r *mockPullRequestRepo) Create(_ context.Context, pr model.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[pr.ID]; exists {
		return errors.New("already exists")
	}
	r.store[pr.ID] = pr
	return nil
}

func (r *mockPullRequestRepo) Get(_ context.Context, id string) (model.PullRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pr, exists := r.store[id]
	if !exists {
		return model.PullRequest{}, repository.ErrNotFound
	}
	return pr, nil
}

func (r *mockPullRequestRepo) List(_ context.Context, _ repository.PullRequestFilter) ([]model.PullRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]model.PullRequest, 0, len(r.store))
	for _, pr := range r.store {
		result = append(result, pr)
	}
	return result, nil
}

func (r *mockPullRequestRepo) Update(_ context.Context, pr model.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[pr.ID]; !exists {
		return repository.ErrNotFound
	}
	r.store[pr.ID] = pr
	return nil
}

func (r *mockPullRequestRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[id]; !exists {
		return repository.ErrNotFound
	}
	delete(r.store, id)
	return nil
}

// ─── Test Helpers ──────────────────────────────────────────────────────────

func testPullRequest(id, projectID string, status model.PRStatus, source model.PRSource, ticketIDs ...string) model.PullRequest {
	now := time.Now().UTC().Truncate(time.Second)
	return model.PullRequest{
		ID:         id,
		ProjectID:  projectID,
		ExternalID: "ext-" + id,
		Source:     source,
		Title:      "PR " + id,
		URL:        "https://github.com/example/repo/pull/" + id,
		Status:     status,
		TicketIDs:  ticketIDs,
		Reviews:    []model.Review{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// ─── PullRequestService Tests ──────────────────────────────────────────────

func TestPullRequestService_Create(t *testing.T) {
	repo := newMockPullRequestRepo()
	svc := domain.NewPullRequestService(repo)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")

	err := svc.Create(ctx, pr)
	must(t, err)

	// Verify it was stored in the repo.
	got, err := repo.Get(ctx, "pr-1")
	must(t, err)
	if got.ID != pr.ID {
		t.Errorf("got ID %q, want %q", got.ID, pr.ID)
	}
	if got.Title != pr.Title {
		t.Errorf("got Title %q, want %q", got.Title, pr.Title)
	}
}

func TestPullRequestService_Create_Invalid(t *testing.T) {
	repo := newMockPullRequestRepo()
	svc := domain.NewPullRequestService(repo)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")
	pr.Title = "" // empty title — invalid

	err := svc.Create(ctx, pr)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatal("expected validation error, not ErrNotFound")
	}

	// Verify the mock was NOT called (PR should not be stored).
	_, getErr := repo.Get(ctx, "pr-1")
	if !errors.Is(getErr, repository.ErrNotFound) {
		t.Fatal("PR was stored in repo despite validation failure")
	}
}

func TestPullRequestService_Get(t *testing.T) {
	repo := newMockPullRequestRepo()
	svc := domain.NewPullRequestService(repo)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")
	must(t, svc.Create(ctx, pr))

	got, err := svc.Get(ctx, "pr-1")
	must(t, err)
	if got.ID != pr.ID {
		t.Errorf("got ID %q, want %q", got.ID, pr.ID)
	}
	if got.Title != pr.Title {
		t.Errorf("got Title %q, want %q", got.Title, pr.Title)
	}
	if got.ProjectID != pr.ProjectID {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, pr.ProjectID)
	}
	if got.Status != pr.Status {
		t.Errorf("got Status %q, want %q", got.Status, pr.Status)
	}
}

func TestPullRequestService_Get_NotFound(t *testing.T) {
	repo := newMockPullRequestRepo()
	svc := domain.NewPullRequestService(repo)
	ctx := context.Background()

	_, err := svc.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPullRequestService_List(t *testing.T) {
	repo := newMockPullRequestRepo()
	svc := domain.NewPullRequestService(repo)
	ctx := context.Background()

	prs := []model.PullRequest{
		testPullRequest("pr-1", "proj-a", model.PRStatusOpen, model.PRSourceGitHub, "t1"),
		testPullRequest("pr-2", "proj-b", model.PRStatusMerged, model.PRSourceGitLab, "t2"),
		testPullRequest("pr-3", "proj-c", model.PRStatusClosed, model.PRSourceGitHub, "t3"),
	}
	for _, pr := range prs {
		must(t, svc.Create(ctx, pr))
	}

	result, err := svc.List(ctx, repository.PullRequestFilter{})
	must(t, err)
	if len(result) != len(prs) {
		t.Fatalf("got %d PRs, want %d", len(result), len(prs))
	}

	// Verify all IDs are present.
	ids := make(map[string]bool)
	for _, pr := range result {
		ids[pr.ID] = true
	}
	for _, pr := range prs {
		if !ids[pr.ID] {
			t.Errorf("missing PR %q in results", pr.ID)
		}
	}
}

func TestPullRequestService_Update(t *testing.T) {
	repo := newMockPullRequestRepo()
	svc := domain.NewPullRequestService(repo)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")
	must(t, svc.Create(ctx, pr))

	pr.Title = "Updated PR Title"
	pr.Status = model.PRStatusMerged
	must(t, svc.Update(ctx, pr))

	got, err := svc.Get(ctx, "pr-1")
	must(t, err)
	if got.Title != "Updated PR Title" {
		t.Errorf("got Title %q, want %q", got.Title, "Updated PR Title")
	}
	if got.Status != model.PRStatusMerged {
		t.Errorf("got Status %q, want %q", got.Status, model.PRStatusMerged)
	}
}

func TestPullRequestService_Update_Invalid(t *testing.T) {
	repo := newMockPullRequestRepo()
	svc := domain.NewPullRequestService(repo)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")
	must(t, svc.Create(ctx, pr))

	pr.Title = "" // invalid
	err := svc.Update(ctx, pr)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatal("expected validation error, not ErrNotFound")
	}

	// Verify the PR was NOT modified in the store.
	got, getErr := repo.Get(ctx, "pr-1")
	must(t, getErr)
	if got.Title != "PR pr-1" {
		t.Errorf("PR title changed despite validation failure: got %q, want %q", got.Title, "PR pr-1")
	}
}

func TestPullRequestService_Update_NotFound(t *testing.T) {
	repo := newMockPullRequestRepo()
	svc := domain.NewPullRequestService(repo)
	ctx := context.Background()
	pr := testPullRequest("nonexistent", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")

	err := svc.Update(ctx, pr)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPullRequestService_Delete(t *testing.T) {
	repo := newMockPullRequestRepo()
	svc := domain.NewPullRequestService(repo)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")
	must(t, svc.Create(ctx, pr))

	must(t, svc.Delete(ctx, "pr-1"))

	_, err := svc.Get(ctx, "pr-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestPullRequestService_Delete_NotFound(t *testing.T) {
	repo := newMockPullRequestRepo()
	svc := domain.NewPullRequestService(repo)
	ctx := context.Background()

	err := svc.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
