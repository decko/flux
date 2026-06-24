package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/adapter/scm"
	"github.com/decko/flux/internal/adapter/ticket"
	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/repository"
)

// ─── Mock GitHub Server ────────────────────────────────────────────────────

// mockGitHub is an httptest server that simulates GitHub's API for
// integration testing the full M2 pipeline: adapter → sync → API.
type mockGitHub struct {
	*httptest.Server
	issues   []mockIssue
	pullReqs []mockPR
	reviews  map[int][]mockReview
}

type mockIssue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Body      string    `json:"body"`
	Labels    []ghLabel `json:"labels"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type mockPR struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	State     string  `json:"state"`
	HTMLURL   string  `json:"html_url"`
	Body      string  `json:"body"`
	MergedAt  *string `json:"merged_at"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type mockReview struct {
	ID          int    `json:"id"`
	User        ghUser `json:"user"`
	State       string `json:"state"`
	Body        string `json:"body"`
	SubmittedAt string `json:"submitted_at"`
}

type ghUser struct {
	Login string `json:"login"`
}

func newMockGitHub() *mockGitHub {
	mg := &mockGitHub{
		reviews: make(map[int][]mockReview),
	}
	mg.Server = httptest.NewServer(http.HandlerFunc(mg.handle))
	return mg
}

func (mg *mockGitHub) addIssue(number int, title, state string) {
	ts := time.Now().Format(time.RFC3339)
	mg.issues = append(mg.issues, mockIssue{Number: number, Title: title, State: state, CreatedAt: ts, UpdatedAt: ts})
}

func (mg *mockGitHub) addPR(number int, title, state string, mergedAt *string) {
	ts := time.Now().Format(time.RFC3339)
	mg.pullReqs = append(mg.pullReqs, mockPR{
		Number: number, Title: title, State: state,
		HTMLURL:   fmt.Sprintf("https://github.com/test/repo/pull/%d", number),
		MergedAt:  mergedAt,
		CreatedAt: ts, UpdatedAt: ts,
	})
}

func (mg *mockGitHub) addReview(prNumber int, reviewer, state, body string) {
	mg.reviews[prNumber] = append(mg.reviews[prNumber], mockReview{
		ID:          len(mg.reviews[prNumber]) + 1,
		User:        ghUser{Login: reviewer},
		State:       state,
		Body:        body,
		SubmittedAt: time.Now().Format(time.RFC3339),
	})
}

func (mg *mockGitHub) handle(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth != "Bearer test-github-token" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")

	switch {
	case path == "repos/test/flux":
		mg.handleRepo(w)
	case path == "repos/test/flux/issues":
		mg.handleIssues(w, r)
	case strings.HasPrefix(path, "repos/test/flux/issues/"):
		mg.handleSingleIssue(w, r)
	case path == "repos/test/flux/pulls":
		mg.handlePRs(w, r)
	case strings.HasPrefix(path, "repos/test/flux/pulls/") && strings.HasSuffix(path, "/reviews"):
		mg.handleReviews(w, r)
	case strings.HasPrefix(path, "repos/test/flux/pulls/"):
		mg.handleSinglePR(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (mg *mockGitHub) handleRepo(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"full_name": "test/flux"})
}

func (mg *mockGitHub) handleIssues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Remaining", "4999")
	w.WriteHeader(http.StatusOK)
	if r.URL.Query().Get("page") == "" || r.URL.Query().Get("page") == "1" {
		_ = json.NewEncoder(w).Encode(mg.issues)
		return
	}
	_ = json.NewEncoder(w).Encode([]mockIssue{})
}

