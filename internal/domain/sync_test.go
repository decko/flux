package domain

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/decko/flux/internal/adapter/scm"
	"github.com/decko/flux/internal/adapter/ticket"
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
	var result []model.Project
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

// ─── Mock: TicketRepository ─────────────────────────────────────────────────

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

// ─── Mock: PullRequestRepository ────────────────────────────────────────────

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

// ─── Stub: TicketAdapter ────────────────────────────────────────────────────

type stubTicketAdapter struct {
	name    string
	tickets []model.Ticket
	listErr error
}

func (a *stubTicketAdapter) Name() string { return a.name }

func (a *stubTicketAdapter) ListTickets(_ context.Context, _ string) ([]model.Ticket, error) {
	return a.tickets, a.listErr
}

func (a *stubTicketAdapter) GetTicket(_ context.Context, _, _ string) (*model.Ticket, error) {
	return nil, errors.New("not implemented")
}

func (a *stubTicketAdapter) CreateTicket(_ context.Context, _ *model.Ticket) (*model.Ticket, error) {
	return nil, errors.New("not implemented")
}

func (a *stubTicketAdapter) UpdateTicket(_ context.Context, _ *model.Ticket) error {
	return errors.New("not implemented")
}

func (a *stubTicketAdapter) SyncRelationships(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (a *stubTicketAdapter) Health(_ context.Context) error {
	return nil
}

// ─── Stub: SCMAdapter ───────────────────────────────────────────────────────

type stubSCMAdapter struct {
	name    string
	prs     []model.PullRequest
	listErr error
}

func (a *stubSCMAdapter) Name() string { return a.name }

func (a *stubSCMAdapter) ListPullRequests(_ context.Context, _ string) ([]model.PullRequest, error) {
	return a.prs, a.listErr
}

func (a *stubSCMAdapter) GetPullRequest(_ context.Context, _, _ string) (*model.PullRequest, error) {
	return nil, errors.New("not implemented")
}

func (a *stubSCMAdapter) ListReviews(_ context.Context, _, _ string) ([]model.Review, error) {
	return nil, errors.New("not implemented")
}

func (a *stubSCMAdapter) Health(_ context.Context) error {
	return nil
}

// ─── Test Helpers ───────────────────────────────────────────────────────────

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func sampleTicket(projectID, externalID string) model.Ticket {
	now := time.Now().UTC().Truncate(time.Second)
	return model.Ticket{
		ProjectID:     projectID,
		ExternalID:    externalID,
		Source:        model.TicketSourceGitHub,
		Title:         "Ticket " + externalID,
		Description:   "Description for " + externalID,
		Status:        model.TicketStatusOpen,
		Labels:        []string{},
		Relationships: []model.Relationship{},
		PRs:           []string{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func samplePR(projectID, externalID string) model.PullRequest {
	now := time.Now().UTC().Truncate(time.Second)
	return model.PullRequest{
		ProjectID:  projectID,
		ExternalID: externalID,
		Source:     model.PRSourceGitHub,
		Title:      "PR " + externalID,
		URL:        "https://github.com/example/repo/pull/" + externalID,
		Status:     model.PRStatusOpen,
		TicketIDs:  []string{},
		Reviews:    []model.Review{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// mustCreateProject inserts a project into the mock project repo. Fails the
// test on error.
func mustCreateProject(t *testing.T, repo *mockProjectRepo, p model.Project) {
	t.Helper()
	if err := repo.Create(context.Background(), p); err != nil {
		t.Fatalf("create project: %v", err)
	}
}

// ─── Tests ──────────────────────────────────────────────────────────────────

func TestNewSyncService(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	ticketStub := &stubTicketAdapter{name: "test-ticket"}
	scmStub := &stubSCMAdapter{name: "test-scm"}
	interval := 5 * time.Minute

	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(_ string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketStub, scmStub, nil
		}, interval)
	if svc == nil {
		t.Fatal("NewSyncService returned nil")
	}

	// Status should be zero-valued before any sync.
	status := svc.Status()
	if status.LastSyncAt != nil {
		t.Errorf("expected nil LastSyncAt, got %v", *status.LastSyncAt)
	}
	if status.LastSyncError != "" {
		t.Errorf("expected empty LastSyncError, got %q", status.LastSyncError)
	}
	if status.TicketsSynced != 0 {
		t.Errorf("expected TicketsSynced 0, got %d", status.TicketsSynced)
	}
	if status.PRsSynced != 0 {
		t.Errorf("expected PRsSynced 0, got %d", status.PRsSynced)
	}
}

func TestSyncService_SyncProject_Tickets(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
			sampleTicket("proj-1", "ext-2"),
		},
	}
	scmAdapter := &stubSCMAdapter{name: "test-scm"}
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketAdapter, scmAdapter, nil
		}, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncProject(ctx, "proj-1")
	must(t, err)

	// Verify both tickets were created in the repo.
	list, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(list) != 2 {
		t.Fatalf("got %d tickets, want 2", len(list))
	}

	// Verify IDs were assigned (not empty).
	for _, tk := range list {
		if tk.ID == "" {
			t.Error("expected non-empty ticket ID after sync")
		}
		if tk.ExternalID == "" {
			t.Error("expected non-empty ExternalID")
		}
		if tk.ProjectID != "proj-1" {
			t.Errorf("got ProjectID %q, want %q", tk.ProjectID, "proj-1")
		}
	}

	// Verify status.
	status := svc.Status()
	if status.TicketsSynced != 2 {
		t.Errorf("got TicketsSynced %d, want 2", status.TicketsSynced)
	}
	if status.PRsSynced != 0 {
		t.Errorf("got PRsSynced %d, want 0", status.PRsSynced)
	}
	if status.LastSyncAt == nil {
		t.Error("expected non-zero LastSyncAt")
	}
}

func TestSyncService_SyncProject_PRs(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketAdapter := &stubTicketAdapter{name: "test-ticket"}
	scmAdapter := &stubSCMAdapter{
		name: "test-scm",
		prs: []model.PullRequest{
			samplePR("proj-1", "ext-1"),
			samplePR("proj-1", "ext-2"),
			samplePR("proj-1", "ext-3"),
		},
	}
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketAdapter, scmAdapter, nil
		}, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncProject(ctx, "proj-1")
	must(t, err)

	list, err := prRepo.List(ctx, repository.PullRequestFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(list) != 3 {
		t.Fatalf("got %d PRs, want 3", len(list))
	}

	for _, pr := range list {
		if pr.ID == "" {
			t.Error("expected non-empty PR ID after sync")
		}
		if pr.ExternalID == "" {
			t.Error("expected non-empty ExternalID")
		}
	}

	status := svc.Status()
	if status.PRsSynced != 3 {
		t.Errorf("got PRsSynced %d, want 3", status.PRsSynced)
	}
	if status.TicketsSynced != 0 {
		t.Errorf("got TicketsSynced %d, want 0", status.TicketsSynced)
	}
}

func TestSyncService_SyncProject_BothAdapters(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
			sampleTicket("proj-1", "ext-2"),
		},
	}
	scmAdapter := &stubSCMAdapter{
		name: "test-scm",
		prs: []model.PullRequest{
			samplePR("proj-1", "ext-1"),
		},
	}
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketAdapter, scmAdapter, nil
		}, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncProject(ctx, "proj-1")
	must(t, err)

	tickets, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(tickets) != 2 {
		t.Errorf("got %d tickets, want 2", len(tickets))
	}

	prs, err := prRepo.List(ctx, repository.PullRequestFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(prs) != 1 {
		t.Errorf("got %d PRs, want 1", len(prs))
	}

	status := svc.Status()
	if status.TicketsSynced != 2 {
		t.Errorf("got TicketsSynced %d, want 2", status.TicketsSynced)
	}
	if status.PRsSynced != 1 {
		t.Errorf("got PRsSynced %d, want 1", status.PRsSynced)
	}
}

func TestSyncService_SyncProject_TicketError(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketAdapter := &stubTicketAdapter{
		name:    "test-ticket",
		listErr: errors.New("ticket API error"),
	}
	scmAdapter := &stubSCMAdapter{
		name: "test-scm",
		prs: []model.PullRequest{
			samplePR("proj-1", "ext-1"),
		},
	}
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketAdapter, scmAdapter, nil
		}, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncProject(ctx, "proj-1")
	// SyncProject should not return an error — it logs errors per adapter.
	if err != nil {
		t.Fatalf("SyncProject should not return adapter errors, got: %v", err)
	}

	// PRs should still be synced despite ticket error.
	prs, err := prRepo.List(ctx, repository.PullRequestFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(prs) != 1 {
		t.Errorf("got %d PRs, want 1 (PRs should sync despite ticket error)", len(prs))
	}

	// No tickets should exist.
	tickets, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(tickets) != 0 {
		t.Errorf("got %d tickets, want 0", len(tickets))
	}

	status := svc.Status()
	if status.TicketsSynced != 0 {
		t.Errorf("got TicketsSynced %d, want 0", status.TicketsSynced)
	}
	if status.PRsSynced != 1 {
		t.Errorf("got PRsSynced %d, want 1", status.PRsSynced)
	}
	if status.LastSyncError == "" {
		t.Error("expected LastSyncError to contain the ticket error")
	}
}

func TestSyncService_SyncProject_SCMError(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
		},
	}
	scmAdapter := &stubSCMAdapter{
		name:    "test-scm",
		listErr: errors.New("SCM API error"),
	}
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketAdapter, scmAdapter, nil
		}, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncProject(ctx, "proj-1")
	if err != nil {
		t.Fatalf("SyncProject should not return adapter errors, got: %v", err)
	}

	// Tickets should still be synced despite SCM error.
	tickets, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(tickets) != 1 {
		t.Errorf("got %d tickets, want 1 (tickets should sync despite SCM error)", len(tickets))
	}

	status := svc.Status()
	if status.TicketsSynced != 1 {
		t.Errorf("got TicketsSynced %d, want 1", status.TicketsSynced)
	}
	if status.PRsSynced != 0 {
		t.Errorf("got PRsSynced %d, want 0", status.PRsSynced)
	}
	if status.LastSyncError == "" {
		t.Error("expected LastSyncError to contain the SCM error")
	}
}

