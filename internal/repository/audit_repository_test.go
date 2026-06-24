package repository_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Mock: AuditRepository ──────────────────────────────────────────────────

type mockAuditRepo struct {
	mu    sync.Mutex
	store []model.AuditEvent
}

func newMockAuditRepo() *mockAuditRepo {
	return &mockAuditRepo{store: make([]model.AuditEvent, 0)}
}

func (r *mockAuditRepo) Insert(_ context.Context, event model.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store = append(r.store, event)
	return nil
}

func (r *mockAuditRepo) List(_ context.Context, filter repository.AuditFilter) ([]model.AuditEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []model.AuditEvent
	for _, e := range r.store {
		if filter.ActorID != "" && e.ActorID != filter.ActorID {
			continue
		}
		if filter.ResourceType != "" && e.ResourceType != filter.ResourceType {
			continue
		}
		if filter.ResourceID != "" && e.ResourceID != filter.ResourceID {
			continue
		}
		if filter.Action != "" && string(e.Action) != filter.Action {
			continue
		}
		if !filter.Since.IsZero() && e.CreatedAt.Before(filter.Since) {
			continue
		}
		if !filter.Until.IsZero() && e.CreatedAt.After(filter.Until) {
			continue
		}
		result = append(result, e)
	}
	// Apply pagination: offset then limit
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(result) {
		return []model.AuditEvent{}, nil
	}
	result = result[offset:]
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}
	return result, nil
}

func (r *mockAuditRepo) Latest(_ context.Context) (*model.AuditEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.store) == 0 {
		return nil, nil
	}
	latest := r.store[0]
	for _, e := range r.store[1:] {
		if e.CreatedAt.After(latest.CreatedAt) {
			latest = e
		}
	}
	return &latest, nil
}

// ─── Test Helpers ───────────────────────────────────────────────────────────

func testAuditEvent(id, actorID string, action model.AuditAction, resourceType, resourceID string, createdAt time.Time) model.AuditEvent {
	return model.AuditEvent{
		ID:           id,
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     `{}`,
		CreatedAt:    createdAt,
	}
}

// ─── AuditRepository Tests ──────────────────────────────────────────────────

func TestAuditRepository_Insert(t *testing.T) {
	repo := newMockAuditRepo()
	ctx := context.Background()
	event := testAuditEvent("aev-1", "user-1", "project.created", "project", "proj-1", time.Now())

	err := repo.Insert(ctx, event)
	if err != nil {
		t.Fatalf("Insert returned error: %v", err)
	}
}

func TestAuditRepository_List(t *testing.T) {
	repo := newMockAuditRepo()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	events := []model.AuditEvent{
		testAuditEvent("aev-1", "user-1", "project.created", "project", "proj-1", now.Add(-2*time.Hour)),
		testAuditEvent("aev-2", "user-1", "ticket.updated", "ticket", "ticket-1", now.Add(-1*time.Hour)),
		testAuditEvent("aev-3", "user-2", "project.created", "project", "proj-2", now),
	}
	for _, e := range events {
		must(t, repo.Insert(ctx, e))
	}

	t.Run("all events", func(t *testing.T) {
		result, err := repo.List(ctx, repository.AuditFilter{})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 3 {
			t.Errorf("got %d events, want 3", len(result))
		}
	})

	t.Run("filter by actor", func(t *testing.T) {
		result, err := repo.List(ctx, repository.AuditFilter{ActorID: "user-1"})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("got %d events, want 2", len(result))
		}
	})

	t.Run("filter by resource type", func(t *testing.T) {
		result, err := repo.List(ctx, repository.AuditFilter{ResourceType: "ticket"})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("got %d events, want 1", len(result))
		}
		if result[0].ID != "aev-2" {
			t.Errorf("got event %q, want %q", result[0].ID, "aev-2")
		}
	})

	t.Run("filter by resource id", func(t *testing.T) {
		result, err := repo.List(ctx, repository.AuditFilter{ResourceID: "proj-2"})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("got %d events, want 1", len(result))
		}
		if result[0].ID != "aev-3" {
			t.Errorf("got event %q, want %q", result[0].ID, "aev-3")
		}
	})

	t.Run("filter by action", func(t *testing.T) {
		result, err := repo.List(ctx, repository.AuditFilter{Action: "project.created"})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("got %d events, want 2", len(result))
		}
	})

	t.Run("filter by date range", func(t *testing.T) {
		// Only events within the last 90 minutes
		result, err := repo.List(ctx, repository.AuditFilter{
			Since: now.Add(-90 * time.Minute),
		})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("got %d events, want 2", len(result))
		}
	})

	t.Run("filter by until", func(t *testing.T) {
		// Only events before 90 minutes ago
		result, err := repo.List(ctx, repository.AuditFilter{
			Until: now.Add(-90 * time.Minute),
		})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("got %d events, want 1", len(result))
		}
	})
}

func TestAuditRepository_List_Pagination(t *testing.T) {
	repo := newMockAuditRepo()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	events := []model.AuditEvent{
		testAuditEvent("aev-1", "user-1", "project.created", "project", "proj-1", now),
		testAuditEvent("aev-2", "user-1", "ticket.updated", "ticket", "ticket-1", now),
		testAuditEvent("aev-3", "user-2", "project.created", "project", "proj-2", now),
	}
	for _, e := range events {
		must(t, repo.Insert(ctx, e))
	}

	t.Run("limit 1 returns one event", func(t *testing.T) {
		result, err := repo.List(ctx, repository.AuditFilter{Limit: 1})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("got %d events, want 1", len(result))
		}
	})

	t.Run("offset 1 skips first event", func(t *testing.T) {
		result, err := repo.List(ctx, repository.AuditFilter{Offset: 1})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("got %d events, want 2", len(result))
		}
		if result[0].ID != "aev-2" {
			t.Errorf("got first event %q, want %q", result[0].ID, "aev-2")
		}
	})

	t.Run("limit 1 offset 1 returns second event", func(t *testing.T) {
		result, err := repo.List(ctx, repository.AuditFilter{Limit: 1, Offset: 1})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("got %d events, want 1", len(result))
		}
		if result[0].ID != "aev-2" {
			t.Errorf("got event %q, want %q", result[0].ID, "aev-2")
		}
	})

	t.Run("offset beyond store returns empty", func(t *testing.T) {
		result, err := repo.List(ctx, repository.AuditFilter{Offset: 10})
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("got %d events, want 0", len(result))
		}
	})
}

func TestAuditRepository_List_Empty(t *testing.T) {
	repo := newMockAuditRepo()
	ctx := context.Background()

	result, err := repo.List(ctx, repository.AuditFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("got %d events, want 0", len(result))
	}
}

func TestAuditRepository_NoUpdateDelete(t *testing.T) {
	// Compile-time check: AuditRepository must not have Update or Delete.
	// If Update or Delete are added to the interface, mockAuditRepo will
	// fail to implement it and this line will not compile.
	var _ repository.AuditRepository = (*mockAuditRepo)(nil)
}
