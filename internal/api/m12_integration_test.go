package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// m12MockSyncService implements syncService for testing sync endpoints.
// It records sync calls and provides a configurable Status response.
type m12MockSyncService struct {
	mu            sync.Mutex
	lastSyncAt    *time.Time
	lastSyncError string
	ticketsSynced int
	prsSynced     int
}

func (s *m12MockSyncService) Status() domain.SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return domain.SyncStatus{
		LastSyncAt:      s.lastSyncAt,
		LastSyncError:   s.lastSyncError,
		TicketsSynced:   s.ticketsSynced,
		PRsSynced:       s.prsSynced,
		WebhooksHealthy: true,
	}
}

func (s *m12MockSyncService) SyncNow(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.lastSyncAt = &now
	s.ticketsSynced = 5
	s.prsSynced = 3
	s.lastSyncError = ""
	return nil
}

// m12WebhookPayload creates a GitHub issues webhook payload with a configurable
// issue number, action, repo full name, sender, and optional labels.
func m12WebhookPayload(issueNumber int, action, fullName, senderLogin string, labels []string) []byte {
	type ghLabel struct {
		Name string `json:"name"`
	}
	issueLabels := make([]ghLabel, len(labels))
	for i, l := range labels {
		issueLabels[i] = ghLabel{Name: l}
	}

	state := "open"
	if action == "closed" {
		state = "closed"
	}

	payload := map[string]interface{}{
		"action": action,
		"issue": map[string]interface{}{
			"number":   issueNumber,
			"title":    fmt.Sprintf("Test Issue %d", issueNumber),
			"state":    state,
			"labels":   issueLabels,
			"html_url": fmt.Sprintf("https://github.com/%s/issues/%d", fullName, issueNumber),
		},
		"repository": map[string]interface{}{
			"full_name": fullName,
		},
		"sender": map[string]interface{}{
			"login": senderLogin,
		},
	}
	if action == "labeled" || action == "unlabeled" {
		if len(labels) > 0 {
			payload["label"] = map[string]string{"name": labels[len(labels)-1]}
		}
	}
	data, _ := json.Marshal(payload)
	return data
}