func TestSyncService_SyncProject_Upsert(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	scmAdapter := &stubSCMAdapter{name: "test-scm"}

	// Use a closure variable so we can swap the ticket adapter.
	var curTicketAdapter *stubTicketAdapter
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			// Return the adapter even if nil — the core sync handles nil gracefully.
			return curTicketAdapter, scmAdapter, nil
		}, 5*time.Minute)

	ctx := context.Background()

	// First sync: create ticket.
	curTicketAdapter = &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-same"),
		},
	}
	err := svc.SyncProject(ctx, "proj-1")
	must(t, err)

	// Second sync: same ExternalID returned by adapter.
	// The service should update the existing ticket, not create a duplicate.
	curTicketAdapter = &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-same"),
		},
	}
	// Change the title to verify update behavior.
	curTicketAdapter.tickets[0].Title = "Updated Title"
	err = svc.SyncProject(ctx, "proj-1")
	must(t, err)

	// Verify only 1 ticket exists for this project (not 2).
	list, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(list) != 1 {
		t.Fatalf("got %d tickets, want 1 (upsert should not duplicate)", len(list))
	}

	// Verify the ticket has the updated title.
	if list[0].Title != "Updated Title" {
		t.Errorf("got Title %q, want %q (expected update)", list[0].Title, "Updated Title")
	}

	// Status should reflect only 1 ticket synced in the second pass
	// (the first pass already counted 1).
	status := svc.Status()
	if status.TicketsSynced != 1 {
		t.Errorf("got TicketsSynced %d, want 1", status.TicketsSynced)
	}
}

