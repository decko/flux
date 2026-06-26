package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Helpers ────────────────────────────────────────────────────────────────

// hmacSign signs a payload with a secret using HMAC-SHA256 and returns the
// full "sha256=..." header value, matching GitHub's X-Hub-Signature-256 format.
func hmacSign(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// githubPayload returns a minimal GitHub webhook JSON payload for an issues
// event. For labeled/unlabeled actions, the top-level "label" field is included.
// Labels are passed as a string slice of label names.
func githubPayload(action, fullName, senderLogin string, labels []string) []byte {
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
			"number":   1,
			"title":    "Test Issue",
			"state":    state,
			"labels":   issueLabels,
			"html_url": fmt.Sprintf("https://github.com/%s/issues/1", fullName),
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

// ─── Setup ──────────────────────────────────────────────────────────────────

// setupWebhookTestServer creates an in-memory SQLite database, runs migrations,
// seeds test data (webhook secret, project, trigger rules), and returns a
// Server configured with all needed services.
func setupWebhookTestServer(t *testing.T) *Server {
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

	// Create webhook_secrets table and seed a test secret.
	// This table does not exist in migrations yet and will be added by the
	// webhook handler implementation. The raw SQL here is the test-level
	// schema definition that the handler implementation must match.
	if _, err := db.ExecContext(context.Background(), `CREATE TABLE IF NOT EXISTS webhook_secrets (
		repo_url TEXT PRIMARY KEY,
		secret TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create webhook_secrets table: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `INSERT OR REPLACE INTO webhook_secrets (repo_url, secret) VALUES (?, ?)`,
		"https://github.com/test-owner/test-repo", "test-webhook-secret",
	); err != nil {
		t.Fatalf("seed webhook secret: %v", err)
	}

	projectRepo := repository.NewSQLiteProjectRepository(sdb)
	ticketRepo := repository.NewSQLiteTicketRepository(sdb)
	pipelineRepo := repository.NewSQLitePipelineRunRepository(sdb)
	triggerRepo := repository.NewSQLiteTriggerRuleRepository(sdb)
	webhookSecretRepo := repository.NewSQLiteWebhookSecretRepository(sdb)

	projectSvc := domain.NewProjectService(projectRepo)
	ticketSvc := domain.NewTicketService(ticketRepo)
	pipelineSvc := domain.NewPipelineRunService(pipelineRepo)

	return NewServer(
		WithJWTSecret(testJWTSecretBytes),
		WithProjectService(projectSvc),
		WithTicketService(ticketSvc),
		WithPipelineService(pipelineSvc),
		WithTriggerRuleRepo(triggerRepo),
		WithWebhookSecretRepo(webhookSecretRepo),
	)
}

// seedWebhookProject creates a project linked to the test repo URL.
func seedWebhookProject(t *testing.T, srv *Server) model.Project {
	t.Helper()

	p := model.Project{
		ID:      uuid.New().String(),
		Name:    "test-project",
		RepoURL: "https://github.com/test-owner/test-repo",
		Definition: model.ProjectDefinition{
			Language: "Go",
		},
		Adapters: []model.AdapterConfig{},
		Pipelines: []model.PipelineConfig{
			{Type: "soda", Name: "dev-loop", Config: map[string]string{}},
			{Type: "soda", Name: "plan", Config: map[string]string{}},
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := srv.projectSvc.Create(context.Background(), p); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return p
}

// seedWebhookTriggerRule creates an enabled trigger rule for the given project
// that matches issues labeled "bug" and triggers the "dev-loop" pipeline.
func seedWebhookTriggerRule(t *testing.T, srv *Server, projectID string) model.TriggerRule {
	t.Helper()

	rule := model.TriggerRule{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Label:     "bug",
		Pipeline:  "dev-loop",
		Enabled:   true,
		Priority:  10,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := srv.triggerRuleRepo.Create(context.Background(), rule); err != nil {
		t.Fatalf("seed trigger rule: %v", err)
	}
	return rule
}

// ─── Test cases ─────────────────────────────────────────────────────────────

// TestWebhookGithub_ValidSignature verifies that a properly signed webhook
// request with a valid payload returns 200 and upserts a ticket.
func TestWebhookGithub_ValidSignature(t *testing.T) {
	srv := setupWebhookTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedWebhookProject(t, srv)

	payload := githubPayload("opened", "test-owner/test-repo", "testuser", nil)
	sig := hmacSign(payload, "test-webhook-secret")

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/webhooks/github: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Verify a ticket was upserted.
	tickets, err := srv.ticketSvc.List(context.Background(), repository.TicketFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	if len(tickets) != 1 {
		t.Errorf("got %d tickets, want 1", len(tickets))
	}
	if len(tickets) > 0 {
		if tickets[0].ExternalID != "1" {
			t.Errorf("got external_id %q, want %q", tickets[0].ExternalID, "1")
		}
		if tickets[0].Title != "Test Issue" {
			t.Errorf("got title %q, want %q", tickets[0].Title, "Test Issue")
		}
		if tickets[0].Source != model.TicketSourceGitHub {
			t.Errorf("got source %q, want %q", tickets[0].Source, model.TicketSourceGitHub)
		}
	}

	// No trigger rule set up — no pipeline run should exist.
	runs, err := srv.pipelineSvc.List(context.Background(), repository.PipelineRunFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("list pipeline runs: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("got %d pipeline runs, want 0", len(runs))
	}
}

// TestWebhookGithub_InvalidSignature verifies that a request with an incorrect
// HMAC signature returns 401 and does not create a ticket.
func TestWebhookGithub_InvalidSignature(t *testing.T) {
	srv := setupWebhookTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	seedWebhookProject(t, srv)

	payload := githubPayload("opened", "test-owner/test-repo", "testuser", nil)
	// Sign with wrong secret.
	sig := hmacSign(payload, "wrong-secret")

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/webhooks/github: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestWebhookGithub_MissingSignature verifies that a request without the
// X-Hub-Signature-256 header returns 401.
func TestWebhookGithub_MissingSignature(t *testing.T) {
	srv := setupWebhookTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	seedWebhookProject(t, srv)

	payload := githubPayload("opened", "test-owner/test-repo", "testuser", nil)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	// Deliberately omit X-Hub-Signature-256.

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/webhooks/github: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestWebhookGithub_UnsupportedEvent verifies that a webhook for an event type
// that flux does not handle (e.g., "member") returns 200 and does nothing.
func TestWebhookGithub_UnsupportedEvent(t *testing.T) {
	srv := setupWebhookTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	seedWebhookProject(t, srv)

	// A "member" event has a different payload shape — build it inline.
	payload := map[string]interface{}{
		"action": "added",
		"member": map[string]string{"login": "newuser"},
		"repository": map[string]string{
			"full_name": "test-owner/test-repo",
		},
		"sender": map[string]string{
			"login": "admin",
		},
	}
	body, _ := json.Marshal(payload)
	sig := hmacSign(body, "test-webhook-secret")

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "member")
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/webhooks/github: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Verify no ticket was created and no pipeline run was triggered.
	project, err := srv.projectSvc.List(context.Background(), repository.ProjectFilter{})
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(project) > 0 {
		tickets, err := srv.ticketSvc.List(context.Background(), repository.TicketFilter{ProjectID: project[0].ID})
		if err != nil {
			t.Fatalf("list tickets: %v", err)
		}
		if len(tickets) != 0 {
			t.Errorf("got %d tickets, want 0 for unsupported event", len(tickets))
		}
		runs, err := srv.pipelineSvc.List(context.Background(), repository.PipelineRunFilter{ProjectID: project[0].ID})
		if err != nil {
			t.Fatalf("list pipeline runs: %v", err)
		}
		if len(runs) != 0 {
			t.Errorf("got %d pipeline runs, want 0 for unsupported event", len(runs))
		}
	}
}

// TestWebhookGithub_BotActor verifies that a webhook from a bot sender (e.g.,
// flux-bot) returns 200 and does not trigger any pipeline runs, even if the
// ticket matches a trigger rule.
func TestWebhookGithub_BotActor(t *testing.T) {
	srv := setupWebhookTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedWebhookProject(t, srv)
	seedWebhookTriggerRule(t, srv, project.ID)

	payload := githubPayload("labeled", "test-owner/test-repo", "flux-bot", []string{"bug"})
	sig := hmacSign(payload, "test-webhook-secret")

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/webhooks/github: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Ticket should still be upserted (bot events are valid issue events).
	tickets, err := srv.ticketSvc.List(context.Background(), repository.TicketFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	if len(tickets) != 1 {
		t.Errorf("got %d tickets, want 1 (bot issue events still upsert)", len(tickets))
	}

	// Pipeline should NOT be triggered for bot actors.
	runs, err := srv.pipelineSvc.List(context.Background(), repository.PipelineRunFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("list pipeline runs: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("got %d pipeline runs, want 0 (bot should not trigger)", len(runs))
	}
}

// TestWebhookGithub_UnknownRepo verifies that a webhook for a repository not
// tracked in flux returns 200 without creating a ticket or pipeline run.
func TestWebhookGithub_UnknownRepo(t *testing.T) {
	srv := setupWebhookTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	seedWebhookProject(t, srv)

	payload := githubPayload("opened", "test-owner/unknown-repo", "testuser", nil)
	sig := hmacSign(payload, "test-webhook-secret")

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/webhooks/github: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// No tickets should exist (no matching project found).
	projects, err := srv.projectSvc.List(context.Background(), repository.ProjectFilter{})
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) > 0 {
		tickets, err := srv.ticketSvc.List(context.Background(), repository.TicketFilter{ProjectID: projects[0].ID})
		if err != nil {
			t.Fatalf("list tickets: %v", err)
		}
		if len(tickets) != 0 {
			t.Errorf("got %d tickets, want 0 (unknown repo should not create tickets)", len(tickets))
		}
	}
}

// TestWebhookGithub_IssuesEventTriggersPipeline verifies that a labeled issue
// matching a trigger rule creates both a ticket and a pipeline run.
func TestWebhookGithub_IssuesEventTriggersPipeline(t *testing.T) {
	srv := setupWebhookTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedWebhookProject(t, srv)
	seedWebhookTriggerRule(t, srv, project.ID)

	payload := githubPayload("labeled", "test-owner/test-repo", "testuser", []string{"bug"})
	sig := hmacSign(payload, "test-webhook-secret")

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/webhooks/github: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Verify ticket was upserted with the correct label.
	tickets, err := srv.ticketSvc.List(context.Background(), repository.TicketFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	if len(tickets) != 1 {
		t.Fatalf("got %d tickets, want 1", len(tickets))
	}
	if len(tickets[0].Labels) != 1 || tickets[0].Labels[0] != "bug" {
		t.Errorf("got labels %v, want [bug]", tickets[0].Labels)
	}

	// Verify a pipeline run was created by the trigger rule.
	runs, err := srv.pipelineSvc.List(context.Background(), repository.PipelineRunFilter{
		ProjectID: project.ID,
		TicketID:  tickets[0].ID,
	})
	if err != nil {
		t.Fatalf("list pipeline runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("got %d pipeline runs, want 1", len(runs))
	}
	if runs[0].Pipeline != "dev-loop" {
		t.Errorf("got pipeline %q, want %q", runs[0].Pipeline, "dev-loop")
	}
	if runs[0].Status != model.RunStatusPending {
		t.Errorf("got status %q, want %q", runs[0].Status, model.RunStatusPending)
	}
	if runs[0].TicketID != tickets[0].ID {
		t.Errorf("got ticket_id %q, want %q", runs[0].TicketID, tickets[0].ID)
	}
}

// ─── Auth guard tests ───────────────────────────────────────────────────────

// TestWebhookGithub_PublicEndpoint verifies that the webhook endpoint accepts
// requests without a JWT token (the endpoint is public, relying on HMAC).
func TestWebhookGithub_PublicEndpoint(t *testing.T) {
	srv := setupWebhookTestServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	seedWebhookProject(t, srv)

	payload := githubPayload("opened", "test-owner/test-repo", "testuser", nil)
	sig := hmacSign(payload, "test-webhook-secret")

	// Deliberately send no Authorization header — should still reach the handler
	// (which will verify HMAC) rather than returning 401 from the JWT middleware.
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/webhooks/github: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// The endpoint is public (no JWT), so a valid HMAC should return 200.
	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d (webhook endpoint is public)", resp.StatusCode, http.StatusOK)
	}
}
