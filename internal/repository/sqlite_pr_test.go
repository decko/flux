package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/migration"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Setup ─────────────────────────────────────────────────────────────────

// setupPRTestDB opens an in-memory SQLite database, configures it for
// SQLite use (pool + WAL), creates the pull_requests table via migration, and
// returns a SQLitePullRequestRepository for testing.
func setupPRTestDB(t *testing.T) (*sql.DB, *repository.SQLitePullRequestRepository) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}

	if err := migration.Up(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewSQLitePullRequestRepository(db)
	return db, repo
}

// ─── Create ────────────────────────────────────────────────────────────────

func TestSQLitePRRepo_Create(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")

	err := repo.Create(ctx, pr)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestSQLitePRRepo_Create_DuplicateID(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")

	must(t, repo.Create(ctx, pr))

	err := repo.Create(ctx, pr)
	if err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
}

// ─── Get ───────────────────────────────────────────────────────────────────

func TestSQLitePRRepo_Get(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "ticket-1")

	must(t, repo.Create(ctx, pr))

	got, err := repo.Get(ctx, "pr-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if got.ID != pr.ID {
		t.Errorf("got ID %q, want %q", got.ID, pr.ID)
	}
	if got.ProjectID != pr.ProjectID {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, pr.ProjectID)
	}
	if got.ExternalID != pr.ExternalID {
		t.Errorf("got ExternalID %q, want %q", got.ExternalID, pr.ExternalID)
	}
	if got.Source != pr.Source {
		t.Errorf("got Source %q, want %q", got.Source, pr.Source)
	}
	if got.Title != pr.Title {
		t.Errorf("got Title %q, want %q", got.Title, pr.Title)
	}
	if got.URL != pr.URL {
		t.Errorf("got URL %q, want %q", got.URL, pr.URL)
	}
	if got.Status != pr.Status {
		t.Errorf("got Status %q, want %q", got.Status, pr.Status)
	}
	if !got.CreatedAt.Equal(pr.CreatedAt) {
		t.Errorf("got CreatedAt %v, want %v", got.CreatedAt, pr.CreatedAt)
	}
	if !got.UpdatedAt.Equal(pr.UpdatedAt) {
		t.Errorf("got UpdatedAt %v, want %v", got.UpdatedAt, pr.UpdatedAt)
	}
}

