package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"

	"github.com/decko/flux/internal/adapter/orchestrator"
	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// setupPipelineServer creates an in-memory SQLite database, migrates it,
// creates a PipelineRunService-backed Server, and returns the server along with
// a seed function for populating pipeline runs into the same database.
func setupPipelineServer(t *testing.T) (*Server, func(t *testing.T, run model.PipelineRun) model.PipelineRun) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}

	repo := repository.NewSQLitePipelineRunRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to run migration: %v", err)
	}

	svc := domain.NewPipelineRunService(repo)
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithPipelineService(svc))

	seed := func(t *testing.T, run model.PipelineRun) model.PipelineRun {
		t.Helper()
		if run.ID == "" {
			run.ID = uuid.NewString()
		}
		if run.Status == "" {
			run.Status = model.RunStatusPending
		}
		if run.StartedAt.IsZero() {
			run.StartedAt = time.Now().UTC().Truncate(time.Second)
		}
		if err := svc.Create(context.Background(), run); err != nil {
			t.Fatalf("failed to seed pipeline run: %v", err)
		}
		return run
	}

	return srv, seed
}

// ─── Stub: OrchestratorAdapter ──────────────────────────────────────────────

type stubOrchestrator struct {
	mu           sync.Mutex
	triggeredIDs []string
	canceledIDs  []string
}

func (s *stubOrchestrator) Name() string { return "stub" }

func (s *stubOrchestrator) Trigger(_ context.Context, run model.PipelineRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.triggeredIDs = append(s.triggeredIDs, run.ID)
	return nil
}

func (s *stubOrchestrator) Cancel(_ context.Context, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canceledIDs = append(s.canceledIDs, runID)
	return nil
}

func (s *stubOrchestrator) Status(_ context.Context, _ string) (*model.PipelineRun, error) {
	return nil, nil
}

func (s *stubOrchestrator) Logs(_ context.Context, _ string) (<-chan orchestrator.LogEntry, error) {
	return nil, nil
}

func (s *stubOrchestrator) Health(_ context.Context) error {
	return nil
}

// setupPipelineServerWithOrch creates an in-memory SQLite database, migrates it,
// creates a PipelineRunService with the given orchestrator, and returns the
// server along with a seed function for populating pipeline runs.
func setupPipelineServerWithOrch(t *testing.T, orch orchestrator.OrchestratorAdapter) (*Server, func(t *testing.T, run model.PipelineRun) model.PipelineRun) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}

	repo := repository.NewSQLitePipelineRunRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to run migration: %v", err)
	}

	svc := domain.NewPipelineRunService(repo, domain.WithOrchestrator(orch))
	srv := NewServer(WithJWTSecret(testJWTSecretBytes), WithPipelineService(svc))

	seed := func(t *testing.T, run model.PipelineRun) model.PipelineRun {
		t.Helper()
		if run.ID == "" {
			run.ID = uuid.NewString()
		}
		if run.Status == "" {
			run.Status = model.RunStatusPending
		}
		if run.StartedAt.IsZero() {
			run.StartedAt = time.Now().UTC().Truncate(time.Second)
		}
		if err := svc.Create(context.Background(), run); err != nil {
			t.Fatalf("failed to seed pipeline run: %v", err)
		}
		return run
	}

	return srv, seed
}

// ─── List ─────────────────────────────────────────────────────────────────

