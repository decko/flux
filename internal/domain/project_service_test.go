package domain_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
	"github.com/decko/flux/pkg/authctx"
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

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── ProjectService Tests ─────────────────────────────────────────────────

func TestProjectService_Create(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()
	p := testProject("proj-1", "test-project")

	err := svc.Create(ctx, p)
	must(t, err)

	// Verify it was stored in the repo.
	got, err := repo.Get(ctx, "proj-1")
	must(t, err)
	if got.ID != p.ID {
		t.Errorf("got ID %q, want %q", got.ID, p.ID)
	}
	if got.Name != p.Name {
		t.Errorf("got Name %q, want %q", got.Name, p.Name)
	}
}

func TestProjectService_Create_Invalid(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()
	p := testProject("proj-1", "") // empty name — invalid

	err := svc.Create(ctx, p)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatal("expected validation error, not ErrNotFound")
	}

	// Verify the mock was NOT called (project should not be stored).
	_, getErr := repo.Get(ctx, "proj-1")
	if !errors.Is(getErr, repository.ErrNotFound) {
		t.Fatal("project was stored in repo despite validation failure")
	}
}

func TestProjectService_Get(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()
	p := testProject("proj-1", "test-project")
	must(t, svc.Create(ctx, p))

	got, err := svc.Get(ctx, "proj-1")
	must(t, err)
	if got.ID != p.ID {
		t.Errorf("got ID %q, want %q", got.ID, p.ID)
	}
	if got.Name != p.Name {
		t.Errorf("got Name %q, want %q", got.Name, p.Name)
	}
	if got.RepoURL != p.RepoURL {
		t.Errorf("got RepoURL %q, want %q", got.RepoURL, p.RepoURL)
	}
	if got.Definition.Language != p.Definition.Language {
		t.Errorf("got Language %q, want %q", got.Definition.Language, p.Definition.Language)
	}
}

func TestProjectService_Get_NotFound(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()

	_, err := svc.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_List(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()

	projects := []model.Project{
		testProject("p1", "project-a"),
		testProject("p2", "project-b"),
		testProject("p3", "project-c"),
	}
	for _, p := range projects {
		must(t, svc.Create(ctx, p))
	}

	result, err := svc.List(ctx, repository.ProjectFilter{})
	must(t, err)
	if len(result) != len(projects) {
		t.Fatalf("got %d projects, want %d", len(result), len(projects))
	}

	// Verify all IDs are present.
	ids := make(map[string]bool)
	for _, p := range result {
		ids[p.ID] = true
	}
	for _, p := range projects {
		if !ids[p.ID] {
			t.Errorf("missing project %q in results", p.ID)
		}
	}
}

func TestProjectService_Update(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()
	p := testProject("proj-1", "original")
	must(t, svc.Create(ctx, p))

	p.Name = "updated"
	p.RepoURL = "https://github.com/example/updated"
	must(t, svc.Update(ctx, p))

	got, err := svc.Get(ctx, "proj-1")
	must(t, err)
	if got.Name != "updated" {
		t.Errorf("got Name %q, want %q", got.Name, "updated")
	}
	if got.RepoURL != "https://github.com/example/updated" {
		t.Errorf("got RepoURL %q, want %q", got.RepoURL, "https://github.com/example/updated")
	}
}

func TestProjectService_Update_Invalid(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()
	p := testProject("proj-1", "valid")
	must(t, svc.Create(ctx, p))

	p.Name = "" // invalid
	err := svc.Update(ctx, p)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatal("expected validation error, not ErrNotFound")
	}

	// Verify the project was NOT modified in the store.
	got, getErr := repo.Get(ctx, "proj-1")
	must(t, getErr)
	if got.Name != "valid" {
		t.Errorf("project name changed despite validation failure: got %q, want %q", got.Name, "valid")
	}
}

