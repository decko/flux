package api

import (
	"context"
	"database/sql"
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

// setupTriggerServer creates an in-memory SQLite database, migrates it,
// creates a ProjectService-backed Server with a TriggerRuleRepository,
// and returns the server along with a seed function for creating projects.
func setupTriggerServer(t *testing.T) *Server {
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
	projectRepo := repository.NewSQLiteProjectRepository(sdb)
	triggerRepo := repository.NewSQLiteTriggerRuleRepository(sdb)
	projectSvc := domain.NewProjectService(projectRepo)

	return NewServer(
		WithJWTSecret(testJWTSecretBytes),
		WithProjectService(projectSvc),
		WithTriggerRuleRepo(triggerRepo),
	)
}

// seedProject creates a project in the database for testing.
func seedProject(t *testing.T, srv *Server) model.Project {
	t.Helper()

	p := model.Project{
		ID:      uuid.New().String(),
		Name:    "test-project",
		RepoURL: "https://github.com/example/test",
		Definition: model.ProjectDefinition{
			Language:  "Go",
			Framework: "chi",
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

// seedTriggerRule creates a trigger rule in the database for testing.
func seedTriggerRule(t *testing.T, srv *Server, projectID string) model.TriggerRule {
	t.Helper()

	rule := model.TriggerRule{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Label:     "bug",
		Pipeline:  "dev-loop",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := srv.triggerRuleRepo.Create(context.Background(), rule); err != nil {
		t.Fatalf("seed trigger rule: %v", err)
	}
	return rule
}

// triggerRuleRequestBody builds a JSON request body for a trigger rule.
func triggerRuleRequestBody(label, pipeline string) string {
	if label == "" && pipeline == "" {
		return `{}`
	}
	parts := []string{}
	if label != "" {
		parts = append(parts, fmt.Sprintf(`"label":%q`, label))
	}
	if pipeline != "" {
		parts = append(parts, fmt.Sprintf(`"pipeline":%q`, pipeline))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// ─── List ──────────────────────────────────────────────────────────────────

func TestHandleListTriggerRules_Success(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)
	rule1 := seedTriggerRule(t, srv, project.ID)
	rule2 := seedTriggerRule(t, srv, project.ID)

	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req := authedRequest(http.MethodGet, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var rules []model.TriggerRule
	mustDecode(t, resp, &rules)
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2", len(rules))
	}

	ids := map[string]bool{rules[0].ID: true, rules[1].ID: true}
	if !ids[rule1.ID] {
		t.Errorf("expected rule %q in list", rule1.ID)
	}
	if !ids[rule2.ID] {
		t.Errorf("expected rule %q in list", rule2.ID)
	}
}

func TestHandleListTriggerRules_Empty(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req := authedRequest(http.MethodGet, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var rules []model.TriggerRule
	mustDecode(t, resp, &rules)
	if rules == nil {
		t.Fatal("expected non-nil empty array, got nil")
	}
	if len(rules) != 0 {
		t.Errorf("got %d rules, want 0", len(rules))
	}
}

func TestHandleListTriggerRules_ProjectNotFound(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, uuid.New().String())
	req := authedRequest(http.MethodGet, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var errResp map[string]string
	mustDecode(t, resp, &errResp)
	if _, ok := errResp["error"]; !ok {
		t.Error("JSON response missing 'error' field")
	}
}

// ─── Create ────────────────────────────────────────────────────────────────

func TestHandleCreateTriggerRule_Success(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	body := triggerRuleRequestBody("bug", "dev-loop")
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req := authedRequest(http.MethodPost, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if loc := resp.Header.Get("Location"); loc == "" {
		t.Error("missing Location header")
	}

	var created model.TriggerRule
	mustDecode(t, resp, &created)
	if created.ID == "" {
		t.Error("expected non-empty trigger rule ID")
	}
	if created.ProjectID != project.ID {
		t.Errorf("got project_id %q, want %q", created.ProjectID, project.ID)
	}
	if created.Label != "bug" {
		t.Errorf("got label %q, want %q", created.Label, "bug")
	}
	if created.Pipeline != "dev-loop" {
		t.Errorf("got pipeline %q, want %q", created.Pipeline, "dev-loop")
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
	if created.UpdatedAt.IsZero() {
		t.Error("expected non-zero updated_at")
	}
}

func TestHandleCreateTriggerRule_InvalidPipeline(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	body := triggerRuleRequestBody("bug", "nonexistent-pipeline")
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req := authedRequest(http.MethodPost, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]string
	mustDecode(t, resp, &errResp)
	if !strings.Contains(errResp["error"], "invalid pipeline") {
		t.Errorf("error message %q does not contain 'invalid pipeline'", errResp["error"])
	}
}

func TestHandleCreateTriggerRule_MissingLabel(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	body := triggerRuleRequestBody("", "dev-loop")
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req := authedRequest(http.MethodPost, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]string
	mustDecode(t, resp, &errResp)
	if !strings.Contains(errResp["error"], "label is required") {
		t.Errorf("error message %q does not contain 'label is required'", errResp["error"])
	}
}

func TestHandleCreateTriggerRule_MissingPipeline(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	body := triggerRuleRequestBody("bug", "")
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req := authedRequest(http.MethodPost, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]string
	mustDecode(t, resp, &errResp)
	if !strings.Contains(errResp["error"], "pipeline is required") {
		t.Errorf("error message %q does not contain 'pipeline is required'", errResp["error"])
	}
}

func TestHandleCreateTriggerRule_ProjectNotFound(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := triggerRuleRequestBody("bug", "dev-loop")
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, uuid.New().String())
	req := authedRequest(http.MethodPost, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleCreateTriggerRule_MalformedJSON(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	body := `{bad json}`
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req := authedRequest(http.MethodPost, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var errResp map[string]string
	mustDecode(t, resp, &errResp)
	if _, ok := errResp["error"]; !ok {
		t.Error("JSON error response missing 'error' field")
	}
}

// ─── Auth guards ──────────────────────────────────────────────────────────

func TestHandleCreateTriggerRule_Unauthorized(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	body := triggerRuleRequestBody("bug", "dev-loop")
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestHandleCreateTriggerRule_NotAdmin(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	// Use a non-admin token.
	nonAdminToken := generateNonAdminToken()
	body := triggerRuleRequestBody("bug", "dev-loop")
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+nonAdminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// ─── Update ────────────────────────────────────────────────────────────────

func TestHandleUpdateTriggerRule_Success(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)
	rule := seedTriggerRule(t, srv, project.ID)

	updateBody := `{"label":"critical","pipeline":"plan"}`
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules/%s", ts.URL, project.ID, rule.ID)
	req := authedRequest(http.MethodPut, u, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var updated model.TriggerRule
	mustDecode(t, resp, &updated)
	if updated.ID != rule.ID {
		t.Errorf("got ID %q, want %q", updated.ID, rule.ID)
	}
	if updated.Label != "critical" {
		t.Errorf("got label %q, want %q", updated.Label, "critical")
	}
	if updated.Pipeline != "plan" {
		t.Errorf("got pipeline %q, want %q", updated.Pipeline, "plan")
	}
	if updated.ProjectID != project.ID {
		t.Errorf("got project_id %q, want %q", updated.ProjectID, project.ID)
	}
}

func TestHandleUpdateTriggerRule_NotFound(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	updateBody := `{"label":"critical","pipeline":"plan"}`
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules/%s", ts.URL, project.ID, uuid.New().String())
	req := authedRequest(http.MethodPut, u, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// ─── Delete ────────────────────────────────────────────────────────────────

func TestHandleDeleteTriggerRule_Success(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)
	rule := seedTriggerRule(t, srv, project.ID)

	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules/%s", ts.URL, project.ID, rule.ID)
	req := authedRequest(http.MethodDelete, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestHandleDeleteTriggerRule_NotFound(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules/%s", ts.URL, project.ID, uuid.New().String())
	req := authedRequest(http.MethodDelete, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var errResp map[string]string
	mustDecode(t, resp, &errResp)
	if _, ok := errResp["error"]; !ok {
		t.Error("JSON response missing 'error' field")
	}
}

// ─── Unauthorized for non-admin endpoints ──────────────────────────────────

func TestHandleDeleteTriggerRule_Unauthorized(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)
	rule := seedTriggerRule(t, srv, project.ID)

	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules/%s", ts.URL, project.ID, rule.ID)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestHandleDeleteTriggerRule_NotAdmin(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)
	rule := seedTriggerRule(t, srv, project.ID)

	nonAdminToken := generateNonAdminToken()
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules/%s", ts.URL, project.ID, rule.ID)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, u, nil)
	req.Header.Set("Authorization", "Bearer "+nonAdminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// ─── List unauthorized (non-admin can list) ──────────────────────────────

func TestHandleListTriggerRules_AnyAuthUserCanList(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)
	seedTriggerRule(t, srv, project.ID)

	// Use a non-admin token to list (should succeed).
	nonAdminToken := generateNonAdminToken()
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+nonAdminToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d (non-admin should be able to list)", resp.StatusCode, http.StatusOK)
	}

	var rules []model.TriggerRule
	mustDecode(t, resp, &rules)
	if len(rules) != 1 {
		t.Errorf("got %d rules, want 1", len(rules))
	}
}

// TestTriggerRuleIntegration is a smoke test that exercises create, list,
// update, and delete in sequence.
func TestTriggerRuleIntegration(t *testing.T) {
	srv := setupTriggerServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	project := seedProject(t, srv)

	// Create a rule.
	createBody := triggerRuleRequestBody("bug", "dev-loop")
	u := fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID)
	req := authedRequest(http.MethodPost, u, strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create trigger rule: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create got status %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var created model.TriggerRule
	mustDecode(t, resp, &created)
	_ = resp.Body.Close()

	// List rules.
	req = authedRequest(http.MethodGet, u, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list trigger rules: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var rules []model.TriggerRule
	mustDecode(t, resp, &rules)
	_ = resp.Body.Close()
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}

	// Update the rule.
	updateBody := `{"label":"critical","pipeline":"plan"}`
	u = fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules/%s", ts.URL, project.ID, created.ID)
	req = authedRequest(http.MethodPut, u, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update trigger rule: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
	_ = resp.Body.Close()

	// Delete the rule.
	req = authedRequest(http.MethodDelete, u, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete trigger rule: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete got status %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	_ = resp.Body.Close()

	// Verify deleted via list.
	req = authedRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/projects/%s/trigger-rules", ts.URL, project.ID), nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	mustDecode(t, resp, &rules)
	_ = resp.Body.Close()
	if len(rules) != 0 {
		t.Errorf("got %d rules after delete, want 0", len(rules))
	}
}
