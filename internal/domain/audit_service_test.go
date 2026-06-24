package domain_test

import (
	"context"
	"sync"
	"testing"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Mock: AuditRepository ─────────────────────────────────────────────────

type mockAuditRepo struct {
	mu    sync.Mutex
	store []model.AuditEvent
}

func newMockAuditRepo() *mockAuditRepo {
	return &mockAuditRepo{store: make([]model.AuditEvent, 0)}
}

func (r *mockAuditRepo) Create(_ context.Context, e model.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store = append(r.store, e)
	return nil
}

func (r *mockAuditRepo) List(_ context.Context) ([]model.AuditEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]model.AuditEvent, len(r.store))
	copy(result, r.store)
	return result, nil
}

func (r *mockAuditRepo) Latest(_ context.Context) (model.AuditEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.store) == 0 {
		return model.AuditEvent{}, repository.ErrNotFound
	}
	return r.store[len(r.store)-1], nil
}

// ─── Tests ─────────────────────────────────────────────────────────────────

func TestAuditService_Record_SetsHash(t *testing.T) {
	repo := newMockAuditRepo()
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	event := model.AuditEvent{
		ID:           "evt-1",
		ActorID:      "user-1",
		Action:       "create",
		ResourceType: "project",
		ResourceID:   "proj-1",
	}

	if err := svc.Record(ctx, &event); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	if event.Hash == "" {
		t.Fatal("expected non-empty hash after Record")
	}

	// First event should have empty previous_hash.
	if event.PreviousHash != "" {
		t.Errorf("first event: expected empty previous_hash, got %q", event.PreviousHash)
	}
}

func TestAuditService_Record_ChainLinksCorrectly(t *testing.T) {
	repo := newMockAuditRepo()
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	events := []model.AuditEvent{
		{ID: "evt-1", ActorID: "user-1", Action: "create", ResourceType: "project", ResourceID: "proj-1"},
		{ID: "evt-2", ActorID: "user-1", Action: "update", ResourceType: "project", ResourceID: "proj-1"},
		{ID: "evt-3", ActorID: "user-2", Action: "delete", ResourceType: "ticket", ResourceID: "ticket-5"},
	}

	for i := range events {
		if err := svc.Record(ctx, &events[i]); err != nil {
			t.Fatalf("Record event %d returned error: %v", i, err)
		}
	}

	// First event has empty previous_hash.
	if events[0].PreviousHash != "" {
		t.Errorf("event 0: expected empty previous_hash, got %q", events[0].PreviousHash)
	}

	// Each event's previous_hash must equal the prior event's hash.
	for i := 1; i < len(events); i++ {
		if events[i].PreviousHash != events[i-1].Hash {
			t.Errorf("event %d: previous_hash %q does not match event %d hash %q",
				i, events[i].PreviousHash, i-1, events[i-1].Hash)
		}
	}

	// All hashes must be non-empty and distinct.
	for i, evt := range events {
		if evt.Hash == "" {
			t.Errorf("event %d: hash is empty", i)
		}
		for j := i + 1; j < len(events); j++ {
			if evt.Hash == events[j].Hash {
				t.Errorf("event %d and %d have the same hash %q", i, j, evt.Hash)
			}
		}
	}
}

func TestAuditService_Record_EmptyStoreFirstEvent(t *testing.T) {
	repo := newMockAuditRepo()
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	event := model.AuditEvent{
		ID:           "evt-first",
		ActorID:      "system",
		Action:       "bootstrap",
		ResourceType: "system",
		ResourceID:   "init",
	}

	if err := svc.Record(ctx, &event); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	if event.PreviousHash != "" {
		t.Errorf("first event: expected empty previous_hash, got %q", event.PreviousHash)
	}
	if event.Hash == "" {
		t.Fatal("first event: expected non-empty hash")
	}
}

func TestAuditService_VerifyIntegrity_Valid(t *testing.T) {
	repo := newMockAuditRepo()
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	events := []model.AuditEvent{
		{ID: "e1", ActorID: "u1", Action: "create", ResourceType: "proj", ResourceID: "p1"},
		{ID: "e2", ActorID: "u1", Action: "update", ResourceType: "proj", ResourceID: "p1"},
	}
	for i := range events {
		if err := svc.Record(ctx, &events[i]); err != nil {
			t.Fatalf("Record event %d: %v", i, err)
		}
	}

	valid, firstBrokenAt, err := svc.VerifyIntegrity(ctx)
	if err != nil {
		t.Fatalf("VerifyIntegrity returned error: %v", err)
	}
	if !valid {
		t.Errorf("expected valid=true, got false")
	}
	if firstBrokenAt != "" {
		t.Errorf("expected empty first_broken_at, got %q", firstBrokenAt)
	}
}

func TestAuditService_VerifyIntegrity_TamperedHash(t *testing.T) {
	repo := newMockAuditRepo()
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	events := []model.AuditEvent{
		{ID: "e1", ActorID: "u1", Action: "create", ResourceType: "proj", ResourceID: "p1"},
		{ID: "e2", ActorID: "u1", Action: "update", ResourceType: "proj", ResourceID: "p1"},
	}
	for i := range events {
		if err := svc.Record(ctx, &events[i]); err != nil {
			t.Fatalf("Record event %d: %v", i, err)
		}
	}

	// Tamper with the second event's hash directly in the mock store.
	repo.mu.Lock()
	repo.store[1].Hash = "tampered-hash"
	repo.mu.Unlock()

	valid, firstBrokenAt, err := svc.VerifyIntegrity(ctx)
	if err != nil {
		t.Fatalf("VerifyIntegrity returned error: %v", err)
	}
	if valid {
		t.Errorf("expected valid=false for tampered chain")
	}
	if firstBrokenAt == "" {
		t.Errorf("expected non-empty first_broken_at for tampered chain")
	}
}

func TestAuditService_VerifyIntegrity_EmptyStore(t *testing.T) {
	repo := newMockAuditRepo()
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	valid, firstBrokenAt, err := svc.VerifyIntegrity(ctx)
	if err != nil {
		t.Fatalf("VerifyIntegrity returned error: %v", err)
	}
	if !valid {
		t.Errorf("expected valid=true for empty store")
	}
	if firstBrokenAt != "" {
		t.Errorf("expected empty first_broken_at for empty store, got %q", firstBrokenAt)
	}
}

// Compile-time check that mockAuditRepo implements AuditRepository.
var _ repository.AuditRepository = (*mockAuditRepo)(nil)
