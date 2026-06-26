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

// ─── Stub: UserRepository for UserService ──────────────────────────────────

type stubUserRepoForService struct {
	mu    sync.Mutex
	users map[string]model.User
}

func newStubUserRepoForService() *stubUserRepoForService {
	return &stubUserRepoForService{users: make(map[string]model.User)}
}

func (r *stubUserRepoForService) Create(_ context.Context, u model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.users {
		if existing.Email == u.Email {
			return repository.ErrDuplicateEmail
		}
	}
	r.users[u.ID] = u
	return nil
}

func (r *stubUserRepoForService) GetByEmail(_ context.Context, email string) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return model.User{}, repository.ErrNotFound
}

func (r *stubUserRepoForService) GetByID(_ context.Context, id string) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, exists := r.users[id]
	if !exists {
		return model.User{}, repository.ErrNotFound
	}
	return u, nil
}

func (r *stubUserRepoForService) Update(_ context.Context, u model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[u.ID]; !exists {
		return repository.ErrNotFound
	}
	r.users[u.ID] = u
	return nil
}

func (r *stubUserRepoForService) List(_ context.Context) ([]model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]model.User, 0, len(r.users))
	for _, u := range r.users {
		result = append(result, u)
	}
	return result, nil
}

func (r *stubUserRepoForService) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[id]; !exists {
		return repository.ErrNotFound
	}
	delete(r.users, id)
	return nil
}

func (r *stubUserRepoForService) Count(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.users), nil
}

func (r *stubUserRepoForService) CountByRole(_ context.Context, role string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, u := range r.users {
		if u.Role == role {
			count++
		}
	}
	return count, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func stubTestUser(id, email, role string) model.User {
	return model.User{
		ID:           id,
		Email:        email,
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
		Role:         role,
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
	}
}

// ─── TestUserService_ListUsers ─────────────────────────────────────────────

func TestUserService_ListUsers(t *testing.T) {
	repo := newStubUserRepoForService()
	svc := domain.NewUserService(repo)
	ctx := context.Background()

	users := []model.User{
		stubTestUser("u1", "alice@example.com", "admin"),
		stubTestUser("u2", "bob@example.com", "user"),
		stubTestUser("u3", "carol@example.com", "user"),
	}
	for _, u := range users {
		if err := repo.Create(ctx, u); err != nil {
			t.Fatalf("setup: create user: %v", err)
		}
	}

	got, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers returned error: %v", err)
	}
	if len(got) != len(users) {
		t.Errorf("got %d users, want %d", len(got), len(users))
	}
}

// ─── TestUserService_UpdateRole ────────────────────────────────────────────