func TestSyncService_Status(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
		},
	}
	scmAdapter := &stubSCMAdapter{name: "test-scm"}
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketAdapter, scmAdapter, nil
		}, 5*time.Minute)

	// Status before sync.
	before := svc.Status()
	if before.LastSyncAt != nil {
		t.Error("expected nil LastSyncAt before first sync")
	}

	beforeSync := time.Now()
	err := svc.SyncProject(context.Background(), "proj-1")
	must(t, err)
	afterSync := time.Now()

	after := svc.Status()
	if after.LastSyncAt == nil {
		t.Error("expected non-zero LastSyncAt after sync")
	}
	if after.LastSyncAt != nil && ((*after.LastSyncAt).Before(beforeSync) || (*after.LastSyncAt).After(afterSync)) {
		t.Errorf("LastSyncAt %v should be between %v and %v", *after.LastSyncAt, beforeSync, afterSync)
	}
	if after.TicketsSynced != 1 {
		t.Errorf("got TicketsSynced %d, want 1", after.TicketsSynced)
	}
	if after.PRsSynced != 0 {
		t.Errorf("got PRsSynced %d, want 0", after.PRsSynced)
	}
	if after.LastSyncError != "" {
		t.Errorf("got LastSyncError %q, want empty", after.LastSyncError)
	}
}