// sendWebhook sends a signed GitHub issues webhook event to the test server.
func sendWebhook(ctx context.Context, ts *httptest.Server, payload []byte, secret string) (*http.Response, error) {
	sig := hmacSign(payload, secret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", sig)
	return http.DefaultClient.Do(req)
}

// TestM12_WebhookDrivenIngress verifies the M12 milestone end-to-end:
//   - Deterministic ticket IDs from webhook ingress (same ID as sync path)
//   - Webhook health tracking (last_webhook_at updated on project)
//   - Audit events recorded for webhook ticket ingress
//   - Admin-gated sync trigger (non-admin 403, admin 202)
//   - Webhook health status (webhooks_healthy field in sync status)
//   - Webhook secret rotation (old secret rejected after rotation)
func TestM12_WebhookDrivenIngress(t *testing.T) {
	ctx := context.Background()

	// ─── 1. Setup ──────────────────────────────────────────────────────────
	//
	// Start mock GitHub for webhook secret rotation (PATCH /hooks/:id).
	// Must be running before project service creation since the project
	// service is configured with a mock updater from the start.
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

	// In-memory SQLite database with migrations.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("configure db: %v", err)
	}
	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlite")

	// Repositories.
	projectRepo := repository.NewSQLiteProjectRepository(sdb)
	ticketRepo := repository.NewSQLiteTicketRepository(sdb)
	auditRepo := repository.NewSQLiteAuditRepository(sdb)
	webhookSecretRepo := repository.NewSQLiteWebhookSecretRepository(sdb)
	userRepo := repository.NewSQLiteUserRepository(sdb)

	// Services.
	auditSvc := domain.NewAuditService(auditRepo)
	ticketSvc := domain.NewTicketService(ticketRepo)
	userSvc := domain.NewUserService(userRepo, domain.WithUserAuditService(auditSvc))

	webhookUpd := &mockGitHubUpdater{serverURL: mockGitHub.URL}
	projectSvc := domain.NewProjectService(projectRepo,
		domain.WithAuditService(auditSvc),
		domain.WithSecretRepo(webhookSecretRepo),
		domain.WithWebhookUpdater(webhookUpd),
	)

	// Seed an admin user for auth and audit actor tracking.
	now := time.Now().UTC()
	adminUser := model.User{
		ID:           "m12-admin",
		Email:        "m12-admin@flux.dev",
		PasswordHash: "$2a$10$abcdefghijklmnopqrstuv",
		Role:         "admin",
		CreatedAt:    now,
	}
	if err := userRepo.Create(ctx, adminUser); err != nil {
		t.Fatalf("seed admin user: %v", err)
	}

	// Create a test project with a GitHub adapter and a non-zero webhook ID
	// (required for webhook secret rotation). Use an authenticated context so
	// audit events record the admin actor.
	adminCtx := authctx.WithUserID(ctx, "m12-admin")
	project := model.Project{
		ID:             "proj-m12",
		Name:           "M12 Test Project",
		RepoURL:        "https://github.com/test-owner/test-repo",
		InstallationID: 1,
		WebhookID:      42,
		Adapters: []model.AdapterConfig{
			{Type: "github", Config: map[string]string{"owner": "test-owner", "repo": "test-repo"}},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := projectSvc.Create(adminCtx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Store the initial webhook secret for HMAC signing.
	originalSecret := "m12-original-webhook-secret-32bytes!"
	if err := webhookSecretRepo.Set(ctx, project.RepoURL, originalSecret); err != nil {
		t.Fatalf("set webhook secret: %v", err)
	}

	// Mock sync service for sync/trigger and sync/status endpoints.
	mockSync := &m12MockSyncService{}

	// Create server with all M12 wiring — real services for webhooks and
	// tickets, mock for sync, full auth middleware.
	srv := NewServer(
		WithJWTSecret(testJWTSecretBytes),
		WithProjectService(projectSvc),
		WithTicketService(ticketSvc),
		WithAuditService(auditSvc),
		WithUserService(userSvc),
		WithWebhookSecretRepo(webhookSecretRepo),
		WithSyncService(mockSync),
	)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	adminHeader := "Bearer " + generateTestToken()

	// ─── 2. Verify ID Determinism ─────────────────────────────────────────
	//
	// A ticket created via webhook should have the same deterministic ID as
	// one created via the sync path, for the same source + externalID pair.
	t.Log("Step 2: Verify ID determinism")

	expectedID := model.TicketID(model.TicketSourceGitHub, "1")

	payload := m12WebhookPayload(1, "opened", "test-owner/test-repo", "tester", nil)
	resp, err := sendWebhook(ctx, ts, payload, originalSecret)
	if err != nil {
		t.Fatalf("webhook POST (issue 1): %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("webhook (issue 1): got %d, want 200", resp.StatusCode)
	}

	// Fetch the ticket by the expected deterministic ID.
	ticket, err := ticketRepo.Get(ctx, expectedID)
	if err != nil {
		t.Fatalf("get ticket by deterministic ID %q: %v", expectedID, err)
	}
	if ticket.ID != expectedID {
		t.Errorf("ticket.ID = %q, want %q (deterministic ID mismatch)", ticket.ID, expectedID)
	}
	if ticket.ExternalID != "1" {
		t.Errorf("ticket.ExternalID = %q, want %q", ticket.ExternalID, "1")
	}
	if ticket.Source != model.TicketSourceGitHub {
		t.Errorf("ticket.Source = %q, want %q", ticket.Source, model.TicketSourceGitHub)
	}
	t.Logf("  ✓ Webhook ticket ID %q matches deterministic ID", ticket.ID)

	// Confirm the sync path produces the same ID.
	syncID := model.TicketID(model.TicketSourceGitHub, "1")
	if syncID != ticket.ID {
		t.Errorf("sync path ID = %q, webhook path ID = %q (mismatch)", syncID, ticket.ID)
	}

	// ─── 3. Verify Webhook Health (last_webhook_at) ──────────────────────
	t.Log("Step 3: Verify webhook health (last_webhook_at)")

	pBefore, err := projectRepo.Get(ctx, "proj-m12")
	if err != nil {
		t.Fatalf("get project before second webhook: %v", err)
	}
	t.Logf("  Before: LastWebhookAt = %v", pBefore.LastWebhookAt)

	// Send a second webhook for a different issue to ensure a new create event.
	payload2 := m12WebhookPayload(2, "opened", "test-owner/test-repo", "tester", nil)
	resp2, err := sendWebhook(ctx, ts, payload2, originalSecret)
	if err != nil {
		t.Fatalf("webhook POST (issue 2): %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("webhook (issue 2): got %d, want 200", resp2.StatusCode)
	}

	pAfter, err := projectRepo.Get(ctx, "proj-m12")
	if err != nil {
		t.Fatalf("get project after second webhook: %v", err)
	}
	if pAfter.LastWebhookAt == nil {
		t.Error("LastWebhookAt is nil after webhook event — expected non-nil timestamp")
	} else {
		t.Logf("  After: LastWebhookAt = %v", pAfter.LastWebhookAt)
		if pBefore.LastWebhookAt != nil && pAfter.LastWebhookAt.Before(*pBefore.LastWebhookAt) {
			t.Error("LastWebhookAt moved backwards")
		}
	}

	// ─── 4. Verify Audit Events for Ingress ──────────────────────────────
	t.Log("Step 4: Verify audit events for ingress")

	respAudit, err := httpGet(ts.URL+"/api/v1/audit-events", adminHeader)
	if err != nil {
		t.Fatalf("GET /api/v1/audit-events: %v", err)
	}
	defer func() { _ = respAudit.Body.Close() }()
	if respAudit.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/audit-events: got %d, want 200", respAudit.StatusCode)
	}

	var events []map[string]interface{}
	if err := json.NewDecoder(respAudit.Body).Decode(&events); err != nil {
		t.Fatalf("decode audit events: %v", err)
	}

	foundWebhookCreate := false
	foundWebhookUpdate := false
	for _, e := range events {
		action, _ := e["action"].(string)
		switch action {
		case string(model.AuditActionTicketCreatedWebhook):
			foundWebhookCreate = true
			t.Logf("  Found ticket.created.webhook: actor=%v resource_id=%v",
				e["actor_id"], e["resource_id"])
		case string(model.AuditActionTicketUpdatedWebhook):
			foundWebhookUpdate = true
			t.Logf("  Found ticket.updated.webhook: actor=%v resource_id=%v",
				e["actor_id"], e["resource_id"])
		}
	}
	if !foundWebhookCreate && !foundWebhookUpdate {
		t.Error("no ticket.created.webhook or ticket.updated.webhook audit event found")
	}
	if foundWebhookCreate {
		t.Log("  ✓ ticket.created.webhook audit event recorded")
	}

	// Verify unauthenticated access to audit events is blocked.
	respAuditUnauth, err := httpGet(ts.URL+"/api/v1/audit-events", "")
	if err != nil {
		t.Fatalf("GET /api/v1/audit-events (unauth): %v", err)
	}
	_ = respAuditUnauth.Body.Close()
	if respAuditUnauth.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth audit events: got %d, want 401", respAuditUnauth.StatusCode)
	}

	// ─── 5. Verify Admin-Gated Sync Trigger ────────────────────────────
	t.Log("Step 5: Verify admin-gated sync trigger")

	// Non-admin → 403 Forbidden.
	reqTrigger := nonAdminRequest(http.MethodPost, ts.URL+"/api/v1/sync/trigger", "")
	respTrigger, err := http.DefaultClient.Do(reqTrigger)
	if err != nil {
		t.Fatalf("POST /api/v1/sync/trigger (non-admin): %v", err)
	}
	_ = respTrigger.Body.Close()
	if respTrigger.StatusCode != http.StatusForbidden {
		t.Errorf("non-admin sync trigger: got %d, want %d",
			respTrigger.StatusCode, http.StatusForbidden)
	}
	t.Log("  ✓ Non-admin → 403")

	// Admin → 202 Accepted.
	reqTriggerAdmin := authedRequest(http.MethodPost, ts.URL+"/api/v1/sync/trigger", nil)
	respTriggerAdmin, err := http.DefaultClient.Do(reqTriggerAdmin)
	if err != nil {
		t.Fatalf("POST /api/v1/sync/trigger (admin): %v", err)
	}
	_ = respTriggerAdmin.Body.Close()
	if respTriggerAdmin.StatusCode != http.StatusAccepted {
		t.Errorf("admin sync trigger: got %d, want %d",
			respTriggerAdmin.StatusCode, http.StatusAccepted)
	}
	t.Log("  ✓ Admin → 202")

	// ─── 6. Verify Webhook Health Status in Sync Status ────────────────
	t.Log("Step 6: Verify webhook health status field")

	respStatus, err := httpGet(ts.URL+"/api/v1/sync/status", adminHeader)
	if err != nil {
		t.Fatalf("GET /api/v1/sync/status: %v", err)
	}
	defer func() { _ = respStatus.Body.Close() }()
	if respStatus.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/v1/sync/status: got %d, want 200", respStatus.StatusCode)
	}

	var statusResp syncStatusResponse
	if err := json.NewDecoder(respStatus.Body).Decode(&statusResp); err != nil {
		t.Fatalf("decode sync status: %v", err)
	}
	if !statusResp.WebhooksHealthy {
		t.Error("expected webhooks_healthy to be true in sync status response")
	}
	t.Logf("  ✓ webhooks_healthy = %v", statusResp.WebhooksHealthy)

	// Verify all expected sync status fields are present.
	if statusResp.TicketsSynced < 0 {
		t.Error("tickets_synced should be non-negative")
	}
	if statusResp.PRsSynced < 0 {
		t.Error("prs_synced should be non-negative")
	}
	t.Logf("  Sync status: last_sync_at=%v tickets=%d prs=%d",
		statusResp.LastSyncAt, statusResp.TicketsSynced, statusResp.PRsSynced)

	// ─── 7. Verify Webhook Secret Rotation ─────────────────────────────
	t.Log("Step 7: Verify webhook secret rotation")

	// Read the original secret before rotation.
	origSecret, err := webhookSecretRepo.Get(ctx, project.RepoURL)
	if err != nil {
		t.Fatalf("get original secret: %v", err)
	}
	if origSecret != originalSecret {
		t.Fatalf("original secret mismatch: got %q, want %q", origSecret, originalSecret)
	}

	// Rotate as admin.
	reqRotate := authedRequest(http.MethodPost,
		ts.URL+"/api/v1/projects/proj-m12/webhook/rotate-secret", nil)
	respRotate, err := http.DefaultClient.Do(reqRotate)
	if err != nil {
		t.Fatalf("POST /api/v1/projects/proj-m12/webhook/rotate-secret: %v", err)
	}
	defer func() { _ = respRotate.Body.Close() }()
	if respRotate.StatusCode != http.StatusOK {
		t.Errorf("rotate secret: got %d, want %d", respRotate.StatusCode, http.StatusOK)
	}

	var rotateBody map[string]string
	if err := json.NewDecoder(respRotate.Body).Decode(&rotateBody); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	if rotateBody["status"] != "rotated" {
		t.Errorf("rotate response status = %q, want %q", rotateBody["status"], "rotated")
	}
	t.Log("  ✓ POST /api/v1/projects/proj-m12/webhook/rotate-secret → 200")

	// Verify the secret in the repository has changed.
	newSecret, err := webhookSecretRepo.Get(ctx, project.RepoURL)
	if err != nil {
		t.Fatalf("get new secret: %v", err)
	}
	if newSecret == origSecret {
		t.Error("webhook secret was not changed after rotation")
	}
	if len(newSecret) != 64 { // 32 random bytes hex-encoded = 64 chars
		t.Errorf("expected new secret length 64, got %d", len(newSecret))
	}
	t.Log("  ✓ Webhook secret changed in repository")

	// Verify the old secret no longer authenticates webhooks.
	payloadOld := m12WebhookPayload(3, "opened", "test-owner/test-repo", "attacker", nil)
	respOld, err := sendWebhook(ctx, ts, payloadOld, origSecret)
	if err != nil {
		t.Fatalf("webhook with old secret: %v", err)
	}
	_ = respOld.Body.Close()
	if respOld.StatusCode != http.StatusUnauthorized {
		t.Errorf("webhook with old secret: got %d, want 401", respOld.StatusCode)
	}
	t.Log("  ✓ Old secret rejected (401)")

	// Verify the new secret works.
	payloadNew := m12WebhookPayload(3, "opened", "test-owner/test-repo", "tester", nil)
	respNew, err := sendWebhook(ctx, ts, payloadNew, newSecret)
	if err != nil {
		t.Fatalf("webhook with new secret: %v", err)
	}
	_ = respNew.Body.Close()
	if respNew.StatusCode != http.StatusOK {
		t.Errorf("webhook with new secret: got %d, want 200", respNew.StatusCode)
	}
	t.Log("  ✓ New secret accepted (200)")

	t.Log("M12 smoke test passed")
}