func TestListPipelineRuns(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		srv, _ := setupPipelineServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/pipeline-runs", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pipeline-runs: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body pipelineRunPage
		mustDecode(t, resp, &body)
		if body.Items == nil {
			t.Fatal("expected non-nil items array, got nil")
		}
		if len(body.Items) != 0 {
			t.Errorf("got %d items, want 0", len(body.Items))
		}
	})

	t.Run("with items", func(t *testing.T) {
		srv, seed := setupPipelineServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.PipelineRun{
			ProjectID:    "proj-1",
			TicketID:     "ticket-1",
			Orchestrator: "soda",
			Pipeline:     "plan",
			Status:       model.RunStatusPending,
		})
		seed(t, model.PipelineRun{
			ProjectID:    "proj-1",
			TicketID:     "ticket-2",
			Orchestrator: "soda",
			Pipeline:     "code-review",
			Status:       model.RunStatusCompleted,
		})

		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/pipeline-runs", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pipeline-runs: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body pipelineRunPage
		mustDecode(t, resp, &body)
		if len(body.Items) != 2 {
			t.Errorf("got %d items, want 2", len(body.Items))
		}
	})

	t.Run("filter by project_id", func(t *testing.T) {
		srv, seed := setupPipelineServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.PipelineRun{
			ProjectID:    "proj-1",
			TicketID:     "ticket-1",
			Orchestrator: "soda",
			Pipeline:     "plan",
			Status:       model.RunStatusPending,
		})
		seed(t, model.PipelineRun{
			ProjectID:    "proj-2",
			TicketID:     "ticket-2",
			Orchestrator: "soda",
			Pipeline:     "code-review",
			Status:       model.RunStatusRunning,
		})

		u := ts.URL + "/api/v1/pipeline-runs?project_id=proj-1"
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pipeline-runs?project_id=proj-1: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body pipelineRunPage
		mustDecode(t, resp, &body)
		if len(body.Items) != 1 {
			t.Fatalf("got %d items, want 1", len(body.Items))
		}
		if body.Items[0].ProjectID != "proj-1" {
			t.Errorf("got project_id %q, want %q", body.Items[0].ProjectID, "proj-1")
		}
	})

	t.Run("filter by ticket_id", func(t *testing.T) {
		srv, seed := setupPipelineServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.PipelineRun{
			ProjectID:    "proj-1",
			TicketID:     "ticket-1",
			Orchestrator: "soda",
			Pipeline:     "plan",
			Status:       model.RunStatusPending,
		})
		seed(t, model.PipelineRun{
			ProjectID:    "proj-1",
			TicketID:     "ticket-2",
			Orchestrator: "soda",
			Pipeline:     "code-review",
			Status:       model.RunStatusRunning,
		})

		u := ts.URL + "/api/v1/pipeline-runs?ticket_id=ticket-1"
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pipeline-runs?ticket_id=ticket-1: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body pipelineRunPage
		mustDecode(t, resp, &body)
		if len(body.Items) != 1 {
			t.Fatalf("got %d items, want 1", len(body.Items))
		}
		if body.Items[0].TicketID != "ticket-1" {
			t.Errorf("got ticket_id %q, want %q", body.Items[0].TicketID, "ticket-1")
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		srv, seed := setupPipelineServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		seed(t, model.PipelineRun{
			ProjectID:    "proj-1",
			TicketID:     "ticket-1",
			Orchestrator: "soda",
			Pipeline:     "plan",
			Status:       model.RunStatusPending,
		})
		seed(t, model.PipelineRun{
			ProjectID:    "proj-1",
			TicketID:     "ticket-2",
			Orchestrator: "soda",
			Pipeline:     "code-review",
			Status:       model.RunStatusCompleted,
		})

		u := ts.URL + "/api/v1/pipeline-runs?status=pending"
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pipeline-runs?status=pending: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var body pipelineRunPage
		mustDecode(t, resp, &body)
		if len(body.Items) != 1 {
			t.Fatalf("got %d items, want 1", len(body.Items))
		}
		if body.Items[0].Status != model.RunStatusPending {
			t.Errorf("got status %q, want %q", body.Items[0].Status, model.RunStatusPending)
		}
	})
}

// ─── Get ──────────────────────────────────────────────────────────────────

