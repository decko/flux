package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Mock: ProjectRepository ──────────────────────────────────────────────

type mockProjectRepo struct {
	mu    sync.Mutex
	store map[string]model.Project
}

func newMockProjectRepo() *mockProjectRepo {
	return &mockProjectRepo{store: make(map[string]model.Project)}
}

func (r *mockProjectRepo) Create(_ context.Context, p model.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[p.ID]; exists {
		return errors.New("already exists")
	}
	r.store[p.ID] = p
	return nil
}

func (r *mockProjectRepo) Get(_ context.Context, id string) (model.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, exists := r.store[id]
	if !exists {
		return model.Project{}, repository.ErrNotFound
	}
	return p, nil
}

func (r *mockProjectRepo) List(_ context.Context, _ repository.ProjectFilter) ([]model.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]model.Project, 0, len(r.store))
	for _, p := range r.store {
		result = append(result, p)
	}
	return result, nil
}

func (r *mockProjectRepo) Update(_ context.Context, p model.Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[p.ID]; !exists {
		return repository.ErrNotFound
	}
	r.store[p.ID] = p
	return nil
}

func (r *mockProjectRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[id]; !exists {
		return repository.ErrNotFound
	}
	delete(r.store, id)
	return nil
}

// ─── Mock: TicketRepository ───────────────────────────────────────────────

type mockTicketRepo struct {
	mu    sync.Mutex
	store map[string]model.Ticket
}

func newMockTicketRepo() *mockTicketRepo {
	return &mockTicketRepo{store: make(map[string]model.Ticket)}
}

func (r *mockTicketRepo) Create(_ context.Context, t model.Ticket) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[t.ID]; exists {
		return errors.New("already exists")
	}
	r.store[t.ID] = t
	return nil
}

func (r *mockTicketRepo) Get(_ context.Context, id string) (model.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, exists := r.store[id]
	if !exists {
		return model.Ticket{}, repository.ErrNotFound
	}
	return t, nil
}

func (r *mockTicketRepo) List(_ context.Context, filter repository.TicketFilter) ([]model.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []model.Ticket
	for _, t := range r.store {
		if filter.ProjectID != "" && t.ProjectID != filter.ProjectID {
			continue
		}
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.Source != "" && t.Source != filter.Source {
			continue
		}
		if len(filter.Labels) > 0 {
			match := false
			for _, want := range filter.Labels {
				for _, got := range t.Labels {
					if got == want {
						match = true
						break
					}
				}
				if match {
					break
				}
			}
			if !match {
				continue
			}
		}
		result = append(result, t)
	}
	return result, nil
}

func (r *mockTicketRepo) Update(_ context.Context, t model.Ticket) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[t.ID]; !exists {
		return repository.ErrNotFound
	}
	r.store[t.ID] = t
	return nil
}

func (r *mockTicketRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[id]; !exists {
		return repository.ErrNotFound
	}
	delete(r.store, id)
	return nil
}

// ─── Mock: PullRequestRepository ──────────────────────────────────────────

type mockPRRepo struct {
	mu    sync.Mutex
	store map[string]model.PullRequest
}

func newMockPRRepo() *mockPRRepo {
	return &mockPRRepo{store: make(map[string]model.PullRequest)}
}

func (r *mockPRRepo) Create(_ context.Context, pr model.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[pr.ID]; exists {
		return errors.New("already exists")
	}
	r.store[pr.ID] = pr
	return nil
}

func (r *mockPRRepo) Get(_ context.Context, id string) (model.PullRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pr, exists := r.store[id]
	if !exists {
		return model.PullRequest{}, repository.ErrNotFound
	}
	return pr, nil
}