func TestSyncService_RunStop(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketStub := &stubTicketAdapter{name: "test-ticket"}
	scmStub := &stubSCMAdapter{name: "test-scm"}
	interval := 50 * time.Millisecond

	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(_ string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketStub, scmStub, nil
		}, interval)

	// Run in background goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		svc.Run(ctx)
		close(done)
	}()

	// Let it run for a couple of tick intervals.
	time.Sleep(150 * time.Millisecond)

	// Stop should return cleanly (no hang).
	svc.Stop()

	// Verify Run exited by checking done channel with timeout.
	select {
	case <-done:
		// OK: goroutine exited.
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit within 2 seconds after Stop")
	}
}

func TestSyncService_SyncProject_ContextCanceled(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
		},
	}
	scmAdapter := &stubSCMAdapter{name: "test-scm"}
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketAdapter, scmAdapter, nil
		}, 5*time.Minute)

	// Create a canceled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.SyncProject(ctx, "proj-1")
	if err == nil {
		t.Fatal("expected error when context is canceled, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// Verify no tickets were created.
	list, listErr := ticketRepo.List(context.Background(), repository.TicketFilter{ProjectID: "proj-1"})
	must(t, listErr)
	if len(list) != 0 {
		t.Errorf("got %d tickets, want 0 (nothing should be created on canceled context)", len(list))
	}
}

// ─── New Tests: Per-Project Sync ────────────────────────────────────────────

func TestSyncService_SyncNow_MultipleProjects(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()

	// Register two projects.
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Project One"})
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-2", Name: "Project Two"})

	// Per-project adapter configs.
	type projectAdapters struct {
		ticket *stubTicketAdapter
		scm    *stubSCMAdapter
	}
	adapters := map[string]projectAdapters{
		"proj-1": {
			ticket: &stubTicketAdapter{
				name: "github",
				tickets: []model.Ticket{
					sampleTicket("proj-1", "ext-1"),
					sampleTicket("proj-1", "ext-2"),
				},
			},
			scm: &stubSCMAdapter{
				name: "github",
				prs: []model.PullRequest{
					samplePR("proj-1", "ext-101"),
				},
			},
		},
		"proj-2": {
			ticket: &stubTicketAdapter{
				name: "jira",
				tickets: []model.Ticket{
					sampleTicket("proj-2", "ext-3"),
				},
			},
			scm: &stubSCMAdapter{
				name: "github",
				prs: []model.PullRequest{
					samplePR("proj-2", "ext-102"),
					samplePR("proj-2", "ext-103"),
				},
			},
		},
	}

	factory := func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
		a, ok := adapters[projectID]
		if !ok {
			return nil, nil, errors.New("unknown project")
		}
		return a.ticket, a.scm, nil
	}

	svc := NewSyncService(ticketRepo, prRepo, projectRepo, factory, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncNow(ctx)
	must(t, err)

	// Verify proj-1 tickets.
	p1Tickets, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(p1Tickets) != 2 {
		t.Errorf("proj-1: got %d tickets, want 2", len(p1Tickets))
	}

	// Verify proj-1 PRs.
	p1PRs, err := prRepo.List(ctx, repository.PullRequestFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(p1PRs) != 1 {
		t.Errorf("proj-1: got %d PRs, want 1", len(p1PRs))
	}

	// Verify proj-2 tickets.
	p2Tickets, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-2"})
	must(t, err)
	if len(p2Tickets) != 1 {
		t.Errorf("proj-2: got %d tickets, want 1", len(p2Tickets))
	}

	// Verify proj-2 PRs.
	p2PRs, err := prRepo.List(ctx, repository.PullRequestFilter{ProjectID: "proj-2"})
	must(t, err)
	if len(p2PRs) != 2 {
		t.Errorf("proj-2: got %d PRs, want 2", len(p2PRs))
	}

	// Verify aggregate status includes all projects.
	status := svc.Status()
	if status.TicketsSynced != 3 {
		t.Errorf("got TicketsSynced %d, want 3 (2+1)", status.TicketsSynced)
	}
	if status.PRsSynced != 3 {
		t.Errorf("got PRsSynced %d, want 3 (1+2)", status.PRsSynced)
	}
	if status.LastSyncAt == nil {
		t.Error("expected non-zero LastSyncAt")
	}
}

func TestSyncService_SyncProject_Isolation(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()

	// Register two projects.
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Project One"})
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-2", Name: "Project Two"})

	// Both projects have valid adapters but we only sync proj-1.
	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
		},
	}
	scmAdapter := &stubSCMAdapter{name: "test-scm"}

	factoryCallCount := 0
	factory := func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
		factoryCallCount++
		if projectID == "proj-1" {
			return ticketAdapter, scmAdapter, nil
		}
		return nil, nil, errors.New("should not be called")
	}

	svc := NewSyncService(ticketRepo, prRepo, projectRepo, factory, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncProject(ctx, "proj-1")
	must(t, err)

	// Verify only proj-1 data exists.
	tickets, err := ticketRepo.List(ctx, repository.TicketFilter{})
	must(t, err)
	if len(tickets) != 1 {
		t.Errorf("got %d total tickets, want 1", len(tickets))
	}
	for _, tk := range tickets {
		if tk.ProjectID != "proj-1" {
			t.Errorf("expected only proj-1 tickets, got project %q", tk.ProjectID)
		}
	}

	// Factory should only have been called for the synced project.
	if factoryCallCount != 1 {
		t.Errorf("factory called %d times, want 1", factoryCallCount)
	}

	// Aggregate status should reflect only proj-1.
	status := svc.Status()
	if status.TicketsSynced != 1 {
		t.Errorf("got TicketsSynced %d, want 1", status.TicketsSynced)
	}
}