func TestGetPipelineRun(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		srv, seed := setupPipelineServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		orig := seed(t, model.PipelineRun{
			ProjectID:    "proj-1",
			TicketID:     "ticket-1",
			Orchestrator: "soda",
			Pipeline:     "plan",
			Status:       model.RunStatusPending,
		})

		u := fmt.Sprintf("%s/api/v1/pipeline-runs/%s", ts.URL, orig.ID)
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pipeline-runs/%s: %v", orig.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var got model.PipelineRun
		mustDecode(t, resp, &got)
		if got.ID != orig.ID {
			t.Errorf("got ID %q, want %q", got.ID, orig.ID)
		}
		if got.ProjectID != orig.ProjectID {
			t.Errorf("got project_id %q, want %q", got.ProjectID, orig.ProjectID)
		}
		if got.TicketID != orig.TicketID {
			t.Errorf("got ticket_id %q, want %q", got.TicketID, orig.TicketID)
		}
		if got.Status != orig.Status {
			t.Errorf("got status %q, want %q", got.Status, orig.Status)
		}
		if got.Pipeline != orig.Pipeline {
			t.Errorf("got pipeline %q, want %q", got.Pipeline, orig.Pipeline)
		}
	})

	t.Run("not found", func(t *testing.T) {
		srv, _ := setupPipelineServer(t)
		ts := httptest.NewServer(srv)
		defer ts.Close()

		id := uuid.NewString()
		u := fmt.Sprintf("%s/api/v1/pipeline-runs/%s", ts.URL, id)
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/pipeline-runs/%s: %v", id, err)
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
	})
}

// ─── Create ───────────────────────────────────────────────────────────────

// pipelineRunRequestBody builds a JSON request body for a pipeline run create.
func pipelineRunRequestBody(projectID, ticketID, orchestrator, pipeline, status string) string {
	if status == "" {
		return fmt.Sprintf(`{"project_id":%q,"ticket_id":%q,"orchestrator":%q,"pipeline":%q}`,
			projectID, ticketID, orchestrator, pipeline)
	}
	return fmt.Sprintf(`{"project_id":%q,"ticket_id":%q,"orchestrator":%q,"pipeline":%q,"status":%q}`,
		projectID, ticketID, orchestrator, pipeline, status)
}

func TestCreatePipelineRun(t *testing.T) {
	srv, _ := setupPipelineServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("happy path", func(t *testing.T) {
		body := pipelineRunRequestBody("proj-1", "ticket-1", "soda", "plan", "")
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/pipeline-runs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/pipeline-runs: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusCreated)
		}
		if loc := resp.Header.Get("Location"); loc == "" {
			t.Error("missing Location header")
		}

		var created model.PipelineRun
		mustDecode(t, resp, &created)

		if created.ID == "" {
			t.Error("expected non-empty pipeline run ID")
		}
		if created.ProjectID != "proj-1" {
			t.Errorf("got project_id %q, want %q", created.ProjectID, "proj-1")
		}
		if created.TicketID != "ticket-1" {
			t.Errorf("got ticket_id %q, want %q", created.TicketID, "ticket-1")
		}
		if created.Orchestrator != "soda" {
			t.Errorf("got orchestrator %q, want %q", created.Orchestrator, "soda")
		}
		if created.Pipeline != "plan" {
			t.Errorf("got pipeline %q, want %q", created.Pipeline, "plan")
		}
		if created.Status != model.RunStatusPending {
			t.Errorf("got status %q, want %q", created.Status, model.RunStatusPending)
		}
		if created.StartedAt.IsZero() {
			t.Error("expected non-zero started_at")
		}
	})

	t.Run("missing project_id", func(t *testing.T) {
		body := `{"ticket_id":"ticket-1","orchestrator":"soda","pipeline":"plan"}`
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/pipeline-runs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/pipeline-runs: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		var errResp map[string]string
		mustDecode(t, resp, &errResp)
		if !strings.Contains(errResp["error"], "project id is required") {
			t.Errorf("error message %q does not contain 'project id is required'", errResp["error"])
		}
	})

	t.Run("missing ticket_id", func(t *testing.T) {
		body := `{"project_id":"proj-1","orchestrator":"soda","pipeline":"plan"}`
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/pipeline-runs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/pipeline-runs: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		var errResp map[string]string
		mustDecode(t, resp, &errResp)
		if !strings.Contains(errResp["error"], "ticket id is required") {
			t.Errorf("error message %q does not contain 'ticket id is required'", errResp["error"])
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		body := `{"project_id":"proj-1","ticket_id":"ticket-1","orchestrator":"soda","pipeline":"plan","status":"bogus"}`
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/pipeline-runs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/pipeline-runs: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		var errResp map[string]string
		mustDecode(t, resp, &errResp)
		if !strings.Contains(errResp["error"], "invalid") {
			t.Errorf("error message %q does not contain 'invalid'", errResp["error"])
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		body := `{bad json}`
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/pipeline-runs", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/pipeline-runs: %v", err)
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
	})
}