func (r *mockPRRepo) List(_ context.Context, filter repository.PullRequestFilter) ([]model.PullRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []model.PullRequest
	for _, pr := range r.store {
		if filter.ProjectID != "" && pr.ProjectID != filter.ProjectID {
			continue
		}
		if filter.Status != "" && pr.Status != filter.Status {
			continue
		}
		if filter.TicketID != "" {
			match := false
			for _, tid := range pr.TicketIDs {
				if tid == filter.TicketID {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		result = append(result, pr)
	}
	return result, nil
}

func (r *mockPRRepo) Update(_ context.Context, pr model.PullRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[pr.ID]; !exists {
		return repository.ErrNotFound
	}
	r.store[pr.ID] = pr
	return nil
}

func (r *mockPRRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[id]; !exists {
		return repository.ErrNotFound
	}
	delete(r.store, id)
	return nil
}

// ─── Mock: PipelineRunRepository ──────────────────────────────────────────

type mockPipelineRunRepo struct {
	mu    sync.Mutex
	store map[string]model.PipelineRun
}

func newMockPipelineRunRepo() *mockPipelineRunRepo {
	return &mockPipelineRunRepo{store: make(map[string]model.PipelineRun)}
}

func (r *mockPipelineRunRepo) Create(_ context.Context, run model.PipelineRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[run.ID]; exists {
		return errors.New("already exists")
	}
	r.store[run.ID] = run
	return nil
}

func (r *mockPipelineRunRepo) Get(_ context.Context, id string) (model.PipelineRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, exists := r.store[id]
	if !exists {
		return model.PipelineRun{}, repository.ErrNotFound
	}
	return run, nil
}

func (r *mockPipelineRunRepo) List(_ context.Context, filter repository.PipelineRunFilter) ([]model.PipelineRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []model.PipelineRun
	for _, run := range r.store {
		if filter.ProjectID != "" && run.ProjectID != filter.ProjectID {
			continue
		}
		if filter.TicketID != "" && run.TicketID != filter.TicketID {
			continue
		}
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		result = append(result, run)
	}
	return result, nil
}

func (r *mockPipelineRunRepo) Update(_ context.Context, run model.PipelineRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.store[run.ID]; !exists {
		return repository.ErrNotFound
	}
	r.store[run.ID] = run
	return nil
}

// ─── Test Helpers ─────────────────────────────────────────────────────────

func testProject(id, name string) model.Project {
	now := time.Now().UTC().Truncate(time.Second)
	return model.Project{
		ID:      id,
		Name:    name,
		RepoURL: "https://github.com/example/" + name,
		Definition: model.ProjectDefinition{
			Language:  "Go",
			Framework: "chi",
		},
		Adapters:  []model.AdapterConfig{},
		Pipelines: []model.PipelineConfig{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func testTicket(id, projectID string, status model.TicketStatus, source model.TicketSource, labels ...string) model.Ticket {
	now := time.Now().UTC().Truncate(time.Second)
	return model.Ticket{
		ID:            id,
		ProjectID:     projectID,
		ExternalID:    "ext-" + id,
		Source:        source,
		Title:         "Ticket " + id,
		Description:   "Description for " + id,
		Status:        status,
		Labels:        labels,
		Relationships: []model.Relationship{},
		PRs:           []string{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

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

func testPipelineRun(id, projectID, ticketID string, status model.RunStatus) model.PipelineRun {
	now := time.Now().UTC().Truncate(time.Second)
	return model.PipelineRun{
		ID:           id,
		ProjectID:    projectID,
		TicketID:     ticketID,
		Orchestrator: "soda",
		Pipeline:     "dev-loop",
		Status:       status,
		Phases:       []model.PhaseResult{},
		StartedAt:    now,
		CompletedAt:  nil,
		Cost:         nil,
	}
}

// ─── ProjectRepository Tests ──────────────────────────────────────────────

func TestProjectRepository_Create(t *testing.T) {
	repo := newMockProjectRepo()
	ctx := context.Background()
	p := testProject("proj-1", "test-project")

	err := repo.Create(ctx, p)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestProjectRepository_Get(t *testing.T) {
	repo := newMockProjectRepo()
	ctx := context.Background()
	p := testProject("proj-1", "test-project")
	must(t, repo.Create(ctx, p))

	got, err := repo.Get(ctx, "proj-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("got ID %q, want %q", got.ID, p.ID)
	}
	if got.Name != p.Name {
		t.Errorf("got Name %q, want %q", got.Name, p.Name)
	}
	if got.RepoURL != p.RepoURL {
		t.Errorf("got RepoURL %q, want %q", got.RepoURL, p.RepoURL)
	}
}

func TestProjectRepository_GetNotFound(t *testing.T) {
	repo := newMockProjectRepo()
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectRepository_List(t *testing.T) {
	repo := newMockProjectRepo()
	ctx := context.Background()

	projects := []model.Project{
		testProject("p1", "project-a"),
		testProject("p2", "project-b"),
		testProject("p3", "project-c"),
	}
	for _, p := range projects {
		must(t, repo.Create(ctx, p))
	}

	result, err := repo.List(ctx, repository.ProjectFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != len(projects) {
		t.Errorf("got %d projects, want %d", len(result), len(projects))
	}
}

func TestProjectRepository_Update(t *testing.T) {
	repo := newMockProjectRepo()
	ctx := context.Background()
	p := testProject("proj-1", "original")
	must(t, repo.Create(ctx, p))

	p.Name = "updated"
	p.RepoURL = "https://github.com/example/updated"
	must(t, repo.Update(ctx, p))

	got, err := repo.Get(ctx, "proj-1")
	if err != nil {
		t.Fatalf("Get after update returned error: %v", err)
	}
	if got.Name != "updated" {
		t.Errorf("got Name %q, want %q", got.Name, "updated")
	}
	if got.RepoURL != "https://github.com/example/updated" {
		t.Errorf("got RepoURL %q, want %q", got.RepoURL, "https://github.com/example/updated")
	}
}

func TestProjectRepository_UpdateNotFound(t *testing.T) {
	repo := newMockProjectRepo()
	ctx := context.Background()
	p := testProject("nonexistent", "ghost")

	err := repo.Update(ctx, p)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectRepository_Delete(t *testing.T) {
	repo := newMockProjectRepo()
	ctx := context.Background()
	p := testProject("proj-1", "delete-me")
	must(t, repo.Create(ctx, p))

	must(t, repo.Delete(ctx, "proj-1"))

	_, err := repo.Get(ctx, "proj-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestProjectRepository_DeleteNotFound(t *testing.T) {
	repo := newMockProjectRepo()
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── TicketRepository Tests ───────────────────────────────────────────────

func TestTicketRepository_Create(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)

	err := repo.Create(ctx, tk)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestTicketRepository_Get(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)
	must(t, repo.Create(ctx, tk))

	got, err := repo.Get(ctx, "ticket-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != tk.ID {
		t.Errorf("got ID %q, want %q", got.ID, tk.ID)
	}
	if got.Title != tk.Title {
		t.Errorf("got Title %q, want %q", got.Title, tk.Title)
	}
	if got.Source != tk.Source {
		t.Errorf("got Source %q, want %q", got.Source, tk.Source)
	}
	if got.Status != tk.Status {
		t.Errorf("got Status %q, want %q", got.Status, tk.Status)
	}
}

func TestTicketRepository_GetNotFound(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTicketRepository_List(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()

	tickets := []model.Ticket{
		testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub),
		testTicket("t2", "proj-1", model.TicketStatusClosed, model.TicketSourceJira),
		testTicket("t3", "proj-2", model.TicketStatusOpen, model.TicketSourceLinear),
	}
	for _, tk := range tickets {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != len(tickets) {
		t.Errorf("got %d tickets, want %d", len(result), len(tickets))
	}
}

func TestTicketRepository_List_FilterByProject(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()

	tickets := []model.Ticket{
		testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub),
		testTicket("t2", "proj-1", model.TicketStatusClosed, model.TicketSourceJira),
		testTicket("t3", "proj-2", model.TicketStatusOpen, model.TicketSourceLinear),
	}
	for _, tk := range tickets {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d tickets, want 2", len(result))
	}
}

func TestTicketRepository_List_FilterByStatus(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()

	tickets := []model.Ticket{
		testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub),
		testTicket("t2", "proj-1", model.TicketStatusClosed, model.TicketSourceJira),
		testTicket("t3", "proj-2", model.TicketStatusOpen, model.TicketSourceLinear),
	}
	for _, tk := range tickets {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{Status: model.TicketStatusOpen})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d tickets, want 2", len(result))
	}
}

func TestTicketRepository_List_FilterBySource(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()

	tickets := []model.Ticket{
		testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub),
		testTicket("t2", "proj-1", model.TicketStatusClosed, model.TicketSourceJira),
		testTicket("t3", "proj-2", model.TicketStatusOpen, model.TicketSourceLinear),
	}
	for _, tk := range tickets {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{Source: model.TicketSourceLinear})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d tickets, want 1", len(result))
	}
	if result[0].ID != "t3" {
		t.Errorf("got ticket ID %q, want %q", result[0].ID, "t3")
	}
}

func TestTicketRepository_List_FilterByLabels(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()

	t1 := testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub, "bug", "critical")
	t2 := testTicket("t2", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub, "feature")
	t3 := testTicket("t3", "proj-2", model.TicketStatusClosed, model.TicketSourceJira, "bug")
	for _, tk := range []model.Ticket{t1, t2, t3} {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{Labels: []string{"bug"}})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d tickets, want 2", len(result))
	}
}

func TestTicketRepository_Update(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)
	must(t, repo.Create(ctx, tk))

	tk.Status = model.TicketStatusInProgress
	tk.Title = "Updated Title"
	must(t, repo.Update(ctx, tk))

	got, err := repo.Get(ctx, "ticket-1")
	if err != nil {
		t.Fatalf("Get after update returned error: %v", err)
	}
	if got.Status != model.TicketStatusInProgress {
		t.Errorf("got Status %q, want %q", got.Status, model.TicketStatusInProgress)
	}
	if got.Title != "Updated Title" {
		t.Errorf("got Title %q, want %q", got.Title, "Updated Title")
	}
}

func TestTicketRepository_UpdateNotFound(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()
	tk := testTicket("nonexistent", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)

	err := repo.Update(ctx, tk)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTicketRepository_Delete(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)
	must(t, repo.Create(ctx, tk))

	must(t, repo.Delete(ctx, "ticket-1"))

	_, err := repo.Get(ctx, "ticket-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTicketRepository_DeleteNotFound(t *testing.T) {
	repo := newMockTicketRepo()
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── PullRequestRepository Tests ──────────────────────────────────────────

func TestPullRequestRepository_Create(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")

	err := repo.Create(ctx, pr)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestPullRequestRepository_Get(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")
	must(t, repo.Create(ctx, pr))

	got, err := repo.Get(ctx, "pr-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != pr.ID {
		t.Errorf("got ID %q, want %q", got.ID, pr.ID)
	}
	if got.Title != pr.Title {
		t.Errorf("got Title %q, want %q", got.Title, pr.Title)
	}
	if got.Source != pr.Source {
		t.Errorf("got Source %q, want %q", got.Source, pr.Source)
	}
	if got.Status != pr.Status {
		t.Errorf("got Status %q, want %q", got.Status, pr.Status)
	}
}

func TestPullRequestRepository_GetNotFound(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPullRequestRepository_List(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()

	prs := []model.PullRequest{
		testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1"),
		testPullRequest("pr-2", "proj-1", model.PRStatusMerged, model.PRSourceGitLab, "t2"),
		testPullRequest("pr-3", "proj-2", model.PRStatusClosed, model.PRSourceGitHub, "t3"),
	}
	for _, pr := range prs {
		must(t, repo.Create(ctx, pr))
	}

	result, err := repo.List(ctx, repository.PullRequestFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != len(prs) {
		t.Errorf("got %d pull requests, want %d", len(result), len(prs))
	}
}

func TestPullRequestRepository_List_FilterByProject(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()

	prs := []model.PullRequest{
		testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1"),
		testPullRequest("pr-2", "proj-1", model.PRStatusMerged, model.PRSourceGitLab, "t2"),
		testPullRequest("pr-3", "proj-2", model.PRStatusClosed, model.PRSourceGitHub, "t3"),
	}
	for _, pr := range prs {
		must(t, repo.Create(ctx, pr))
	}

	result, err := repo.List(ctx, repository.PullRequestFilter{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d pull requests, want 2", len(result))
	}
}

func TestPullRequestRepository_List_FilterByStatus(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()

	prs := []model.PullRequest{
		testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1"),
		testPullRequest("pr-2", "proj-1", model.PRStatusMerged, model.PRSourceGitLab, "t2"),
		testPullRequest("pr-3", "proj-2", model.PRStatusClosed, model.PRSourceGitHub, "t3"),
	}
	for _, pr := range prs {
		must(t, repo.Create(ctx, pr))
	}

	result, err := repo.List(ctx, repository.PullRequestFilter{Status: model.PRStatusMerged})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d pull requests, want 1", len(result))
	}
	if result[0].ID != "pr-2" {
		t.Errorf("got PR ID %q, want %q", result[0].ID, "pr-2")
	}
}

func TestPullRequestRepository_List_FilterByTicketID(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()

	prs := []model.PullRequest{
		testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1", "t2"),
		testPullRequest("pr-2", "proj-1", model.PRStatusMerged, model.PRSourceGitLab, "t2"),
		testPullRequest("pr-3", "proj-2", model.PRStatusClosed, model.PRSourceGitHub, "t3"),
	}
	for _, pr := range prs {
		must(t, repo.Create(ctx, pr))
	}

	result, err := repo.List(ctx, repository.PullRequestFilter{TicketID: "t2"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d pull requests, want 2", len(result))
	}
}

func TestPullRequestRepository_Update(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1")
	must(t, repo.Create(ctx, pr))

	pr.Status = model.PRStatusMerged
	pr.Title = "Updated PR Title"
	must(t, repo.Update(ctx, pr))

	got, err := repo.Get(ctx, "pr-1")
	if err != nil {
		t.Fatalf("Get after update returned error: %v", err)
	}
	if got.Status != model.PRStatusMerged {
		t.Errorf("got Status %q, want %q", got.Status, model.PRStatusMerged)
	}
	if got.Title != "Updated PR Title" {
		t.Errorf("got Title %q, want %q", got.Title, "Updated PR Title")
	}
}

func TestPullRequestRepository_UpdateNotFound(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()
	pr := testPullRequest("nonexistent", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1")

	err := repo.Update(ctx, pr)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPullRequestRepository_Delete(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1")
	must(t, repo.Create(ctx, pr))

	must(t, repo.Delete(ctx, "pr-1"))

	_, err := repo.Get(ctx, "pr-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestPullRequestRepository_DeleteNotFound(t *testing.T) {
	repo := newMockPRRepo()
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── PipelineRunRepository Tests ──────────────────────────────────────────

func TestPipelineRunRepository_Create(t *testing.T) {
	repo := newMockPipelineRunRepo()
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)

	err := repo.Create(ctx, run)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestPipelineRunRepository_Get(t *testing.T) {
	repo := newMockPipelineRunRepo()
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, repo.Create(ctx, run))

	got, err := repo.Get(ctx, "run-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != run.ID {
		t.Errorf("got ID %q, want %q", got.ID, run.ID)
	}
	if got.Orchestrator != run.Orchestrator {
		t.Errorf("got Orchestrator %q, want %q", got.Orchestrator, run.Orchestrator)
	}
	if got.Status != run.Status {
		t.Errorf("got Status %q, want %q", got.Status, run.Status)
	}
}

func TestPipelineRunRepository_GetNotFound(t *testing.T) {
	repo := newMockPipelineRunRepo()
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPipelineRunRepository_List(t *testing.T) {
	repo := newMockPipelineRunRepo()
	ctx := context.Background()

	runs := []model.PipelineRun{
		testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending),
		testPipelineRun("run-2", "proj-1", "ticket-1", model.RunStatusRunning),
		testPipelineRun("run-3", "proj-2", "ticket-2", model.RunStatusCompleted),
	}
	for _, run := range runs {
		must(t, repo.Create(ctx, run))
	}

	result, err := repo.List(ctx, repository.PipelineRunFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != len(runs) {
		t.Errorf("got %d pipeline runs, want %d", len(result), len(runs))
	}
}

func TestPipelineRunRepository_List_FilterByProject(t *testing.T) {
	repo := newMockPipelineRunRepo()
	ctx := context.Background()

	runs := []model.PipelineRun{
		testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending),
		testPipelineRun("run-2", "proj-1", "ticket-1", model.RunStatusRunning),
		testPipelineRun("run-3", "proj-2", "ticket-2", model.RunStatusCompleted),
	}
	for _, run := range runs {
		must(t, repo.Create(ctx, run))
	}

	result, err := repo.List(ctx, repository.PipelineRunFilter{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d pipeline runs, want 2", len(result))
	}
}

func TestPipelineRunRepository_List_FilterByTicket(t *testing.T) {
	repo := newMockPipelineRunRepo()
	ctx := context.Background()

	runs := []model.PipelineRun{
		testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending),
		testPipelineRun("run-2", "proj-1", "ticket-1", model.RunStatusRunning),
		testPipelineRun("run-3", "proj-1", "ticket-2", model.RunStatusCompleted),
	}
	for _, run := range runs {
		must(t, repo.Create(ctx, run))
	}

	result, err := repo.List(ctx, repository.PipelineRunFilter{TicketID: "ticket-1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d pipeline runs, want 2", len(result))
	}
}

func TestPipelineRunRepository_List_FilterByStatus(t *testing.T) {
	repo := newMockPipelineRunRepo()
	ctx := context.Background()

	runs := []model.PipelineRun{
		testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending),
		testPipelineRun("run-2", "proj-1", "ticket-1", model.RunStatusRunning),
		testPipelineRun("run-3", "proj-2", "ticket-2", model.RunStatusCompleted),
	}
	for _, run := range runs {
		must(t, repo.Create(ctx, run))
	}

	result, err := repo.List(ctx, repository.PipelineRunFilter{Status: model.RunStatusCompleted})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d pipeline runs, want 1", len(result))
	}
	if result[0].ID != "run-3" {
		t.Errorf("got run ID %q, want %q", result[0].ID, "run-3")
	}
}

func TestPipelineRunRepository_Update(t *testing.T) {
	repo := newMockPipelineRunRepo()
	ctx := context.Background()
	run := testPipelineRun("run-1", "proj-1", "ticket-1", model.RunStatusPending)
	must(t, repo.Create(ctx, run))

	run.Status = model.RunStatusRunning
	must(t, repo.Update(ctx, run))

	got, err := repo.Get(ctx, "run-1")
	if err != nil {
		t.Fatalf("Get after update returned error: %v", err)
	}
	if got.Status != model.RunStatusRunning {
		t.Errorf("got Status %q, want %q", got.Status, model.RunStatusRunning)
	}
}

func TestPipelineRunRepository_UpdateNotFound(t *testing.T) {
	repo := newMockPipelineRunRepo()
	ctx := context.Background()
	run := testPipelineRun("nonexistent", "proj-1", "ticket-1", model.RunStatusPending)

	err := repo.Update(ctx, run)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── Utility ──────────────────────────────────────────────────────────────

// must is a test helper that fails immediately if err is non-nil.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
