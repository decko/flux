package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// setupWebhookSecretTestServer creates an in-memory SQLite database, migrates
// it, seeds test users (admin + regular), creates a project with a GitHub
// adapter and webhook ID, and returns a Server ready for testing the secret
// rotation endpoint. Optionally starts a mock GitHub API server for PATCH
// requests.
func setupWebhookSecretTestServer(t *testing.T, mockGitHub *httptest.Server) *Server {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("configure db: %v", err)
	}

	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlite")

	// Seed users.
	userRepo := repository.NewSQLiteUserRepository(sdb)
	ctx := context.Background()
	now := time.Now().UTC()
	seedUsers := []model.User{
		{
			ID:           "admin-1",
			Email:        "admin@example.com",
			PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
			Role:         "admin",
			CreatedAt:    now,
		},
		{
			ID:           "user-1",
			Email:        "user@example.com",
			PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
			Role:         "user",
			CreatedAt:    now,
		},
	}
	for _, u := range seedUsers {
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatalf("seed: create user %s: %v", u.ID, err)
		}
	}

	// Create repositories and services.
	projectRepo := repository.NewSQLiteProjectRepository(sdb)
	webhookSecretRepo := repository.NewSQLiteWebhookSecretRepository(sdb)
	auditRepo := repository.NewSQLiteAuditRepository(sdb)
	auditSvc := domain.NewAuditService(auditRepo)
	userSvc := domain.NewUserService(userRepo, domain.WithUserAuditService(auditSvc))

	// Set up mock GitHub API as the webhook updater.
	var webhookUpd domain.WebhookUpdater
	if mockGitHub != nil {
		// Use a GitHubWebhookUpdater-like adapter that points to the mock.
		webhookUpd = &mockGitHubUpdater{serverURL: mockGitHub.URL}
	}

	projectSvc := domain.NewProjectService(projectRepo,
		domain.WithSecretRepo(webhookSecretRepo),
		domain.WithWebhookUpdater(webhookUpd),
		domain.WithAuditService(auditSvc),
	)

	// Seed a test project with GitHub adapter and webhook ID.
	// Use a context with an admin user ID for audit compliance.
	adminCtx := authctx.WithUserID(context.Background(), "admin-1")
	project := model.Project{
		ID:             "proj-1",
		Name:           "test-project",
		RepoURL:        "https://github.com/test-owner/test-repo",
		InstallationID: 1,
		WebhookID:      42,
		Definition: model.ProjectDefinition{
			Language: "Go",
		},
		Adapters: []model.AdapterConfig{
			{Type: "github", Config: map[string]string{"owner": "test-owner", "repo": "test-repo"}},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := projectSvc.Create(adminCtx, project); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	// Store a webhook secret for the project.
	if err := webhookSecretRepo.Set(ctx, project.RepoURL, "original-secret"); err != nil {
		t.Fatalf("seed webhook secret: %v", err)
	}

	return NewServer(
		WithJWTSecret(testJWTSecretBytes),
		WithProjectService(projectSvc),
		WithUserService(userSvc),
		WithWebhookSecretRepo(webhookSecretRepo),
		WithAuditService(auditSvc),
	)
}

// mockGitHubUpdater is a simple webhookUpdater that sends PATCH requests to
// a mock GitHub server URL. It implements the domain.WebhookUpdater interface.
type mockGitHubUpdater struct {
	serverURL string
}

func (u *mockGitHubUpdater) UpdateWebhook(ctx context.Context, installationID int, owner, repo string, webhookID int, webhookURL, secret string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/hooks/%d", u.serverURL, owner, repo, webhookID)
	payload := map[string]interface{}{
		"config": map[string]interface{}{
			"url":          webhookURL,
			"content_type": "json",
			"secret":       secret,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mock GitHub: HTTP %d", resp.StatusCode)
	}
	return nil
}

// ─── Tests ─────────────────────────────────────────────────────────────────

// TestWebhookSecretRotate_AdminSuccess verifies that an admin can rotate the
// webhook secret and receives 200 with {"status":"rotated"}.
func TestWebhookSecretRotate_AdminSuccess(t *testing.T) {
	// Start a mock GitHub server that returns 200 for PATCH requests.
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/hooks/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockGitHub.Close()

	t.Setenv("FLUX_WEBHOOK_URL", mockGitHub.URL+"/api/v1/webhooks/github")

	srv := setupWebhookSecretTestServer(t, mockGitHub)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects/proj-1/webhook/rotate-secret", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/proj-1/webhook/rotate-secret: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "rotated" {
		t.Errorf("got status %q, want %q", body["status"], "rotated")
	}
}

// TestWebhookSecretRotate_NonAdminForbidden verifies that a non-admin user
// receives 403 Forbidden.
func TestWebhookSecretRotate_NonAdminForbidden(t *testing.T) {
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockGitHub.Close()

	srv := setupWebhookSecretTestServer(t, mockGitHub)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := nonAdminRequest(http.MethodPost, ts.URL+"/api/v1/projects/proj-1/webhook/rotate-secret", "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/proj-1/webhook/rotate-secret (non-admin): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestWebhookSecretRotate_Unauthenticated verifies that an unauthenticated
// request returns 401.
func TestWebhookSecretRotate_Unauthenticated(t *testing.T) {
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockGitHub.Close()

	srv := setupWebhookSecretTestServer(t, mockGitHub)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/v1/projects/proj-1/webhook/rotate-secret", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/proj-1/webhook/rotate-secret (no auth): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestWebhookSecretRotate_SecretChanged verifies that after rotation, the
// stored webhook secret is different from the original.
func TestWebhookSecretRotate_SecretChanged(t *testing.T) {
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/hooks/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockGitHub.Close()

	t.Setenv("FLUX_WEBHOOK_URL", mockGitHub.URL+"/api/v1/webhooks/github")

	srv := setupWebhookSecretTestServer(t, mockGitHub)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Read the original secret.
	origSecret, err := srv.webhookSecretRepo.Get(context.Background(), "https://github.com/test-owner/test-repo")
	if err != nil {
		t.Fatalf("get original secret: %v", err)
	}
	if origSecret != "original-secret" {
		t.Fatalf("expected original secret 'original-secret', got %q", origSecret)
	}

	// Rotate.
	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects/proj-1/webhook/rotate-secret", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/proj-1/webhook/rotate-secret: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Read the new secret.
	newSecret, err := srv.webhookSecretRepo.Get(context.Background(), "https://github.com/test-owner/test-repo")
	if err != nil {
		t.Fatalf("get new secret: %v", err)
	}
	if newSecret == origSecret {
		t.Error("secret was not changed after rotation")
	}
	if len(newSecret) != 64 { // 32 bytes hex-encoded
		t.Errorf("expected secret length 64, got %d", len(newSecret))
	}
}

// TestWebhookSecretRotate_AuditEventRecorded verifies that a secret rotation
// records a webhook.secret_rotated audit event with the correct actor.
func TestWebhookSecretRotate_AuditEventRecorded(t *testing.T) {
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/hooks/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockGitHub.Close()

	t.Setenv("FLUX_WEBHOOK_URL", mockGitHub.URL+"/api/v1/webhooks/github")

	srv := setupWebhookSecretTestServer(t, mockGitHub)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects/proj-1/webhook/rotate-secret", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/proj-1/webhook/rotate-secret: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Verify audit event.
	events, err := srv.auditSvc.List(context.Background(), repository.AuditFilter{
		ResourceType: "project",
		ResourceID:   "proj-1",
	})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	var found bool
	for _, e := range events {
		if e.Action == model.AuditActionWebhookSecretRotated {
			found = true
			if e.ActorID != testUserID {
				t.Errorf("ActorID = %q, want %q", e.ActorID, testUserID)
			}
			if e.ResourceID != "proj-1" {
				t.Errorf("ResourceID = %q, want %q", e.ResourceID, "proj-1")
			}
			break
		}
	}
	if !found {
		t.Error("no webhook.secret_rotated audit event found")
	}
}

// TestWebhookSecretRotate_NotFound verifies that rotating the secret for a
// nonexistent project returns 404.
func TestWebhookSecretRotate_NotFound(t *testing.T) {
	mockGitHub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockGitHub.Close()

	srv := setupWebhookSecretTestServer(t, mockGitHub)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects/nonexistent/webhook/rotate-secret", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/nonexistent/webhook/rotate-secret: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