// ─── Method Not Allowed ───────────────────────────────────────────────────

func TestPipelineRunMethodNotAllowed(t *testing.T) {
	srv, _ := setupPipelineServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "put to pipeline-runs list", method: http.MethodPut, path: "/api/v1/pipeline-runs"},
		{name: "delete to pipeline-runs list", method: http.MethodDelete, path: "/api/v1/pipeline-runs"},
		{name: "put to pipeline-run detail", method: http.MethodPut, path: "/api/v1/pipeline-runs/some-id"},
		{name: "delete to pipeline-run detail", method: http.MethodDelete, path: "/api/v1/pipeline-runs/some-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := authedRequest(tt.method, ts.URL+tt.path, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", tt.method, tt.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
			}

			var errResp map[string]string
			mustDecode(t, resp, &errResp)
			if _, ok := errResp["error"]; !ok {
				t.Error("JSON response missing 'error' field")
			}
		})
	}
}

// ─── Trigger / Cancel ───────────────────────────────────────────────────────

func TestHandleTriggerPipelineRun(t *testing.T) {
	orch := &stubOrchestrator{}
	srv, seed := setupPipelineServerWithOrch(t, orch)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	run := seed(t, model.PipelineRun{
		ProjectID:    "proj-1",
		TicketID:     "ticket-1",
		Orchestrator: "soda",
		Pipeline:     "plan",
		Status:       model.RunStatusPending,
	})

	u := fmt.Sprintf("%s/api/v1/pipeline-runs/%s/trigger", ts.URL, run.ID)
	req := authedRequest(http.MethodPost, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	// Verify the orchestrator was called with the correct run ID.
	orch.mu.Lock()
	triggered := len(orch.triggeredIDs) == 1 && orch.triggeredIDs[0] == run.ID
	orch.mu.Unlock()
	if !triggered {
		t.Errorf("expected orchestrator.Trigger to be called with %q; calls: %v", run.ID, orch.triggeredIDs)
	}
}

func TestHandleTriggerPipelineRun_NotFound(t *testing.T) {
	orch := &stubOrchestrator{}
	srv, _ := setupPipelineServerWithOrch(t, orch)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	u := fmt.Sprintf("%s/api/v1/pipeline-runs/%s/trigger", ts.URL, uuid.NewString())
	req := authedRequest(http.MethodPost, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleCancelPipelineRun(t *testing.T) {
	orch := &stubOrchestrator{}
	srv, seed := setupPipelineServerWithOrch(t, orch)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	run := seed(t, model.PipelineRun{
		ProjectID:    "proj-1",
		TicketID:     "ticket-1",
		Orchestrator: "soda",
		Pipeline:     "plan",
		Status:       model.RunStatusRunning,
	})

	u := fmt.Sprintf("%s/api/v1/pipeline-runs/%s/cancel", ts.URL, run.ID)
	req := authedRequest(http.MethodPost, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Verify the orchestrator was called with the correct run ID.
	orch.mu.Lock()
	canceled := len(orch.canceledIDs) == 1 && orch.canceledIDs[0] == run.ID
	orch.mu.Unlock()
	if !canceled {
		t.Errorf("expected orchestrator.Cancel to be called with %q; calls: %v", run.ID, orch.canceledIDs)
	}
}

func TestHandleTriggerPipelineRun_Unauthorized(t *testing.T) {
	orch := &stubOrchestrator{}
	srv, seed := setupPipelineServerWithOrch(t, orch)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	run := seed(t, model.PipelineRun{
		ProjectID:    "proj-1",
		TicketID:     "ticket-1",
		Orchestrator: "soda",
		Pipeline:     "plan",
	})

	// No Authorization header — should be rejected.
	u := fmt.Sprintf("%s/api/v1/pipeline-runs/%s/trigger", ts.URL, run.ID)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", u, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
