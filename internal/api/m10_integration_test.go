package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// TestM10_UI_PipelineTriggerManagement verifies the full M10 flow:
// DB-backed trigger rules → API CRUD → TriggerService evaluation.
func TestM10_UI_PipelineTriggerManagement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// 1. In-memory database with migrations.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	sdb := sqlx.NewDb(db, "sqlite")
	projectRepo := repository.NewSQLiteProjectRepository(sdb)
	pipelineRepo := repository.NewSQLitePipelineRunRepository(sdb)
	ruleRepo := repository.NewSQLiteTriggerRuleRepository(sdb)
	projectSvc := domain.NewProjectService(projectRepo)

	// 2. Create a project with pipelines.
	project := model.Project{
		ID:      "proj-m10",
		Name:    "test",
		RepoURL: "https://github.com/decko/flux",
		Pipelines: []model.PipelineConfig{
			{Name: "review"},
			{Name: "implement"},
		},
	}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// 3. Create trigger rules via repo (simulating API).
	rule := model.TriggerRule{
		ID:        uuid.New().String(),
		ProjectID: "proj-m10",
		Label:     "flux/review",
		Pipeline:  "review",
		Enabled:   true,
	}
	if err := ruleRepo.Create(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	// 4. Verify rules can be listed.
	rules, err := ruleRepo.ListByProject(ctx, "proj-m10")
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	// 5. TriggerService reads rule from DB and fires pipeline.
	pipelineSvc := domain.NewPipelineRunService(pipelineRepo)
	triggerSvc := domain.NewTriggerService(pipelineSvc, projectRepo, pipelineRepo, ruleRepo, "flux-bot")

	ticket := model.Ticket{
		ID:        "ticket-m10-1",
		ProjectID: "proj-m10",
		Labels:    []string{"flux/review"},
		Status:    model.TicketStatusOpen,
	}

	if err := triggerSvc.CheckAndTrigger(ctx, ticket, model.DefaultEvent); err != nil {
		t.Fatalf("CheckAndTrigger: %v", err)
	}

	// Verify pipeline run was created with correct pipeline name.
	runs, _ := pipelineRepo.List(ctx, repository.PipelineRunFilter{ProjectID: "proj-m10"})
	if len(runs) != 1 {
		t.Fatalf("expected 1 pipeline run, got %d", len(runs))
	}
	if runs[0].Pipeline != "review" {
		t.Errorf("run pipeline = %q, want %q", runs[0].Pipeline, "review")
	}

	// 6. Ticket without matching label does NOT trigger.
	noMatch := model.Ticket{
		ID:        "ticket-m10-2",
		ProjectID: "proj-m10",
		Labels:    []string{"bug"},
	}
	if err := triggerSvc.CheckAndTrigger(ctx, noMatch, model.DefaultEvent); err != nil {
		t.Fatalf("no-match check: %v", err)
	}
	runs, _ = pipelineRepo.List(ctx, repository.PipelineRunFilter{ProjectID: "proj-m10"})
	if len(runs) != 1 {
		t.Errorf("expected 1 run (no new trigger), got %d", len(runs))
	}

	// 7. Delete rule → trigger no longer fires.
	if err := ruleRepo.Delete(ctx, rule.ID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}
	if err := triggerSvc.CheckAndTrigger(ctx, ticket, model.DefaultEvent); err != nil {
		t.Fatalf("post-delete trigger: %v", err)
	}
	runs, _ = pipelineRepo.List(ctx, repository.PipelineRunFilter{ProjectID: "proj-m10"})
	if len(runs) != 1 {
		t.Errorf("expected 1 run (rule deleted), got %d", len(runs))
	}

	// 8. Auth: trigger rule API requires authentication.
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithProjectService(projectSvc), WithTriggerRuleRepo(ruleRepo))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Unauthenticated GET → 401
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		ts.URL+"/api/v1/projects/proj-m10/trigger-rules", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET unauthed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthed trigger rules: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	// Authenticated GET → 200
	authedReq := authedRequest(http.MethodGet, ts.URL+"/api/v1/projects/proj-m10/trigger-rules", nil)
	resp2, err := http.DefaultClient.Do(authedReq)
	if err != nil {
		t.Fatalf("GET authed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("authed trigger rules: got %d, want %d", resp2.StatusCode, http.StatusOK)
	}

	// Admin-only POST → 403 for non-admin
	nonAdminToken := generateNonAdminToken()
	body := `{"label":"test","pipeline":"review"}`
	nonAdminReq, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		ts.URL+"/api/v1/projects/proj-m10/trigger-rules", strings.NewReader(body))
	nonAdminReq.Header.Set("Authorization", "Bearer "+nonAdminToken)
	resp3, err := http.DefaultClient.Do(nonAdminReq)
	if err != nil {
		t.Fatalf("POST non-admin: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()
	if resp3.StatusCode != http.StatusForbidden {
		t.Errorf("non-admin POST: got %d, want %d", resp3.StatusCode, http.StatusForbidden)
	}

	t.Log("M10 smoke test passed")
}
