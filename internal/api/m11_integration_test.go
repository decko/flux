package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// TestM11_GitHubWebhooks verifies the M11 webhook flow.
func TestM11_GitHubWebhooks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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
	webhookSecretRepo := repository.NewSQLiteWebhookSecretRepository(sdb)

	project := model.Project{
		ID:      "proj-m11",
		Name:    "test",
		RepoURL: "https://github.com/decko/flux",
	}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	secret := "test-webhook-secret-32bytes!!"
	if err := webhookSecretRepo.Set(ctx, project.RepoURL, secret); err != nil {
		t.Fatalf("set webhook secret: %v", err)
	}

	srv := NewServer(
		WithJWTSecret(testJWTSecretBytes),
		WithWebhookSecretRepo(webhookSecretRepo),
	)
	ts := httptest.NewServer(srv)
	defer ts.Close()

	// Valid webhook → 200.
	payload := githubIssuePayload("labeled", "decko/flux", "contributor", []string{"flux/review"})
	sig := hmacSign([]byte(payload), secret)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "issues")
	req.Header.Set("X-Hub-Signature-256", sig)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST webhook: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("valid webhook: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Invalid signature → 401.
	req2, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader(payload))
	req2.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	resp2, _ := http.DefaultClient.Do(req2)
	if resp2 != nil {
		defer func() { _ = resp2.Body.Close() }()
		if resp2.StatusCode != http.StatusUnauthorized {
			t.Errorf("invalid sig: got %d, want %d", resp2.StatusCode, http.StatusUnauthorized)
		}
	}

	// Unknown repo → 200 (no info leak).
	req3, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		ts.URL+"/api/v1/webhooks/github", strings.NewReader("{}"))
	req3.Header.Set("X-GitHub-Event", "push")
	resp3, _ := http.DefaultClient.Do(req3)
	if resp3 != nil {
		defer func() { _ = resp3.Body.Close() }()
		if resp3.StatusCode != http.StatusUnauthorized {
			t.Errorf("unsiged request: got %d, want %d", resp3.StatusCode, http.StatusUnauthorized)
		}
	}

	t.Log("M11 smoke test passed")
}

func githubIssuePayload(action, fullName, sender string, labels []string) string {
	labelObjs := make([]map[string]string, len(labels))
	for i, l := range labels {
		labelObjs[i] = map[string]string{"name": l}
	}
	p := map[string]interface{}{
		"action": action,
		"repository": map[string]string{
			"full_name": fullName,
		},
		"sender": map[string]string{
			"login": sender,
		},
		"issue": map[string]interface{}{
			"number": 42,
			"title":  "Test issue",
			"labels": labelObjs,
		},
	}
	b, _ := json.Marshal(p)
	return string(b)
}
