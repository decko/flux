package domain

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

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

// ─── Tests ──────────────────────────────────────────────────────────────────

func TestNewSyncService(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	ticketAdapter := &stubTicketAdapter{name: "test-ticket"}
	scmAdapter := &stubSCMAdapter{name: "test-scm"}
	interval := 5 * time.Minute

	svc := NewSyncService(ticketRepo, prRepo, ticketAdapter, scmAdapter, interval)
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

func TestSyncService_SyncNow_Tickets(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
			sampleTicket("proj-1", "ext-2"),
		},
	}
	scmAdapter := &stubSCMAdapter{name: "test-scm"}
	svc := NewSyncService(ticketRepo, prRepo, ticketAdapter, scmAdapter, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncNow(ctx, "proj-1")
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

func TestSyncService_SyncNow_PRs(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	ticketAdapter := &stubTicketAdapter{name: "test-ticket"}
	scmAdapter := &stubSCMAdapter{
		name: "test-scm",
		prs: []model.PullRequest{
			samplePR("proj-1", "ext-1"),
			samplePR("proj-1", "ext-2"),
			samplePR("proj-1", "ext-3"),
		},
	}
	svc := NewSyncService(ticketRepo, prRepo, ticketAdapter, scmAdapter, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncNow(ctx, "proj-1")
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

func TestSyncService_SyncNow_BothAdapters(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
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
	svc := NewSyncService(ticketRepo, prRepo, ticketAdapter, scmAdapter, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncNow(ctx, "proj-1")
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

func TestSyncService_SyncNow_TicketError(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
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
	svc := NewSyncService(ticketRepo, prRepo, ticketAdapter, scmAdapter, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncNow(ctx, "proj-1")
	// SyncNow should not return an error — it logs errors per adapter.
	if err != nil {
		t.Fatalf("SyncNow should not return adapter errors, got: %v", err)
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

func TestSyncService_SyncNow_SCMError(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
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
	svc := NewSyncService(ticketRepo, prRepo, ticketAdapter, scmAdapter, 5*time.Minute)

	ctx := context.Background()
	err := svc.SyncNow(ctx, "proj-1")
	if err != nil {
		t.Fatalf("SyncNow should not return adapter errors, got: %v", err)
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

func TestSyncService_SyncNow_Upsert(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	scmAdapter := &stubSCMAdapter{name: "test-scm"}
	svc := NewSyncService(ticketRepo, prRepo, nil, scmAdapter, 5*time.Minute)

	// Inject a ticket with the same ExternalID into what the adapter returns.
	// We swap ticketAdapter for each pass to simulate the same external data.
	ctx := context.Background()

	// First sync: create ticket.
	adapter1 := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-same"),
		},
	}
	svc.TicketAdapter = adapter1
	err := svc.SyncNow(ctx, "proj-1")
	must(t, err)

	// Second sync: same ExternalID returned by adapter.
	// The service should update the existing ticket, not create a duplicate.
	adapter2 := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-same"),
		},
	}
	// Change the title to verify update behavior.
	adapter2.tickets[0].Title = "Updated Title"
	svc.TicketAdapter = adapter2
	err = svc.SyncNow(ctx, "proj-1")
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
	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
		},
	}
	scmAdapter := &stubSCMAdapter{name: "test-scm"}
	svc := NewSyncService(ticketRepo, prRepo, ticketAdapter, scmAdapter, 5*time.Minute)

	// Status before sync.
	before := svc.Status()
	if before.LastSyncAt != nil {
		t.Error("expected nil LastSyncAt before first sync")
	}

	beforeSync := time.Now()
	err := svc.SyncNow(context.Background(), "proj-1")
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
	ticketAdapter := &stubTicketAdapter{name: "test-ticket"}
	scmAdapter := &stubSCMAdapter{name: "test-scm"}
	interval := 50 * time.Millisecond
	svc := NewSyncService(ticketRepo, prRepo, ticketAdapter, scmAdapter, interval)

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

func TestSyncService_SyncNow_ContextCanceled(t *testing.T) {
	ticketRepo := newMockTicketRepo()
	prRepo := newMockPRRepo()
	ticketAdapter := &stubTicketAdapter{
		name: "test-ticket",
		tickets: []model.Ticket{
			sampleTicket("proj-1", "ext-1"),
		},
	}
	scmAdapter := &stubSCMAdapter{name: "test-scm"}
	svc := NewSyncService(ticketRepo, prRepo, ticketAdapter, scmAdapter, 5*time.Minute)

	// Create a canceled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.SyncNow(ctx, "proj-1")
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