func TestSQLitePRRepo_Get_NotFound(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── List ──────────────────────────────────────────────────────────────────

func TestSQLitePRRepo_List(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()

	prs := []model.PullRequest{
		testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1"),
		testPullRequest("pr-2", "proj-1", model.PRStatusMerged, model.PRSourceGitLab, "t2"),
		testPullRequest("pr-3", "proj-2", model.PRStatusClosed, model.PRSourceGitHub, "t3"),
	}
	for _, pr := range prs {
		must(t, repo.Create(ctx, pr))
	}

	result, err := repo.List(ctx, repository.PullRequestFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != len(prs) {
		t.Errorf("got %d pull requests, want %d", len(result), len(prs))
	}
}

func TestSQLitePRRepo_List_Empty(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()

	result, err := repo.List(ctx, repository.PullRequestFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("got %d pull requests, want 0", len(result))
	}
}

func TestSQLitePRRepo_List_FilterByProject(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()

	prs := []model.PullRequest{
		testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1"),
		testPullRequest("pr-2", "proj-1", model.PRStatusMerged, model.PRSourceGitLab, "t2"),
		testPullRequest("pr-3", "proj-2", model.PRStatusClosed, model.PRSourceGitHub, "t3"),
	}
	for _, pr := range prs {
		must(t, repo.Create(ctx, pr))
	}

	result, err := repo.List(ctx, repository.PullRequestFilter{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d pull requests, want 2", len(result))
	}
}

func TestSQLitePRRepo_List_FilterByStatus(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()

	prs := []model.PullRequest{
		testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1"),
		testPullRequest("pr-2", "proj-1", model.PRStatusMerged, model.PRSourceGitLab, "t2"),
		testPullRequest("pr-3", "proj-2", model.PRStatusClosed, model.PRSourceGitHub, "t3"),
	}
	for _, pr := range prs {
		must(t, repo.Create(ctx, pr))
	}

	result, err := repo.List(ctx, repository.PullRequestFilter{Status: model.PRStatusMerged})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d pull requests, want 1", len(result))
	}
	if result[0].ID != "pr-2" {
		t.Errorf("got PR ID %q, want %q", result[0].ID, "pr-2")
	}
}

func TestSQLitePRRepo_List_FilterByTicketID(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()

	// pr-1: ticket IDs ["t1", "t2"]
	// pr-2: ticket IDs ["t2"]
	// pr-3: ticket IDs ["t3"]
	// Filter: TicketID="t2" → 2 results (pr-1 and pr-2)
	prs := []model.PullRequest{
		testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1", "t2"),
		testPullRequest("pr-2", "proj-1", model.PRStatusMerged, model.PRSourceGitLab, "t2"),
		testPullRequest("pr-3", "proj-2", model.PRStatusClosed, model.PRSourceGitHub, "t3"),
	}
	for _, pr := range prs {
		must(t, repo.Create(ctx, pr))
	}

	result, err := repo.List(ctx, repository.PullRequestFilter{TicketID: "t2"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d pull requests, want 2", len(result))
	}
}

func TestSQLitePRRepo_List_CombinedFilters(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()

	prs := []model.PullRequest{
		testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1"),
		testPullRequest("pr-2", "proj-1", model.PRStatusMerged, model.PRSourceGitLab, "t2"),
		testPullRequest("pr-3", "proj-2", model.PRStatusOpen, model.PRSourceGitHub, "t3"),
		testPullRequest("pr-4", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t4"),
	}
	for _, pr := range prs {
		must(t, repo.Create(ctx, pr))
	}

	result, err := repo.List(ctx, repository.PullRequestFilter{
		ProjectID: "proj-1",
		Status:    model.PRStatusOpen,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d pull requests, want 2", len(result))
	}
}

// ─── Update ────────────────────────────────────────────────────────────────

func TestSQLitePRRepo_Update(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1")
	must(t, repo.Create(ctx, pr))

	pr.Status = model.PRStatusMerged
	pr.Title = "Updated PR Title"
	must(t, repo.Update(ctx, pr))

	got, err := repo.Get(ctx, "pr-1")
	if err != nil {
		t.Fatalf("Get after update returned error: %v", err)
	}
	if got.Status != model.PRStatusMerged {
		t.Errorf("got Status %q, want %q", got.Status, model.PRStatusMerged)
	}
	if got.Title != "Updated PR Title" {
		t.Errorf("got Title %q, want %q", got.Title, "Updated PR Title")
	}
}

func TestSQLitePRRepo_Update_NotFound(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()
	pr := testPullRequest("nonexistent", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1")

	err := repo.Update(ctx, pr)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── Delete ────────────────────────────────────────────────────────────────

func TestSQLitePRRepo_Delete(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()
	pr := testPullRequest("pr-1", "proj-1", model.PRStatusOpen, model.PRSourceGitHub, "t1")
	must(t, repo.Create(ctx, pr))

	must(t, repo.Delete(ctx, "pr-1"))

	_, err := repo.Get(ctx, "pr-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSQLitePRRepo_Delete_NotFound(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── JSON Round Trip ───────────────────────────────────────────────────────

func TestSQLitePRRepo_JSONRoundTrip(t *testing.T) {
	_, repo := setupPRTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	pr := model.PullRequest{
		ID:         "pr-full",
		ProjectID:  "proj-1",
		ExternalID: "ext-full",
		Source:     model.PRSourceGitHub,
		Title:      "Full PR",
		URL:        "https://github.com/example/repo/pull/full",
		Status:     model.PRStatusMerged,
		TicketIDs:  []string{"ticket-1", "ticket-2"},
		Reviews: []model.Review{
			{Author: "alice", Status: model.ReviewStatusApproved, Comment: "LGTM", CreatedAt: now},
			{Author: "bob", Status: model.ReviewStatusChangesRequested, Comment: "Needs fix", CreatedAt: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	must(t, repo.Create(ctx, pr))

	got, err := repo.Get(ctx, "pr-full")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	// Verify scalar fields.
	if got.ID != pr.ID {
		t.Errorf("got ID %q, want %q", got.ID, pr.ID)
	}
	if got.ProjectID != pr.ProjectID {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, pr.ProjectID)
	}
	if got.ExternalID != pr.ExternalID {
		t.Errorf("got ExternalID %q, want %q", got.ExternalID, pr.ExternalID)
	}
	if got.Source != pr.Source {
		t.Errorf("got Source %q, want %q", got.Source, pr.Source)
	}
	if got.Title != pr.Title {
		t.Errorf("got Title %q, want %q", got.Title, pr.Title)
	}
	if got.URL != pr.URL {
		t.Errorf("got URL %q, want %q", got.URL, pr.URL)
	}
	if got.Status != pr.Status {
		t.Errorf("got Status %q, want %q", got.Status, pr.Status)
	}
	if !got.CreatedAt.Equal(pr.CreatedAt) {
		t.Errorf("got CreatedAt %v, want %v", got.CreatedAt, pr.CreatedAt)
	}
	if !got.UpdatedAt.Equal(pr.UpdatedAt) {
		t.Errorf("got UpdatedAt %v, want %v", got.UpdatedAt, pr.UpdatedAt)
	}

	// Verify TicketIDs.
	if len(got.TicketIDs) != len(pr.TicketIDs) {
		t.Errorf("got %d ticket IDs, want %d", len(got.TicketIDs), len(pr.TicketIDs))
	} else {
		for i, want := range pr.TicketIDs {
			if got.TicketIDs[i] != want {
				t.Errorf("ticketID[%d]: got %q, want %q", i, got.TicketIDs[i], want)
			}
		}
	}

	// Verify Reviews.
	if len(got.Reviews) != len(pr.Reviews) {
		t.Errorf("got %d reviews, want %d", len(got.Reviews), len(pr.Reviews))
	} else {
		for i, want := range pr.Reviews {
			if got.Reviews[i].Author != want.Author {
				t.Errorf("review[%d] Author: got %q, want %q", i, got.Reviews[i].Author, want.Author)
			}
			if got.Reviews[i].Status != want.Status {
				t.Errorf("review[%d] Status: got %q, want %q", i, got.Reviews[i].Status, want.Status)
			}
			if got.Reviews[i].Comment != want.Comment {
				t.Errorf("review[%d] Comment: got %q, want %q", i, got.Reviews[i].Comment, want.Comment)
			}
			if !got.Reviews[i].CreatedAt.Equal(want.CreatedAt) {
				t.Errorf("review[%d] CreatedAt: got %v, want %v", i, got.Reviews[i].CreatedAt, want.CreatedAt)
			}
		}
	}
}