func TestProjectService_Update_NotFound(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()
	p := testProject("nonexistent", "ghost")

	err := svc.Update(ctx, p)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_Delete(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()
	p := testProject("proj-1", "delete-me")
	must(t, svc.Create(ctx, p))

	must(t, svc.Delete(ctx, "proj-1"))

	_, err := svc.Get(ctx, "proj-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestProjectService_Delete_NotFound(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()

	err := svc.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── Audit Integration Tests ─────────────────────────────────────────────────

// setupAuditDB creates an in-memory SQLite audit repository for testing.
func setupAuditDB(t *testing.T) *repository.SQLiteAuditRepository {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}
	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlite")
	repo := repository.NewSQLiteAuditRepository(sdb)
	return repo
}

func TestProjectService_Create_AuditRecorded(t *testing.T) {
	auditRepo := setupAuditDB(t)
	auditSvc := domain.NewAuditService(auditRepo)
	projectRepo := newMockProjectRepo()
	svc := domain.NewProjectService(projectRepo, domain.WithAuditService(auditSvc))
	ctx := authctx.WithUserID(context.Background(), "test-user")

	p := testProject("proj-audit-1", "audit-test")
	must(t, svc.Create(ctx, p))

	events, err := auditRepo.List(context.Background(), repository.AuditFilter{})
	must(t, err)
	if len(events) != 1 {
		t.Fatalf("got %d audit events, want 1", len(events))
	}
	if events[0].Action != model.AuditAction("project.created") {
		t.Errorf("Action = %q, want %q", events[0].Action, "project.created")
	}
	if events[0].ResourceID != p.ID {
		t.Errorf("ResourceID = %q, want %q", events[0].ResourceID, p.ID)
	}
	if events[0].ActorID != "test-user" {
		t.Errorf("ActorID = %q, want %q", events[0].ActorID, "test-user")
	}
}

func TestProjectService_Update_AuditRecorded(t *testing.T) {
	auditRepo := setupAuditDB(t)
	auditSvc := domain.NewAuditService(auditRepo)
	projectRepo := newMockProjectRepo()
	svc := domain.NewProjectService(projectRepo, domain.WithAuditService(auditSvc))
	ctx := authctx.WithUserID(context.Background(), "test-user")

	p := testProject("proj-audit-2", "original")
	must(t, svc.Create(ctx, p))

	p.Name = "updated"
	must(t, svc.Update(ctx, p))

	events, err := auditRepo.List(context.Background(), repository.AuditFilter{})
	must(t, err)
	if len(events) != 2 {
		t.Fatalf("got %d audit events, want 2 (create + update)", len(events))
	}
	if events[0].Action != model.AuditAction("project.updated") {
		t.Errorf("Action = %q, want %q", events[0].Action, "project.updated")
	}
	if events[0].ResourceID != p.ID {
		t.Errorf("ResourceID = %q, want %q", events[0].ResourceID, p.ID)
	}
}

func TestProjectService_Delete_AuditRecorded(t *testing.T) {
	auditRepo := setupAuditDB(t)
	auditSvc := domain.NewAuditService(auditRepo)
	projectRepo := newMockProjectRepo()
	svc := domain.NewProjectService(projectRepo, domain.WithAuditService(auditSvc))
	ctx := authctx.WithUserID(context.Background(), "test-user")

	p := testProject("proj-audit-3", "delete-me")
	must(t, svc.Create(ctx, p))

	// Reset audit events so we only check delete event.
	must(t, svc.Delete(ctx, p.ID))

	events, err := auditRepo.List(context.Background(), repository.AuditFilter{})
	must(t, err)
	if len(events) != 2 {
		t.Fatalf("got %d audit events, want 2 (create + delete)", len(events))
	}
	if events[0].Action != model.AuditAction("project.deleted") {
		t.Errorf("Action = %q, want %q", events[0].Action, "project.deleted")
	}
	if events[0].ResourceID != p.ID {
		t.Errorf("ResourceID = %q, want %q", events[0].ResourceID, p.ID)
	}
}

func TestProjectService_AuditNil(t *testing.T) {
	projectRepo := newMockProjectRepo()
	svc := domain.NewProjectService(projectRepo) // no audit service
	ctx := authctx.WithUserID(context.Background(), "test-user")

	p := testProject("proj-noaudit", "no-audit")
	must(t, svc.Create(ctx, p))

	// Verify operation still succeeds.
	got, err := svc.Get(ctx, "proj-noaudit")
	must(t, err)
	if got.Name != "no-audit" {
		t.Errorf("got Name %q, want %q", got.Name, "no-audit")
	}

	// Update with nil audit.
	p.Name = "still-no-audit"
	must(t, svc.Update(ctx, p))

	// Delete with nil audit.
	must(t, svc.Delete(ctx, "proj-noaudit"))
}

// ─── RotateWebhookSecret Tests ──────────────────────────────────────────────

// mockSecretRepo is a thread-safe in-memory implementation of webhookSecretRepo.
type mockSecretRepo struct {
	mu      sync.Mutex
	secrets map[string]string
}

func newMockSecretRepo() *mockSecretRepo {
	return &mockSecretRepo{secrets: make(map[string]string)}
}

func (r *mockSecretRepo) Set(_ context.Context, repoURL, secret string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets[repoURL] = secret
	return nil
}

// mockWebhookUpdater is a test implementation of webhookUpdater that records
// calls and can be configured to return errors.
type mockWebhookUpdater struct {
	mu        sync.Mutex
	calls     []string
	updateErr error
}

func newMockWebhookUpdater() *mockWebhookUpdater {
	return &mockWebhookUpdater{}
}

func (u *mockWebhookUpdater) UpdateWebhook(_ context.Context, installationID int, owner, repo string, webhookID int, webhookURL, secret string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls = append(u.calls, fmt.Sprintf("update:%s/%s:%d", owner, repo, webhookID))
	return u.updateErr
}

// TestRotateWebhookSecret_NoWebhookUpdater verifies that rotation fails when
// the webhook updater is not configured.
func TestRotateWebhookSecret_NoWebhookUpdater(t *testing.T) {
	repo := newMockProjectRepo()
	secretRepo := newMockSecretRepo()
	svc := domain.NewProjectService(repo, domain.WithSecretRepo(secretRepo))
	ctx := context.Background()

	p := testProject("proj-1", "test-project")
	must(t, svc.Create(ctx, p))

	err := svc.RotateWebhookSecret(ctx, "proj-1")
	if !errors.Is(err, domain.ErrWebhookNotConfigured) {
		t.Fatalf("expected ErrWebhookNotConfigured, got %v", err)
	}
}

// TestRotateWebhookSecret_NoSecretRepo verifies that rotation fails when the
// webhook secret repository is not configured.
func TestRotateWebhookSecret_NoSecretRepo(t *testing.T) {
	repo := newMockProjectRepo()
	svc := domain.NewProjectService(repo)
	ctx := context.Background()

	p := testProject("proj-1", "test-project")
	must(t, svc.Create(ctx, p))

	err := svc.RotateWebhookSecret(ctx, "proj-1")
	if !errors.Is(err, domain.ErrWebhookNotConfigured) {
		t.Fatalf("expected ErrWebhookNotConfigured, got %v", err)
	}
}

// TestRotateWebhookSecret_NotFound verifies that rotation fails when the
// project does not exist.
func TestRotateWebhookSecret_NotFound(t *testing.T) {
	repo := newMockProjectRepo()
	secretRepo := newMockSecretRepo()
	upd := newMockWebhookUpdater()
	svc := domain.NewProjectService(repo,
		domain.WithSecretRepo(secretRepo),
		domain.WithWebhookUpdater(upd),
	)
	ctx := context.Background()

	err := svc.RotateWebhookSecret(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestRotateWebhookSecret_NoWebhookID verifies that rotation fails when the
// project has no webhook ID (webhook was never registered).
func TestRotateWebhookSecret_NoWebhookID(t *testing.T) {
	orig := os.Getenv("FLUX_WEBHOOK_URL")
	t.Cleanup(func() {
		if orig != "" {
			_ = os.Setenv("FLUX_WEBHOOK_URL", orig)
		} else {
			_ = os.Unsetenv("FLUX_WEBHOOK_URL")
		}
	})
	_ = os.Setenv("FLUX_WEBHOOK_URL", "https://example.com/webhooks")

	repo := newMockProjectRepo()
	secretRepo := newMockSecretRepo()
	upd := newMockWebhookUpdater()
	svc := domain.NewProjectService(repo,
		domain.WithSecretRepo(secretRepo),
		domain.WithWebhookUpdater(upd),
	)
	ctx := context.Background()

	p := testProject("proj-1", "test-project")
	must(t, svc.Create(ctx, p))

	err := svc.RotateWebhookSecret(ctx, "proj-1")
	if !errors.Is(err, domain.ErrNoWebhookRegistered) {
		t.Fatalf("expected ErrNoWebhookRegistered, got %v", err)
	}
}

// TestRotateWebhookSecret_NoGitHubAdapter verifies that rotation fails when
// the project has no GitHub adapter configured.
func TestRotateWebhookSecret_NoGitHubAdapter(t *testing.T) {
	orig := os.Getenv("FLUX_WEBHOOK_URL")
	t.Cleanup(func() {
		if orig != "" {
			_ = os.Setenv("FLUX_WEBHOOK_URL", orig)
		} else {
			_ = os.Unsetenv("FLUX_WEBHOOK_URL")
		}
	})
	_ = os.Setenv("FLUX_WEBHOOK_URL", "https://example.com/webhooks")

	repo := newMockProjectRepo()
	secretRepo := newMockSecretRepo()
	upd := newMockWebhookUpdater()
	svc := domain.NewProjectService(repo,
		domain.WithSecretRepo(secretRepo),
		domain.WithWebhookUpdater(upd),
	)
	ctx := context.Background()

	// testProject creates a project with empty adapters slice.
	p := testProject("proj-1", "test-project")
	p.WebhookID = 42
	must(t, svc.Create(ctx, p))

	err := svc.RotateWebhookSecret(ctx, "proj-1")
	if !errors.Is(err, domain.ErrNoGitHubAdapter) {
		t.Fatalf("expected ErrNoGitHubAdapter, got %v", err)
	}
}

// TestRotateWebhookSecret_WebhookURLNotSet verifies that rotation fails when
// FLUX_WEBHOOK_URL is not set.
func TestRotateWebhookSecret_WebhookURLNotSet(t *testing.T) {
	// Unset FLUX_WEBHOOK_URL if set.
	orig := os.Getenv("FLUX_WEBHOOK_URL")
	t.Cleanup(func() {
		if orig != "" {
			_ = os.Setenv("FLUX_WEBHOOK_URL", orig)
		} else {
			_ = os.Unsetenv("FLUX_WEBHOOK_URL")
		}
	})
	_ = os.Unsetenv("FLUX_WEBHOOK_URL")

	repo := newMockProjectRepo()
	secretRepo := newMockSecretRepo()
	upd := newMockWebhookUpdater()
	svc := domain.NewProjectService(repo,
		domain.WithSecretRepo(secretRepo),
		domain.WithWebhookUpdater(upd),
	)
	ctx := context.Background()

	p := testProject("proj-1", "test-project")
	p.WebhookID = 42
	p.Adapters = []model.AdapterConfig{
		{Type: "github", Config: map[string]string{"owner": "owner", "repo": "repo"}},
	}
	must(t, svc.Create(ctx, p))

	err := svc.RotateWebhookSecret(ctx, "proj-1")
	if !errors.Is(err, domain.ErrWebhookURLNotSet) {
		t.Fatalf("expected ErrWebhookURLNotSet, got %v", err)
	}
}

// TestRotateWebhookSecret_Success verifies that a full secret rotation succeeds,
// stores the new secret, and records an audit event.
func TestRotateWebhookSecret_Success(t *testing.T) {
	// Set FLUX_WEBHOOK_URL.
	orig := os.Getenv("FLUX_WEBHOOK_URL")
	t.Cleanup(func() {
		if orig != "" {
			_ = os.Setenv("FLUX_WEBHOOK_URL", orig)
		} else {
			_ = os.Unsetenv("FLUX_WEBHOOK_URL")
		}
	})
	_ = os.Setenv("FLUX_WEBHOOK_URL", "https://example.com/webhooks")

	auditRepo := setupAuditDB(t)
	auditSvc := domain.NewAuditService(auditRepo)
	projectRepo := newMockProjectRepo()
	secretRepo := newMockSecretRepo()
	upd := newMockWebhookUpdater()
	svc := domain.NewProjectService(projectRepo,
		domain.WithSecretRepo(secretRepo),
		domain.WithWebhookUpdater(upd),
		domain.WithAuditService(auditSvc),
	)
	ctx := authctx.WithUserID(context.Background(), "test-admin")

	p := testProject("proj-secret", "rotate-test")
	p.WebhookID = 42
	p.InstallationID = 1
	p.RepoURL = "https://github.com/owner/repo"
	p.Adapters = []model.AdapterConfig{
		{Type: "github", Config: map[string]string{"owner": "owner", "repo": "repo"}},
	}
	must(t, svc.Create(ctx, p))

	err := svc.RotateWebhookSecret(ctx, "proj-secret")
	must(t, err)

	// Verify the updater was called.
	if len(upd.calls) != 1 {
		t.Fatalf("expected 1 updater call, got %d", len(upd.calls))
	}
	expectedCall := "update:owner/repo:42"
	if upd.calls[0] != expectedCall {
		t.Errorf("updater call = %q, want %q", upd.calls[0], expectedCall)
	}

	// Verify the secret was changed.
	secretRepo.mu.Lock()
	newSecret := secretRepo.secrets[p.RepoURL]
	secretRepo.mu.Unlock()
	if newSecret == "" {
		t.Fatal("expected secret to be stored")
	}
	if len(newSecret) != 64 { // 32 bytes hex-encoded
		t.Errorf("expected secret length 64 (32 bytes hex), got %d", len(newSecret))
	}

	// Verify audit event was recorded.
	events, err := auditRepo.List(context.Background(), repository.AuditFilter{
		ResourceType: "project",
		ResourceID:   "proj-secret",
	})
	must(t, err)
	var rotationEvent bool
	for _, e := range events {
		if e.Action == model.AuditActionWebhookSecretRotated {
			rotationEvent = true
			if e.ActorID != "test-admin" {
				t.Errorf("ActorID = %q, want %q", e.ActorID, "test-admin")
			}
			break
		}
	}
	if !rotationEvent {
		t.Error("no webhook.secret_rotated audit event found")
	}
}