func TestSyncService_SyncProject_NotFound(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()

	// Register one project, but try to sync a different one.
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Project One"})

	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return nil, nil, errors.New("should not be called")
		}, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncProject(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown project, got nil")
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// No data should have been synced.
	tickets, listErr := ticketRepo.List(ctx, repository.TicketFilter{})
	must(t, listErr)
	if len(tickets) != 0 {
		t.Errorf("got %d tickets, want 0", len(tickets))
	}
}

func TestSyncService_SyncNow_SkipProject_NoToken(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()

	// Register two projects: proj-1 has credentials, proj-2 does not.
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "With Token"})
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-2", Name: "No Token"})

	factory := func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
		if projectID == "proj-2" {
			return nil, nil, errors.New("no credentials for project")
		}
		return &stubTicketAdapter{
			name: "test-ticket",
			tickets: []model.Ticket{
				sampleTicket("proj-1", "ext-1"),
			},
		}, &stubSCMAdapter{name: "test-scm"}, nil
	}

	svc := NewSyncService(ticketRepo, prRepo, projectRepo, factory, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncNow(ctx)
	must(t, err)

	// Verify proj-1 was synced.
	p1Tickets, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(p1Tickets) != 1 {
		t.Errorf("proj-1: got %d tickets, want 1", len(p1Tickets))
	}

	// Verify proj-2 was skipped (no data).
	p2Tickets, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-2"})
	must(t, err)
	if len(p2Tickets) != 0 {
		t.Errorf("proj-2: got %d tickets, want 0 (project should be skipped)", len(p2Tickets))
	}

	p2PRs, err := prRepo.List(ctx, repository.PullRequestFilter{ProjectID: "proj-2"})
	must(t, err)
	if len(p2PRs) != 0 {
		t.Errorf("proj-2: got %d PRs, want 0 (project should be skipped)", len(p2PRs))
	}

	// Aggregate should only reflect proj-1.
	status := svc.Status()
	if status.TicketsSynced != 1 {
		t.Errorf("got TicketsSynced %d, want 1", status.TicketsSynced)
	}
	if status.PRsSynced != 0 {
		t.Errorf("got PRsSynced %d, want 0", status.PRsSynced)
	}
}

