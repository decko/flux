package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/repository"
)

// TestM6_TrustworthyAudit verifies the M6 smoke test:
//   - Audit events recorded on domain operations
//   - Audit endpoint returns events with correct actor_id
//   - Unauthenticated → 401, non-admin → 403
func TestM6_TrustworthyAudit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// 1. In-memory database.
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
	auditRepo := repository.NewSQLiteAuditRepository(sdb)

	// 2. Create audited project service.
	auditSvc := domain.NewAuditService(auditRepo)
	projectSvc := domain.NewProjectService(projectRepo, domain.WithAuditService(auditSvc))

	// 3. Create server.
	srv := NewServer(
		WithJWTSecret(testJWTSecretBytes),
		WithProjectService(projectSvc),
		WithAuditService(auditSvc),
	)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	adminHeader := "Bearer " + generateTestToken()
	viewerHeader := "Bearer " + makeToken("viewer", "user")

	// 4. Create a project → produces audit event.
	body := `{"name":"test","repo_url":"https://github.com/test/repo"}`
	resp, err := httpPost(ctx, ts.URL+"/api/v1/projects", adminHeader, body)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create project: got %d, want 201", resp.StatusCode)
	}

	// 5. GET /audit-events as admin → 200 with event.
	resp, err = httpGet(ts.URL+"/api/v1/audit-events", adminHeader)
	if err != nil {
		t.Fatalf("get audit: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get audit: got %d", resp.StatusCode)
	}

	var events []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("no audit events")
	}
	for _, e := range events {
		if e["action"] == "project.created" {
			if e["actor_id"] == "" {
				t.Error("actor_id empty")
			}
			t.Logf("audit: project.created actor=%v", e["actor_id"])
		}
	}

	// 6. Unauthenticated → 401.
	resp, err = httpGet(ts.URL+"/api/v1/audit-events", "")
	if err != nil {
		t.Fatalf("get unauth: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: got %d, want 401", resp.StatusCode)
	}

	// 7. Non-admin → 403.
	resp, err = httpGet(ts.URL+"/api/v1/audit-events", viewerHeader)
	if err != nil {
		t.Fatalf("get viewer: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("viewer: got %d, want 403", resp.StatusCode)
	}

	t.Log("M6 smoke test passed")
}

func makeToken(sub, role string) string {
	claims := jwt.MapClaims{
		"sub":   sub,
		"role":  role,
		"email": sub + "@flux.dev",
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(testJWTSecretBytes)
	return tokenStr
}

func httpPost(ctx context.Context, url, authHeader, body string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}
