package api

import (
	"context"
	"database/sql"
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

// TestM9_EventDrivenPipelineTriggers verifies the cross-package wiring for
// auto-triggered pipelines from GitHub events.
func TestM9_EventDrivenPipelineTriggers(t *testing.T) {
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

	// 2. Create a project with pipelines.
	project := model.Project{
		ID:      "proj-m9",
		Name:    "test",
		RepoURL: "https://github.com/decko/flux",
		Pipelines: []model.PipelineConfig{
			{Name: "review"},
		},
	}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// 3. Create a DB-backed trigger rule for the project.
	now := time.Now().UTC().Truncate(time.Second)
	rule := model.TriggerRule{
		ID:        uuid.New().String(),
		ProjectID: "proj-m9",
		Label:     "flux/review",
		Pipeline:  "review",
		Enabled:   true,
		Priority:  10,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := ruleRepo.Create(ctx, rule); err != nil {
		t.Fatalf("create trigger rule: %v", err)
	}

	// 4. Create TriggerService with DB-backed rules.
	pipelineSvc := domain.NewPipelineRunService(pipelineRepo)
	triggerSvc := domain.NewTriggerService(
		pipelineSvc,
		projectRepo,
		pipelineRepo,
		ruleRepo,
		"flux-bot",
	)

	// 5. Ticket with trigger label creates a pipeline run.
	ticket := model.Ticket{
		ID:        "ticket-m9-1",
		ProjectID: "proj-m9",
		Labels:    []string{"flux/review"},
		Status:    model.TicketStatusOpen,
	}
	if err := triggerSvc.CheckAndTrigger(ctx, ticket); err != nil {
		t.Fatalf("CheckAndTrigger: %v", err)
	}

	// Verify pipeline run was created.
	runs, err := pipelineRepo.List(ctx, repository.PipelineRunFilter{ProjectID: "proj-m9"})
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 pipeline run, got %d", len(runs))
	}
	if runs[0].Pipeline != "review" {
		t.Errorf("run pipeline = %q, want %q", runs[0].Pipeline, "review")
	}
	if runs[0].TicketID != "ticket-m9-1" {
		t.Errorf("run ticket = %q, want %q", runs[0].TicketID, "ticket-m9-1")
	}

	// 6. Deduplication: second trigger should not create a new run.
	if err := triggerSvc.CheckAndTrigger(ctx, ticket); err != nil {
		t.Fatalf("dedup trigger: %v", err)
	}
	runs, _ = pipelineRepo.List(ctx, repository.PipelineRunFilter{ProjectID: "proj-m9"})
	if len(runs) != 1 {
		t.Errorf("dedup failed: expected 1 run, got %d", len(runs))
	}

	// 7. Ticket without trigger label should not create a run.
	noLabel := model.Ticket{
		ID:        "ticket-m9-2",
		ProjectID: "proj-m9",
		Labels:    []string{"bug"},
	}
	if err := triggerSvc.CheckAndTrigger(ctx, noLabel); err != nil {
		t.Fatalf("no-trigger check: %v", err)
	}
	runs, _ = pipelineRepo.List(ctx, repository.PipelineRunFilter{ProjectID: "proj-m9"})
	if len(runs) != 1 {
		t.Errorf("expected 1 run (no new trigger), got %d", len(runs))
	}

	// 8. Auth: trigger endpoint requires JWT.
	srv := NewServer(WithJWTSecret(testJWTSecretBytes))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		ts.URL+"/api/v1/pipeline-runs",
		strings.NewReader(`{"ticket_id":"42"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /pipeline-runs: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthed trigger: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	t.Log("M9 smoke test passed")
}