func TestSyncService_SyncNow_ProjectError_Isolation(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()

	// Register two projects.
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Failing Project"})
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-2", Name: "Healthy Project"})

	type projectAdapters struct {
		ticket *stubTicketAdapter
		scm    *stubSCMAdapter
	}
	adapters := map[string]projectAdapters{
		"proj-1": {
			ticket: &stubTicketAdapter{
				name:    "failing-ticket",
				listErr: errors.New("ticket API unavailable"),
			},
			scm: &stubSCMAdapter{
				name:    "failing-scm",
				listErr: errors.New("SCM API unavailable"),
			},
		},
		"proj-2": {
			ticket: &stubTicketAdapter{
				name: "healthy-ticket",
				tickets: []model.Ticket{
					sampleTicket("proj-2", "ext-1"),
				},
			},
			scm: &stubSCMAdapter{
				name: "healthy-scm",
				prs: []model.PullRequest{
					samplePR("proj-2", "ext-101"),
				},
			},
		},
	}

	factory := func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
		a, ok := adapters[projectID]
		if !ok {
			return nil, nil, errors.New("unknown project")
		}
		return a.ticket, a.scm, nil
	}

	svc := NewSyncService(ticketRepo, prRepo, projectRepo, factory, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncNow(ctx)
	must(t, err)

	// proj-1 failed: no tickets or PRs should exist.
	p1Tickets, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(p1Tickets) != 0 {
		t.Errorf("proj-1: got %d tickets, want 0 (adapter error should prevent sync)", len(p1Tickets))
	}
	p1PRs, err := prRepo.List(ctx, repository.PullRequestFilter{ProjectID: "proj-1"})
	must(t, err)
	if len(p1PRs) != 0 {
		t.Errorf("proj-1: got %d PRs, want 0 (adapter error should prevent sync)", len(p1PRs))
	}

	// proj-2 synced successfully.
	p2Tickets, err := ticketRepo.List(ctx, repository.TicketFilter{ProjectID: "proj-2"})
	must(t, err)
	if len(p2Tickets) != 1 {
		t.Errorf("proj-2: got %d tickets, want 1", len(p2Tickets))
	}
	p2PRs, err := prRepo.List(ctx, repository.PullRequestFilter{ProjectID: "proj-2"})
	must(t, err)
	if len(p2PRs) != 1 {
		t.Errorf("proj-2: got %d PRs, want 1", len(p2PRs))
	}

	// Aggregate should reflect only proj-2 (the successful one).
	status := svc.Status()
	if status.TicketsSynced != 1 {
		t.Errorf("got TicketsSynced %d, want 1", status.TicketsSynced)
	}
	if status.PRsSynced != 1 {
		t.Errorf("got PRsSynced %d, want 1", status.PRsSynced)
	}
	// LastSyncError should be set (from proj-1's failure).
	if status.LastSyncError == "" {
		t.Error("expected LastSyncError to be set from project error")
	}
}

func TestSyncService_PerProjectStatus(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()

	// Register two projects.
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Project One"})
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-2", Name: "Project Two"})

	type projectAdapters struct {
		ticket *stubTicketAdapter
		scm    *stubSCMAdapter
	}
	adapters := map[string]projectAdapters{
		"proj-1": {
			ticket: &stubTicketAdapter{
				name: "github",
				tickets: []model.Ticket{
					sampleTicket("proj-1", "ext-1"),
					sampleTicket("proj-1", "ext-2"),
				},
			},
			scm: &stubSCMAdapter{
				name: "github",
				prs: []model.PullRequest{
					samplePR("proj-1", "ext-101"),
				},
			},
		},
		"proj-2": {
			ticket: &stubTicketAdapter{
				name: "jira",
				tickets: []model.Ticket{
					sampleTicket("proj-2", "ext-3"),
				},
			},
			scm: &stubSCMAdapter{
				name: "github",
				prs: []model.PullRequest{
					samplePR("proj-2", "ext-102"),
					samplePR("proj-2", "ext-103"),
				},
			},
		},
	}

	factory := func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
		a, ok := adapters[projectID]
		if !ok {
			return nil, nil, errors.New("unknown project")
		}
		return a.ticket, a.scm, nil
	}

	svc := NewSyncService(ticketRepo, prRepo, projectRepo, factory, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncNow(ctx)
	must(t, err)

	status := svc.Status()

	// Verify per-project status for proj-1.
	ps1, ok := status.Projects["proj-1"]
	if !ok {
		t.Fatal("expected per-project status for proj-1")
	}
	if ps1.TicketsSynced != 2 {
		t.Errorf("proj-1 TicketsSynced: got %d, want 2", ps1.TicketsSynced)
	}
	if ps1.PRsSynced != 1 {
		t.Errorf("proj-1 PRsSynced: got %d, want 1", ps1.PRsSynced)
	}
	if ps1.LastSyncAt == nil {
		t.Error("proj-1: expected non-nil LastSyncAt")
	}
	if ps1.LastSyncError != "" {
		t.Errorf("proj-1: unexpected LastSyncError %q", ps1.LastSyncError)
	}

	// Verify per-project status for proj-2.
	ps2, ok := status.Projects["proj-2"]
	if !ok {
		t.Fatal("expected per-project status for proj-2")
	}
	if ps2.TicketsSynced != 1 {
		t.Errorf("proj-2 TicketsSynced: got %d, want 1", ps2.TicketsSynced)
	}
	if ps2.PRsSynced != 2 {
		t.Errorf("proj-2 PRsSynced: got %d, want 2", ps2.PRsSynced)
	}
	if ps2.LastSyncAt == nil {
		t.Error("proj-2: expected non-nil LastSyncAt")
	}
	if ps2.LastSyncError != "" {
		t.Errorf("proj-2: unexpected LastSyncError %q", ps2.LastSyncError)
	}

	// Verify aggregate status matches sum.
	if status.TicketsSynced != 3 {
		t.Errorf("aggregate TicketsSynced: got %d, want 3", status.TicketsSynced)
	}
	if status.PRsSynced != 3 {
		t.Errorf("aggregate PRsSynced: got %d, want 3", status.PRsSynced)
	}
}

