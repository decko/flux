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

// ─── Mock: AuditRepository ─────────────────────────────────────────────────

type mockAuditRepo struct {
	mu    sync.Mutex
	count int64
	err   error
}

func newMockAuditRepo() *mockAuditRepo {
	return &mockAuditRepo{}
}

func (r *mockAuditRepo) Insert(_ context.Context, _ model.AuditEvent) error {
	return nil
}

func (r *mockAuditRepo) List(_ context.Context, _ repository.AuditFilter) ([]model.AuditEvent, error) {
	return nil, nil
}

func (r *mockAuditRepo) PurgeOlderThan(_ context.Context, _ time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count, r.err
}

// ─── Tests ─────────────────────────────────────────────────────────────────

func TestAuditService_PurgeOldEvents_Success(t *testing.T) {
	repo := newMockAuditRepo()
	repo.count = 5
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	err := svc.PurgeOldEvents(ctx, 90)
	if err != nil {
		t.Fatalf("PurgeOldEvents returned error: %v", err)
	}
}

func TestAuditService_PurgeOldEvents_ZeroRetention(t *testing.T) {
	repo := newMockAuditRepo()
	repo.count = 10
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	err := svc.PurgeOldEvents(ctx, 0)
	if err != nil {
		t.Fatalf("PurgeOldEvents with 0 retention returned error: %v", err)
	}
}

func TestAuditService_PurgeOldEvents_NegativeRetention(t *testing.T) {
	repo := newMockAuditRepo()
	repo.count = 10
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	err := svc.PurgeOldEvents(ctx, -1)
	if err != nil {
		t.Fatalf("PurgeOldEvents with negative retention returned error: %v", err)
	}
}

func TestAuditService_PurgeOldEvents_RepoError(t *testing.T) {
	repo := newMockAuditRepo()
	repo.err = errors.New("database error")
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	err := svc.PurgeOldEvents(ctx, 90)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAuditService_PurgeOldEvents_ZeroDeleted(t *testing.T) {
	repo := newMockAuditRepo()
	repo.count = 0
	svc := domain.NewAuditService(repo)
	ctx := context.Background()

	err := svc.PurgeOldEvents(ctx, 90)
	if err != nil {
		t.Fatalf("PurgeOldEvents returned error: %v", err)
	}
}
