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

// ─── Mock: TicketRepository ────────────────────────────────────────────────

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

func (r *mockTicketRepo) List(_ context.Context, _ repository.TicketFilter) ([]model.Ticket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]model.Ticket, 0, len(r.store))
	for _, t := range r.store {
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

// ─── Test Helpers ──────────────────────────────────────────────────────────

func testTicket(id, projectID string) model.Ticket {
	now := time.Now().UTC().Truncate(time.Second)
	return model.Ticket{
		ID:            id,
		ProjectID:     projectID,
		ExternalID:    "ext-" + id,
		Source:        model.TicketSourceGitHub,
		Title:         "Ticket " + id,
		Description:   "Description for " + id,
		Status:        model.TicketStatusOpen,
		Labels:        []string{},
		Relationships: []model.Relationship{},
		PRs:           []string{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// ─── TicketService Tests ───────────────────────────────────────────────────

func TestTicketService_Create(t *testing.T) {
	repo := newMockTicketRepo()
	svc := domain.NewTicketService(repo)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1")

	err := svc.Create(ctx, tk)
	must(t, err)

	// Verify it was stored in the repo.
	got, err := repo.Get(ctx, "ticket-1")
	must(t, err)
	if got.ID != tk.ID {
		t.Errorf("got ID %q, want %q", got.ID, tk.ID)
	}
	if got.Title != tk.Title {
		t.Errorf("got Title %q, want %q", got.Title, tk.Title)
	}
}

func TestTicketService_Create_Invalid(t *testing.T) {
	repo := newMockTicketRepo()
	svc := domain.NewTicketService(repo)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1")
	tk.Title = "" // empty title — invalid

	err := svc.Create(ctx, tk)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatal("expected validation error, not ErrNotFound")
	}

	// Verify the mock was NOT called (ticket should not be stored).
	_, getErr := repo.Get(ctx, "ticket-1")
	if !errors.Is(getErr, repository.ErrNotFound) {
		t.Fatal("ticket was stored in repo despite validation failure")
	}
}

func TestTicketService_Get(t *testing.T) {
	repo := newMockTicketRepo()
	svc := domain.NewTicketService(repo)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1")
	must(t, svc.Create(ctx, tk))

	got, err := svc.Get(ctx, "ticket-1")
	must(t, err)
	if got.ID != tk.ID {
		t.Errorf("got ID %q, want %q", got.ID, tk.ID)
	}
	if got.Title != tk.Title {
		t.Errorf("got Title %q, want %q", got.Title, tk.Title)
	}
	if got.ProjectID != tk.ProjectID {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, tk.ProjectID)
	}
	if got.Status != tk.Status {
		t.Errorf("got Status %q, want %q", got.Status, tk.Status)
	}
}

func TestTicketService_Get_NotFound(t *testing.T) {
	repo := newMockTicketRepo()
	svc := domain.NewTicketService(repo)
	ctx := context.Background()

	_, err := svc.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTicketService_List(t *testing.T) {
	repo := newMockTicketRepo()
	svc := domain.NewTicketService(repo)
	ctx := context.Background()

	tickets := []model.Ticket{
		testTicket("t1", "proj-a"),
		testTicket("t2", "proj-b"),
		testTicket("t3", "proj-c"),
	}
	for _, tk := range tickets {
		must(t, svc.Create(ctx, tk))
	}

	result, err := svc.List(ctx, repository.TicketFilter{})
	must(t, err)
	if len(result) != len(tickets) {
		t.Fatalf("got %d tickets, want %d", len(result), len(tickets))
	}

	// Verify all IDs are present.
	ids := make(map[string]bool)
	for _, tk := range result {
		ids[tk.ID] = true
	}
	for _, tk := range tickets {
		if !ids[tk.ID] {
			t.Errorf("missing ticket %q in results", tk.ID)
		}
	}
}

func TestTicketService_Update(t *testing.T) {
	repo := newMockTicketRepo()
	svc := domain.NewTicketService(repo)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1")
	must(t, svc.Create(ctx, tk))

	tk.Title = "Updated Title"
	tk.Status = model.TicketStatusInProgress
	must(t, svc.Update(ctx, tk))

	got, err := svc.Get(ctx, "ticket-1")
	must(t, err)
	if got.Title != "Updated Title" {
		t.Errorf("got Title %q, want %q", got.Title, "Updated Title")
	}
	if got.Status != model.TicketStatusInProgress {
		t.Errorf("got Status %q, want %q", got.Status, model.TicketStatusInProgress)
	}
}

func TestTicketService_Update_Invalid(t *testing.T) {
	repo := newMockTicketRepo()
	svc := domain.NewTicketService(repo)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1")
	must(t, svc.Create(ctx, tk))

	tk.Title = "" // invalid
	err := svc.Update(ctx, tk)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatal("expected validation error, not ErrNotFound")
	}

	// Verify the ticket was NOT modified in the store.
	got, getErr := repo.Get(ctx, "ticket-1")
	must(t, getErr)
	if got.Title != "Ticket ticket-1" {
		t.Errorf("ticket title changed despite validation failure: got %q, want %q", got.Title, "Ticket ticket-1")
	}
}

func TestTicketService_Update_NotFound(t *testing.T) {
	repo := newMockTicketRepo()
	svc := domain.NewTicketService(repo)
	ctx := context.Background()
	tk := testTicket("nonexistent", "proj-1")

	err := svc.Update(ctx, tk)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTicketService_Delete(t *testing.T) {
	repo := newMockTicketRepo()
	svc := domain.NewTicketService(repo)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1")
	must(t, svc.Create(ctx, tk))

	must(t, svc.Delete(ctx, "ticket-1"))

	_, err := svc.Get(ctx, "ticket-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestTicketService_Delete_NotFound(t *testing.T) {
	repo := newMockTicketRepo()
	svc := domain.NewTicketService(repo)
	ctx := context.Background()

	err := svc.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