// ─── Mock: AuditRepository (captures events) ─────────────────────────────

type captureAuditRepo struct {
	mu     sync.Mutex
	events []model.AuditEvent
	latest *model.AuditEvent
}

func newCaptureAuditRepo() *captureAuditRepo {
	return &captureAuditRepo{events: make([]model.AuditEvent, 0)}
}

func (r *captureAuditRepo) Insert(_ context.Context, e model.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	r.latest = &e
	return nil
}

func (r *captureAuditRepo) List(_ context.Context, _ repository.AuditFilter) ([]model.AuditEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]model.AuditEvent, len(r.events))
	copy(result, r.events)
	// Return in descending order to match SQLiteAuditRepository behavior.
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result, nil
}

func (r *captureAuditRepo) Latest(_ context.Context) (*model.AuditEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.latest, nil
}

func (r *captureAuditRepo) PurgeOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

// ─── Tests: Sync Audit Events ────────────────────────────────────────────

func TestSyncService_AuditEvent_TicketCreate(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	auditRepo := newCaptureAuditRepo()
	auditSvc := NewAuditService(auditRepo)
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
		},
	}
	scmAdapter := &stubSCMAdapter{name: "test-scm"}
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketAdapter, scmAdapter, nil
		}, 5*time.Minute)
	svc.WithSyncAuditService(auditSvc)

	ctx := context.Background()
	err := svc.SyncProject(ctx, "proj-1")
	must(t, err)

	// Verify an audit event was recorded for ticket creation via sync.
	events, err := auditRepo.List(ctx, repository.AuditFilter{})
	must(t, err)
	if len(events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(events))
	}
	if events[0].Action != model.AuditActionTicketCreatedSync {
		t.Errorf("action = %q, want %q", events[0].Action, model.AuditActionTicketCreatedSync)
	}
	if events[0].ResourceType != "ticket" {
		t.Errorf("resource_type = %q, want %q", events[0].ResourceType, "ticket")
	}
	if events[0].Metadata != "origin=sync" {
		t.Errorf("metadata = %q, want %q", events[0].Metadata, "origin=sync")
	}
	// Sync events should have a system actor indicating the sync origin.
	if events[0].ActorID != "system:sync" {
		t.Errorf("actor_id = %q, want %q", events[0].ActorID, "system:sync")
	}
}