func (mg *mockGitHub) handleSingleIssue(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	numStr := parts[len(parts)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	for _, issue := range mg.issues {
		if issue.Number == num {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(issue)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (mg *mockGitHub) handlePRs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Remaining", "4999")
	w.WriteHeader(http.StatusOK)
	if r.URL.Query().Get("page") == "" || r.URL.Query().Get("page") == "1" {
		_ = json.NewEncoder(w).Encode(mg.pullReqs)
		return
	}
	_ = json.NewEncoder(w).Encode([]mockPR{})
}

func (mg *mockGitHub) handleSinglePR(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	numStr := parts[len(parts)-1]
	num, _ := strconv.Atoi(numStr)
	for _, pr := range mg.pullReqs {
		if pr.Number == num {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(pr)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}

func (mg *mockGitHub) handleReviews(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	numStr := parts[4] // repos/test/flux/pulls/{NUM}/reviews
	num, _ := strconv.Atoi(numStr)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if revs, ok := mg.reviews[num]; ok {
		_ = json.NewEncoder(w).Encode(revs)
	} else {
		_ = json.NewEncoder(w).Encode([]mockReview{})
	}
}

// ─── Integration Test ──────────────────────────────────────────────────────

// TestM2FullPipeline_EndToEndSync verifies the end-to-end M2 flow:
// mock GitHub → adapters → sync service → repository → API endpoints.
func TestM2FullPipeline_EndToEndSync(t *testing.T) {
	t.Parallel()

	// 1. Set up mock GitHub with test data.
	gh := newMockGitHub()
	defer gh.Close()

	gh.addIssue(1, "Fix login bug", "open")
	gh.addIssue(2, "Add dark mode", "closed")
	gh.addPR(10, "PR: fix login", "open", nil)
	now := time.Now().Format(time.RFC3339)
	gh.addPR(11, "PR: dark mode", "closed", &now) // merged
	gh.addReview(10, "reviewer1", "APPROVED", "LGTM")
	gh.addReview(10, "reviewer2", "CHANGES_REQUESTED", "needs tests")

	// 2. Create in-memory repositories.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()
	ticketRepo := repository.NewSQLiteTicketRepository(db)
	prRepo := repository.NewSQLitePullRequestRepository(db)
	if err := ticketRepo.Migrate(t.Context()); err != nil {
		t.Fatalf("ticket migrate: %v", err)
	}
	if err := prRepo.Migrate(t.Context()); err != nil {
		t.Fatalf("pr migrate: %v", err)
	}

	// 3. Create real GitHub adapters pointed at the mock server.
	ticketAdapter := ticket.NewGitHubAdapter("test", "flux", "test-github-token", nil,
		ticket.WithBaseURL(gh.URL))
	scmAdapter := scm.NewGitHubAdapter("test", "flux", "test-github-token", nil,
		scm.WithBaseURL(gh.URL))

	// 4. Create the sync service.
	syncSvc := domain.NewSyncService(ticketRepo, prRepo, ticketAdapter, scmAdapter, time.Hour)

	// 5. Create the API server with all M2 wiring.
	adapters := map[string]domain.AdapterInfo{
		"github": {Type: "github", Name: "github", Health: "healthy"},
	}
	srv := NewServer(
		WithJWTSecret(testJWTSecretBytes),
		WithSyncService(syncSvc),
		WithAdapters(adapters),
	)

	// 6. Run a sync.
	syncErr := syncSvc.SyncNow(t.Context(), "")
	if syncErr != nil {
		t.Fatalf("SyncNow failed: %v", syncErr)
	}

	// 7. Verify tickets were synced.
	status := syncSvc.Status()
	if status.TicketsSynced != 2 {
		t.Errorf("got %d tickets synced, want 2", status.TicketsSynced)
	}
	if status.PRsSynced != 2 {
		t.Errorf("got %d PRs synced, want 2", status.PRsSynced)
	}
	if status.LastSyncAt == nil {
		t.Error("LastSyncAt should not be nil after sync")
	}
	if status.LastSyncError != "" {
		t.Errorf("LastSyncError should be empty, got %q", status.LastSyncError)
	}

	// 8. Verify data is persisted in repositories.
	tickets, err := ticketRepo.List(t.Context(), repository.TicketFilter{ProjectID: ""})
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	if len(tickets) != 2 {
		t.Fatalf("got %d tickets in repo, want 2", len(tickets))
	}
	foundFix, foundDark := false, false
	for _, tk := range tickets {
		if tk.ExternalID == "1" && tk.Title == "Fix login bug" && tk.Status == "open" {
			foundFix = true
		}
		if tk.ExternalID == "2" && tk.Title == "Add dark mode" && tk.Status == "closed" {
			foundDark = true
		}
		if tk.ID == "" {
			t.Error("ticket ID should be assigned")
		}
		if tk.Source != "github" {
			t.Errorf("ticket source should be github, got %q", tk.Source)
		}
	}
	if !foundFix || !foundDark {
		t.Errorf("expected tickets not found: fix=%v dark=%v", foundFix, foundDark)
	}

	prs, err := prRepo.List(t.Context(), repository.PullRequestFilter{ProjectID: ""})
	if err != nil {
		t.Fatalf("list PRs: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("got %d PRs in repo, want 2", len(prs))
	}
	foundMerged := false
	for _, pr := range prs {
		if pr.ExternalID == "11" && pr.Status == "merged" {
			foundMerged = true
		}
	}
	if !foundMerged {
		t.Error("PR #11 should have status 'merged'")
	}

	// 9. Verify API endpoints return the synced data.
	ts := httptest.NewServer(srv)
	defer ts.Close()

	authHeader := "Bearer " + generateTestToken()

	resp, err := httpGet(ts.URL+"/api/v1/sync/status", authHeader)
	if err != nil {
		t.Fatalf("GET /sync/status: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /sync/status: got %d, want 200", resp.StatusCode)
	}
	var syncResp syncStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		t.Fatalf("decode sync status: %v", err)
	}
	_ = resp.Body.Close()
	if syncResp.TicketsSynced != 2 {
		t.Errorf("API TicketsSynced: got %d, want 2", syncResp.TicketsSynced)
	}
	if syncResp.PRsSynced != 2 {
		t.Errorf("API PRsSynced: got %d, want 2", syncResp.PRsSynced)
	}
	if syncResp.LastSyncAt == nil {
		t.Error("API LastSyncAt should not be null")
	}

	// Adapters endpoint.
	resp, err = httpGet(ts.URL+"/api/v1/adapters", authHeader)
	if err != nil {
		t.Fatalf("GET /adapters: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /adapters: got %d, want 200", resp.StatusCode)
	}
	var adapterList []domain.AdapterInfo
	if err := json.NewDecoder(resp.Body).Decode(&adapterList); err != nil {
		t.Fatalf("decode adapters: %v", err)
	}
	if len(adapterList) != 1 || adapterList[0].Type != "github" {
		t.Errorf("unexpected adapter list: %+v", adapterList)
	}

	// 10. Trigger another sync — should dedup (upsert, not duplicate).
	err = syncSvc.SyncNow(t.Context(), "")
	if err != nil {
		t.Fatalf("second SyncNow failed: %v", err)
	}
	tickets2, _ := ticketRepo.List(t.Context(), repository.TicketFilter{})
	if len(tickets2) != 2 {
		t.Errorf("upsert should not duplicate: got %d tickets, want 2", len(tickets2))
	}

	// 11. Verify 401 when unauthenticated.
	resp, err = httpGet(ts.URL+"/api/v1/sync/status", "")
	if err != nil {
		t.Fatalf("GET /sync/status (unauth): %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated request: got %d, want 401", resp.StatusCode)
	}

	t.Log("✅ M2 full pipeline: github → adapters → sync → repo → API — all verified")
}

func httpGet(url, authHeader string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	return http.DefaultClient.Do(req)
}
