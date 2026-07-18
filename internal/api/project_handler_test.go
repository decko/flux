package api

import (
	"database/sql"
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

	"github.com/decko/flux/internal/adapter/github"
	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// setupProjectServer creates an in-memory SQLite database, migrates it,
// creates a ProjectService-backed Server, and returns the server.
func setupProjectServer(t *testing.T) *Server {
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
	repo := repository.NewSQLiteProjectRepository(sdb)

	svc := domain.NewProjectService(repo)
	return NewServer(WithJWTSecret(testJWTSecretBytes), WithProjectService(svc))
}

// projectRequestBody builds a JSON request body with the given name and repo URL.
func projectRequestBody(name, repoURL string) string {
	data, _ := json.Marshal(map[string]string{
		"name":     name,
		"repo_url": repoURL,
	})
	return string(data)
}

// mustDecode decodes the JSON response body into the given target.
func mustDecode(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
}

// ─── Create ────────────────────────────────────────────────────────────────

func TestCreateProject(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("happy path", func(t *testing.T) {
		body := projectRequestBody("test-project", "https://github.com/example/test")
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/projects: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusCreated)
		}
		if loc := resp.Header.Get("Location"); loc == "" {
			t.Error("missing Location header")
		}

		var created model.Project
		mustDecode(t, resp, &created)

		if created.ID == "" {
			t.Error("expected non-empty project ID")
		}
		if created.Name != "test-project" {
			t.Errorf("got name %q, want %q", created.Name, "test-project")
		}
		if created.RepoURL != "https://github.com/example/test" {
			t.Errorf("got repo_url %q, want %q", created.RepoURL, "https://github.com/example/test")
		}
		if created.CreatedAt.IsZero() {
			t.Error("expected non-zero created_at")
		}
		if created.UpdatedAt.IsZero() {
			t.Error("expected non-zero updated_at")
		}
	})

	t.Run("missing name", func(t *testing.T) {
		body := `{"repo_url": "https://github.com/example/test"}`
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/projects: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		var errResp map[string]string
		mustDecode(t, resp, &errResp)
		if !strings.Contains(errResp["error"], "name is required") {
			t.Errorf("error message %q does not contain 'name is required'", errResp["error"])
		}
	})

	t.Run("missing repo_url", func(t *testing.T) {
		body := `{"name": "test-project"}`
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/projects: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}

		var errResp map[string]string
		mustDecode(t, resp, &errResp)
		if !strings.Contains(errResp["error"], "repo url is required") {
			t.Errorf("error message %q does not contain 'repo url is required'", errResp["error"])
		}
	})

	t.Run("malformed JSON", func(t *testing.T) {
		body := `{bad json}`
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /api/v1/projects: %v", err)
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

// ─── Get ───────────────────────────────────────────────────────────────────

func TestGetProject(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("happy path", func(t *testing.T) {
		// First create a project.
		body := projectRequestBody("get-test", "https://github.com/example/get-test")
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create project: %v", err)
		}
		var created model.Project
		mustDecode(t, resp, &created)
		_ = resp.Body.Close()

		// GET by ID.
		u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, created.ID)
		req = authedRequest(http.MethodGet, u, nil)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/projects/%s: %v", created.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var got model.Project
		mustDecode(t, resp, &got)
		if got.ID != created.ID {
			t.Errorf("got ID %q, want %q", got.ID, created.ID)
		}
		if got.Name != created.Name {
			t.Errorf("got name %q, want %q", got.Name, created.Name)
		}
		if got.RepoURL != created.RepoURL {
			t.Errorf("got repo_url %q, want %q", got.RepoURL, created.RepoURL)
		}
	})

	t.Run("not found", func(t *testing.T) {
		id := uuid.NewString()
		u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, id)
		req := authedRequest(http.MethodGet, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/projects/%s: %v", id, err)
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

// ─── List ──────────────────────────────────────────────────────────────────

func TestListProjects(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("empty list", func(t *testing.T) {
		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/projects", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/projects: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var projects []model.Project
		mustDecode(t, resp, &projects)
		if len(projects) != 0 {
			t.Errorf("got %d projects, want 0", len(projects))
		}
	})

	t.Run("list with items", func(t *testing.T) {
		for _, name := range []string{"proj-a", "proj-b"} {
			body := projectRequestBody(name, fmt.Sprintf("https://github.com/example/%s", name))
			req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("create project %s: %v", name, err)
			}
			_ = resp.Body.Close()
		}

		req := authedRequest(http.MethodGet, ts.URL+"/api/v1/projects", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /api/v1/projects: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var projects []model.Project
		mustDecode(t, resp, &projects)
		if len(projects) != 2 {
			t.Fatalf("got %d projects, want 2", len(projects))
		}
		names := make(map[string]bool)
		for _, p := range projects {
			names[p.Name] = true
		}
		if !names["proj-a"] {
			t.Error("expected project 'proj-a' in list")
		}
		if !names["proj-b"] {
			t.Error("expected project 'proj-b' in list")
		}
	})
}

// ─── Update ────────────────────────────────────────────────────────────────

func TestUpdateProject(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// createProject is a helper that POSTs a project and returns the decoded result.
	createProject := func(t *testing.T, name, repoURL string) model.Project {
		t.Helper()
		body := projectRequestBody(name, repoURL)
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create project: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var p model.Project
		mustDecode(t, resp, &p)
		return p
	}

	t.Run("happy path", func(t *testing.T) {
		p := createProject(t, "update-test", "https://github.com/example/update-test")

		updateBody := fmt.Sprintf(`{"id":%q,"name":"updated-name","repo_url":"https://github.com/example/updated"}`, p.ID)
		u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, p.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(updateBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/projects/%s: %v", p.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var updated model.Project
		mustDecode(t, resp, &updated)
		if updated.Name != "updated-name" {
			t.Errorf("got name %q, want %q", updated.Name, "updated-name")
		}
		if updated.RepoURL != "https://github.com/example/updated" {
			t.Errorf("got repo_url %q, want %q", updated.RepoURL, "https://github.com/example/updated")
		}
	})

	t.Run("not found", func(t *testing.T) {
		id := uuid.NewString()
		updateBody := fmt.Sprintf(`{"id":%q,"name":"ghost","repo_url":"https://github.com/example/ghost"}`, id)
		u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, id)
		req := authedRequest(http.MethodPut, u, strings.NewReader(updateBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/projects/%s: %v", id, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("invalid body - missing name", func(t *testing.T) {
		p := createProject(t, "validate-test", "https://github.com/example/validate")

		updateBody := fmt.Sprintf(`{"id":%q,"repo_url":"https://github.com/example/updated"}`, p.ID)
		u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, p.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(updateBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/projects/%s: %v", p.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("id mismatch", func(t *testing.T) {
		p := createProject(t, "mismatch-test", "https://github.com/example/mismatch")

		otherID := uuid.NewString()
		updateBody := fmt.Sprintf(`{"id":%q,"name":"mismatch","repo_url":"https://github.com/example/mismatch"}`, otherID)
		u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, p.ID)
		req := authedRequest(http.MethodPut, u, strings.NewReader(updateBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/v1/projects/%s: %v", p.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})
}

// ─── Delete ────────────────────────────────────────────────────────────────

func TestDeleteProject(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	t.Run("happy path", func(t *testing.T) {
		body := projectRequestBody("delete-test", "https://github.com/example/delete-test")
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create project: %v", err)
		}
		var created model.Project
		mustDecode(t, resp, &created)
		_ = resp.Body.Close()

		u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, created.ID)
		req = authedRequest(http.MethodDelete, u, nil)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("DELETE /api/v1/projects/%s: %v", created.ID, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNoContent)
		}
	})

	t.Run("verify deleted", func(t *testing.T) {
		body := projectRequestBody("verify-del", "https://github.com/example/verify-del")
		req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("create project: %v", err)
		}
		var created model.Project
		mustDecode(t, resp, &created)
		_ = resp.Body.Close()

		u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, created.ID)
		req = authedRequest(http.MethodDelete, u, nil)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("DELETE /api/v1/projects/%s: %v", created.ID, err)
		}
		_ = resp.Body.Close()

		// GET after delete should 404.
		req = authedRequest(http.MethodGet, u, nil)
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET after delete: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("got status %d, want %d after delete", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("not found", func(t *testing.T) {
		id := uuid.NewString()
		u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, id)
		req := authedRequest(http.MethodDelete, u, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("DELETE /api/v1/projects/%s: %v", id, err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})
}

// ─── Installation ID ───────────────────────────────────────────────────────

func TestHandleCreateProject_WithInstallationID(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := `{"name":"install-test","repo_url":"https://github.com/example/install","installation_id":42}`
	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/projects: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var created model.Project
	mustDecode(t, resp, &created)

	if created.InstallationID != 42 {
		t.Errorf("got installation_id %d, want %d", created.InstallationID, 42)
	}
}

func TestHandleGetProject_WithInstallationID(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Create a project with installation_id first.
	createBody := `{"name":"get-install","repo_url":"https://github.com/example/get-install","installation_id":99}`
	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	var created model.Project
	mustDecode(t, resp, &created)
	_ = resp.Body.Close()

	// GET the project and verify installation_id.
	u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, created.ID)
	req = authedRequest(http.MethodGet, u, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/v1/projects/%s: %v", created.ID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got model.Project
	mustDecode(t, resp, &got)

	if got.InstallationID != 99 {
		t.Errorf("got installation_id %d, want %d", got.InstallationID, 99)
	}
}

func TestHandleUpdateProject_WithInstallationID(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Create a project first.
	createBody := `{"name":"update-install","repo_url":"https://github.com/example/update-install","installation_id":10}`
	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	var created model.Project
	mustDecode(t, resp, &created)
	_ = resp.Body.Close()

	// Update the installation_id.
	updateBody := fmt.Sprintf(`{"id":%q,"name":"update-install","repo_url":"https://github.com/example/update-install","installation_id":200}`, created.ID)
	u := fmt.Sprintf("%s/api/v1/projects/%s", ts.URL, created.ID)
	req = authedRequest(http.MethodPut, u, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/v1/projects/%s: %v", created.ID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var updated model.Project
	mustDecode(t, resp, &updated)

	if updated.InstallationID != 200 {
		t.Errorf("got installation_id %d, want %d", updated.InstallationID, 200)
	}
}

// ─── Method Not Allowed ────────────────────────────────────────────────────

func TestProjectMethodNotAllowed(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "patch to projects list", method: http.MethodPatch, path: "/api/v1/projects"},
		{name: "patch to project detail", method: http.MethodPatch, path: "/api/v1/projects/some-id"},
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

// ─── Authz: Non-admin create/update gate ───────────────────────────────────

// TestCreateProject_NonAdminForbidden verifies that a non-admin user cannot
// create a project. The test uses a JWT with role "user" and expects 403
// Forbidden because project creation configures outbound access with server
// credentials and must be restricted to admins.
func TestCreateProject_NonAdminForbidden(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := projectRequestBody("evil-project", "https://github.com/evil/repo")
	req := nonAdminRequest(http.MethodPost, ts.URL+"/api/v1/projects", body)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/v1/projects (non-admin): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-admin POST /projects: got %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestUpdateProject_NonAdminForbidden verifies that a non-admin user cannot
// update a project. First a project is created by an admin, then a non-admin
// JWT attempts a PUT — expect 403 Forbidden.
func TestUpdateProject_NonAdminForbidden(t *testing.T) {
	srv := setupProjectServer(t)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Create a project as admin.
	createBody := projectRequestBody("target-project", "https://github.com/example/target")
	createReq := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("admin create project: %v", err)
	}
	var created model.Project
	mustDecode(t, resp, &created)
	_ = resp.Body.Close()

	// Attempt update as non-admin.
	updateBody := `{"name":"hijacked","repo_url":"https://github.com/evil/hijacked"}`
	updateReq := nonAdminRequest(http.MethodPut, ts.URL+"/api/v1/projects/"+created.ID, updateBody)
	updateReq.Header.Set("Content-Type", "application/json")

	updateResp, err := http.DefaultClient.Do(updateReq)
	if err != nil {
		t.Fatalf("PUT /api/v1/projects/%s (non-admin): %v", created.ID, err)
	}
	defer func() { _ = updateResp.Body.Close() }()

	if updateResp.StatusCode != http.StatusForbidden {
		t.Errorf("non-admin PUT /projects/{id}: got %d, want %d", updateResp.StatusCode, http.StatusForbidden)
	}

	// Verify the project was NOT updated (name unchanged).
	getReq := authedRequest(http.MethodGet, ts.URL+"/api/v1/projects/"+created.ID, nil)
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("admin get project: %v", err)
	}
	defer func() { _ = getResp.Body.Close() }()

	var fetched model.Project
	mustDecode(t, getResp, &fetched)
	if fetched.Name != "target-project" {
		t.Errorf("project name was changed by non-admin: got %q, want %q", fetched.Name, "target-project")
	}
}

// ─── Fire-and-forget context detachment ──────────────────────────────────

// TestCreateProject_NonBlockingWebhook verifies that project creation returns
// 201 without blocking on webhook registration. A WebhookCreator is injected
// so the fire-and-forget goroutine path is actually exercised.
func TestCreateProject_NonBlockingWebhook(t *testing.T) {
	srv := setupProjectServer(t)

	srv.webhookCreator = domain.NewWebhookCreator(nil, nil, nil, nil)

	ts := httptest.NewServer(srv)
	defer ts.Close()

	body := projectRequestBody("fire-forget", "https://github.com/example/fire-forget")
	req := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("POST /api/v1/projects: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("handler took %v, expected < 500ms", elapsed)
	}
}

// TestDeleteProject_NonBlockingWebhook verifies that project deletion returns
// 204 without blocking on webhook unregistration. A mock GitHub server and
// non-zero WebhookID ensure the fire-and-forget goroutine path is entered.
func TestDeleteProject_NonBlockingWebhook(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("configure sqlite: %v", err)
	}
	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sdb := sqlx.NewDb(db, "sqlite")
	projRepo := repository.NewSQLiteProjectRepository(sdb)
	projSvc := domain.NewProjectService(projRepo)

	pemKey, _ := generateTestKeyGH(t)
	appAuth, err := github.NewAppAuth("12345", pemKey)
	if err != nil {
		t.Fatalf("NewAppAuth: %v", err)
	}
	mockGH := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(mockGH.Close)
	appAuth.SetHTTPClient(mockGH.Client())
	appAuth.SetBaseURL(mockGH.URL)

	srv := NewServer(
		WithJWTSecret(testJWTSecretBytes),
		WithProjectService(projSvc),
		WithAppAuth(appAuth),
	)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	createBody := projectRequestBody("to-delete", "https://github.com/example/to-delete")
	createReq := authedRequest(http.MethodPost, ts.URL+"/api/v1/projects", strings.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := http.DefaultClient.Do(createReq)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	var created model.Project
	mustDecode(t, createResp, &created)
	_ = createResp.Body.Close()

	created.WebhookID = 42
	if err := projRepo.Update(t.Context(), created); err != nil {
		t.Fatalf("set webhook_id: %v", err)
	}

	start := time.Now()
	delReq := authedRequest(http.MethodDelete, ts.URL+"/api/v1/projects/"+created.ID, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("DELETE /api/v1/projects: %v", err)
	}
	defer func() { _ = delResp.Body.Close() }()

	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("got status %d, want %d", delResp.StatusCode, http.StatusNoContent)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("handler took %v, expected < 500ms", elapsed)
	}
}