func TestSyncService_AuditEvent_TicketUpdate(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	auditRepo := newCaptureAuditRepo()
	auditSvc := NewAuditService(auditRepo)
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	scmAdapter := &stubSCMAdapter{name: "test-scm"}

	// Pre-seed a ticket in the repo.
	existing := sampleTicket("proj-1", "ext-1")
	existing.ID = model.TicketID(existing.Source, existing.ExternalID)
	now := time.Now()
	existing.CreatedAt = now
	existing.UpdatedAt = now
	if err := ticketRepo.Create(context.Background(), existing); err != nil {
		t.Fatalf("seed ticket: %v", err)
	}

	var curAdapter *stubTicketAdapter
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return curAdapter, scmAdapter, nil
		}, 5*time.Minute)
	svc.WithSyncAuditService(auditSvc)

	// Sync with same ticket but different title to trigger update.
	updated := sampleTicket("proj-1", "ext-1")
	updated.Title = "Updated Title"
	curAdapter = &stubTicketAdapter{
		name:    "test-ticket",
		tickets: []model.Ticket{updated},
	}

	ctx := context.Background()
	err := svc.SyncProject(ctx, "proj-1")
	must(t, err)

	// Should have one audit event for the update.
	events, err := auditRepo.List(ctx, repository.AuditFilter{})
	must(t, err)
	if len(events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(events))
	}
	if events[0].Action != model.AuditActionTicketUpdatedSync {
		t.Errorf("action = %q, want %q", events[0].Action, model.AuditActionTicketUpdatedSync)
	}
	if events[0].Metadata != "origin=sync" {
		t.Errorf("metadata = %q, want %q", events[0].Metadata, "origin=sync")
	}
}

func TestSyncService_AuditEvent_PRCreate(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	auditRepo := newCaptureAuditRepo()
	auditSvc := NewAuditService(auditRepo)
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketAdapter := &stubTicketAdapter{name: "test-ticket"}
	scmAdapter := &stubSCMAdapter{
		name: "test-scm",
		prs: []model.PullRequest{
			samplePR("proj-1", "ext-1"),
		},
	}
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketAdapter, scmAdapter, nil
		}, 5*time.Minute)
	svc.WithSyncAuditService(auditSvc)

	ctx := context.Background()
	err := svc.SyncProject(ctx, "proj-1")
	must(t, err)

	// Verify an audit event was recorded for PR creation via sync.
	events, err := auditRepo.List(ctx, repository.AuditFilter{})
	must(t, err)
	if len(events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(events))
	}
	if events[0].Action != model.AuditActionPRCreatedSync {
		t.Errorf("action = %q, want %q", events[0].Action, model.AuditActionPRCreatedSync)
	}
	if events[0].ResourceType != "pull_request" {
		t.Errorf("resource_type = %q, want %q", events[0].ResourceType, "pull_request")
	}
	if events[0].Metadata != "origin=sync" {
		t.Errorf("metadata = %q, want %q", events[0].Metadata, "origin=sync")
	}
	if events[0].ActorID != "system:sync" {
		t.Errorf("actor_id = %q, want %q", events[0].ActorID, "system:sync")
	}
}

func TestSyncService_AuditEvent_MultipleTicketsAndPRs(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	projectRepo := newMockProjectRepo()
	auditRepo := newCaptureAuditRepo()
	auditSvc := NewAuditService(auditRepo)
	mustCreateProject(t, projectRepo, model.Project{ID: "proj-1", Name: "Test Project"})

	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
			sampleTicket("proj-1", "ext-2"),
		},
	}
	scmAdapter := &stubSCMAdapter{
		name: "test-scm",
		prs: []model.PullRequest{
			samplePR("proj-1", "ext-101"),
		},
	}
	svc := NewSyncService(ticketRepo, prRepo, projectRepo,
		func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error) {
			return ticketAdapter, scmAdapter, nil
		}, 5*time.Minute)
	svc.WithSyncAuditService(auditSvc)

	ctx := context.Background()
	err := svc.SyncProject(ctx, "proj-1")
	must(t, err)

	events, err := auditRepo.List(ctx, repository.AuditFilter{})
	must(t, err)
	// Should have 3 events: 2 ticket creates + 1 PR create.
	if len(events) != 3 {
		t.Fatalf("got %d audit events, want 3 (2 ticket creates + 1 PR create)", len(events))
	}

	// Count by action type.
	var ticketCreates, prCreates int
	for _, e := range events {
		switch e.Action {
		case model.AuditActionTicketCreatedSync:
			ticketCreates++
		case model.AuditActionPRCreatedSync:
			prCreates++
		}
	}
	if ticketCreates != 2 {
		t.Errorf("got %d ticket.created.sync events, want 2", ticketCreates)
	}
	if prCreates != 1 {
		t.Errorf("got %d pull_request.created.sync events, want 1", prCreates)
	}
}