func TestUserService_UpdateRole(t *testing.T) {
	repo := newStubUserRepoForService()
	svc := domain.NewUserService(repo)
	ctx := context.Background()

	admin := stubTestUser("admin-1", "admin@example.com", "admin")
	target := stubTestUser("user-1", "user@example.com", "user")
	if err := repo.Create(ctx, admin); err != nil {
		t.Fatalf("setup: create admin: %v", err)
	}
	if err := repo.Create(ctx, target); err != nil {
		t.Fatalf("setup: create target: %v", err)
	}

	err := svc.UpdateRole(ctx, admin.ID, target.ID, "admin")
	if err != nil {
		t.Fatalf("UpdateRole returned error: %v", err)
	}

	updated, err := repo.GetByID(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.Role != "admin" {
		t.Errorf("got role %q, want %q", updated.Role, "admin")
	}
}

// ─── TestUserService_UpdateRole_InvalidRole ────────────────────────────────

func TestUserService_UpdateRole_InvalidRole(t *testing.T) {
	repo := newStubUserRepoForService()
	svc := domain.NewUserService(repo)
	ctx := context.Background()

	admin := stubTestUser("admin-1", "admin@example.com", "admin")
	target := stubTestUser("user-1", "user@example.com", "user")
	if err := repo.Create(ctx, admin); err != nil {
		t.Fatalf("setup: create admin: %v", err)
	}
	if err := repo.Create(ctx, target); err != nil {
		t.Fatalf("setup: create target: %v", err)
	}

	err := svc.UpdateRole(ctx, admin.ID, target.ID, "superadmin")
	if err == nil {
		t.Fatal("expected error for invalid role, got nil")
	}
}

// ─── TestUserService_UpdateRole_CannotDemoteSelf ───────────────────────────

func TestUserService_UpdateRole_CannotDemoteSelf(t *testing.T) {
	repo := newStubUserRepoForService()
	svc := domain.NewUserService(repo)
	ctx := context.Background()

	admin := stubTestUser("admin-1", "admin@example.com", "admin")
	if err := repo.Create(ctx, admin); err != nil {
		t.Fatalf("setup: create admin: %v", err)
	}

	err := svc.UpdateRole(ctx, admin.ID, admin.ID, "user")
	if err == nil {
		t.Fatal("expected error when demoting self, got nil")
	}
}

// ─── TestUserService_UpdateRole_CannotDemoteLastAdmin ──────────────────────

func TestUserService_UpdateRole_CannotDemoteLastAdmin(t *testing.T) {
	repo := newStubUserRepoForService()
	svc := domain.NewUserService(repo)
	ctx := context.Background()

	admin := stubTestUser("admin-1", "admin@example.com", "admin")
	// Only one admin — demoting them would leave zero.
	if err := repo.Create(ctx, admin); err != nil {
		t.Fatalf("setup: create admin: %v", err)
	}

	err := svc.UpdateRole(ctx, "some-other-admin", admin.ID, "user")
	if err == nil {
		t.Fatal("expected error when demoting last admin, got nil")
	}
}

// ─── TestUserService_DeleteUser ────────────────────────────────────────────

func TestUserService_DeleteUser(t *testing.T) {
	repo := newStubUserRepoForService()
	svc := domain.NewUserService(repo)
	ctx := context.Background()

	admin := stubTestUser("admin-1", "admin@example.com", "admin")
	target := stubTestUser("user-1", "user@example.com", "user")
	if err := repo.Create(ctx, admin); err != nil {
		t.Fatalf("setup: create admin: %v", err)
	}
	if err := repo.Create(ctx, target); err != nil {
		t.Fatalf("setup: create target: %v", err)
	}

	err := svc.DeleteUser(ctx, admin.ID, target.ID)
	if err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}

	_, err = repo.GetByID(ctx, target.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

// ─── TestUserService_DeleteUser_CannotDeleteSelf ───────────────────────────

func TestUserService_DeleteUser_CannotDeleteSelf(t *testing.T) {
	repo := newStubUserRepoForService()
	svc := domain.NewUserService(repo)
	ctx := context.Background()

	admin := stubTestUser("admin-1", "admin@example.com", "admin")
	if err := repo.Create(ctx, admin); err != nil {
		t.Fatalf("setup: create admin: %v", err)
	}

	err := svc.DeleteUser(ctx, admin.ID, admin.ID)
	if err == nil {
		t.Fatal("expected error when deleting self, got nil")
	}
}

// ─── TestUserService_DeleteUser_CannotDeleteLastAdmin ──────────────────────

func TestUserService_DeleteUser_CannotDeleteLastAdmin(t *testing.T) {
	repo := newStubUserRepoForService()
	svc := domain.NewUserService(repo)
	ctx := context.Background()

	admin := stubTestUser("admin-1", "admin@example.com", "admin")
	if err := repo.Create(ctx, admin); err != nil {
		t.Fatalf("setup: create admin: %v", err)
	}

	err := svc.DeleteUser(ctx, "some-other-admin", admin.ID)
	if err == nil {
		t.Fatal("expected error when deleting last admin, got nil")
	}
}
