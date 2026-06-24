package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ─── Setup ─────────────────────────────────────────────────────────────────

// setupTicketTestDB opens an in-memory SQLite database, configures it for
// SQLite use (pool + WAL), creates the tickets table via migration, and
// returns a SQLiteTicketRepository for testing.
func setupTicketTestDB(t *testing.T) (*sql.DB, *repository.SQLiteTicketRepository) {
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

	repo := repository.NewSQLiteTicketRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to run migration: %v", err)
	}
	return db, repo
}

// ─── Create ────────────────────────────────────────────────────────────────

func TestSQLiteTicketRepo_Create(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)

	err := repo.Create(ctx, tk)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}

func TestSQLiteTicketRepo_Create_DuplicateID(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)

	must(t, repo.Create(ctx, tk))

	err := repo.Create(ctx, tk)
	if err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
}

// ─── Get ───────────────────────────────────────────────────────────────────

func TestSQLiteTicketRepo_Get(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)

	must(t, repo.Create(ctx, tk))

	got, err := repo.Get(ctx, "ticket-1")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if got.ID != tk.ID {
		t.Errorf("got ID %q, want %q", got.ID, tk.ID)
	}
	if got.ProjectID != tk.ProjectID {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, tk.ProjectID)
	}
	if got.ExternalID != tk.ExternalID {
		t.Errorf("got ExternalID %q, want %q", got.ExternalID, tk.ExternalID)
	}
	if got.Source != tk.Source {
		t.Errorf("got Source %q, want %q", got.Source, tk.Source)
	}
	if got.Title != tk.Title {
		t.Errorf("got Title %q, want %q", got.Title, tk.Title)
	}
	if got.Description != tk.Description {
		t.Errorf("got Description %q, want %q", got.Description, tk.Description)
	}
	if got.Status != tk.Status {
		t.Errorf("got Status %q, want %q", got.Status, tk.Status)
	}
	if !got.CreatedAt.Equal(tk.CreatedAt) {
		t.Errorf("got CreatedAt %v, want %v", got.CreatedAt, tk.CreatedAt)
	}
	if !got.UpdatedAt.Equal(tk.UpdatedAt) {
		t.Errorf("got UpdatedAt %v, want %v", got.UpdatedAt, tk.UpdatedAt)
	}
}

func TestSQLiteTicketRepo_Get_NotFound(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── List ──────────────────────────────────────────────────────────────────

func TestSQLiteTicketRepo_List(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()

	tickets := []model.Ticket{
		testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub),
		testTicket("t2", "proj-1", model.TicketStatusClosed, model.TicketSourceJira),
		testTicket("t3", "proj-2", model.TicketStatusOpen, model.TicketSourceLinear),
	}
	for _, tk := range tickets {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != len(tickets) {
		t.Errorf("got %d tickets, want %d", len(result), len(tickets))
	}
}

func TestSQLiteTicketRepo_List_Empty(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()

	result, err := repo.List(ctx, repository.TicketFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("got %d tickets, want 0", len(result))
	}
}

func TestSQLiteTicketRepo_List_FilterByProject(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()

	tickets := []model.Ticket{
		testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub),
		testTicket("t2", "proj-1", model.TicketStatusClosed, model.TicketSourceJira),
		testTicket("t3", "proj-2", model.TicketStatusOpen, model.TicketSourceLinear),
	}
	for _, tk := range tickets {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d tickets, want 2", len(result))
	}
}

func TestSQLiteTicketRepo_List_FilterByStatus(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()

	tickets := []model.Ticket{
		testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub),
		testTicket("t2", "proj-1", model.TicketStatusClosed, model.TicketSourceJira),
		testTicket("t3", "proj-2", model.TicketStatusOpen, model.TicketSourceLinear),
	}
	for _, tk := range tickets {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{Status: model.TicketStatusOpen})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d tickets, want 2", len(result))
	}
}

func TestSQLiteTicketRepo_List_FilterBySource(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()

	tickets := []model.Ticket{
		testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub),
		testTicket("t2", "proj-1", model.TicketStatusClosed, model.TicketSourceJira),
		testTicket("t3", "proj-2", model.TicketStatusOpen, model.TicketSourceLinear),
	}
	for _, tk := range tickets {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{Source: model.TicketSourceLinear})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("got %d tickets, want 1", len(result))
	}
	if result[0].ID != "t3" {
		t.Errorf("got ticket ID %q, want %q", result[0].ID, "t3")
	}
}

func TestSQLiteTicketRepo_List_FilterByLabels(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()

	// Tickets with labels:
	//   t1: ["bug", "critical"]
	//   t2: ["feature"]
	//   t3: ["bug"]
	// Filtering by Labels: ["bug"] → OR-match → t1 and t3 (2 results).
	t1 := testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub, "bug", "critical")
	t2 := testTicket("t2", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub, "feature")
	t3 := testTicket("t3", "proj-2", model.TicketStatusClosed, model.TicketSourceJira, "bug")
	for _, tk := range []model.Ticket{t1, t2, t3} {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{Labels: []string{"bug"}})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d tickets, want 2", len(result))
	}
}

func TestSQLiteTicketRepo_List_CombinedFilters(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()

	tickets := []model.Ticket{
		testTicket("t1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub),
		testTicket("t2", "proj-1", model.TicketStatusClosed, model.TicketSourceJira),
		testTicket("t3", "proj-2", model.TicketStatusOpen, model.TicketSourceLinear),
		testTicket("t4", "proj-1", model.TicketStatusOpen, model.TicketSourceLinear),
	}
	for _, tk := range tickets {
		must(t, repo.Create(ctx, tk))
	}

	result, err := repo.List(ctx, repository.TicketFilter{
		ProjectID: "proj-1",
		Status:    model.TicketStatusOpen,
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("got %d tickets, want 2", len(result))
	}
}

// ─── Update ────────────────────────────────────────────────────────────────

func TestSQLiteTicketRepo_Update(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)
	must(t, repo.Create(ctx, tk))

	tk.Status = model.TicketStatusInProgress
	tk.Title = "Updated Title"
	must(t, repo.Update(ctx, tk))

	got, err := repo.Get(ctx, "ticket-1")
	if err != nil {
		t.Fatalf("Get after update returned error: %v", err)
	}
	if got.Status != model.TicketStatusInProgress {
		t.Errorf("got Status %q, want %q", got.Status, model.TicketStatusInProgress)
	}
	if got.Title != "Updated Title" {
		t.Errorf("got Title %q, want %q", got.Title, "Updated Title")
	}
}

func TestSQLiteTicketRepo_Update_NotFound(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()
	tk := testTicket("nonexistent", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)

	err := repo.Update(ctx, tk)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── Delete ────────────────────────────────────────────────────────────────

func TestSQLiteTicketRepo_Delete(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()
	tk := testTicket("ticket-1", "proj-1", model.TicketStatusOpen, model.TicketSourceGitHub)
	must(t, repo.Create(ctx, tk))

	must(t, repo.Delete(ctx, "ticket-1"))

	_, err := repo.Get(ctx, "ticket-1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSQLiteTicketRepo_Delete_NotFound(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// ─── JSON Round Trip ───────────────────────────────────────────────────────

func TestSQLiteTicketRepo_JSONRoundTrip(t *testing.T) {
	_, repo := setupTicketTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	tk := model.Ticket{
		ID:          "ticket-full",
		ProjectID:   "proj-1",
		ExternalID:  "ext-full",
		Source:      model.TicketSourceGitHub,
		Title:       "Full Ticket",
		Description: "Full desc",
		Status:      model.TicketStatusOpen,
		Labels:      []string{"bug", "critical", "frontend"},
		Relationships: []model.Relationship{
			{Type: model.RelationBlocks, TargetID: "ticket-2"},
			{Type: model.RelationRelatesTo, TargetID: "ticket-3"},
		},
		PRs:       []string{"pr-1", "pr-2"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	must(t, repo.Create(ctx, tk))

	got, err := repo.Get(ctx, "ticket-full")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	// Verify scalar fields.
	if got.ID != tk.ID {
		t.Errorf("got ID %q, want %q", got.ID, tk.ID)
	}
	if got.ProjectID != tk.ProjectID {
		t.Errorf("got ProjectID %q, want %q", got.ProjectID, tk.ProjectID)
	}
	if got.ExternalID != tk.ExternalID {
		t.Errorf("got ExternalID %q, want %q", got.ExternalID, tk.ExternalID)
	}
	if got.Source != tk.Source {
		t.Errorf("got Source %q, want %q", got.Source, tk.Source)
	}
	if got.Title != tk.Title {
		t.Errorf("got Title %q, want %q", got.Title, tk.Title)
	}
	if got.Description != tk.Description {
		t.Errorf("got Description %q, want %q", got.Description, tk.Description)
	}
	if got.Status != tk.Status {
		t.Errorf("got Status %q, want %q", got.Status, tk.Status)
	}
	if !got.CreatedAt.Equal(tk.CreatedAt) {
		t.Errorf("got CreatedAt %v, want %v", got.CreatedAt, tk.CreatedAt)
	}
	if !got.UpdatedAt.Equal(tk.UpdatedAt) {
		t.Errorf("got UpdatedAt %v, want %v", got.UpdatedAt, tk.UpdatedAt)
	}

	// Verify Labels.
	if len(got.Labels) != len(tk.Labels) {
		t.Errorf("got %d labels, want %d", len(got.Labels), len(tk.Labels))
	} else {
		for i, want := range tk.Labels {
			if got.Labels[i] != want {
				t.Errorf("label[%d]: got %q, want %q", i, got.Labels[i], want)
			}
		}
	}

	// Verify Relationships.
	if len(got.Relationships) != len(tk.Relationships) {
		t.Errorf("got %d relationships, want %d", len(got.Relationships), len(tk.Relationships))
	} else {
		for i, want := range tk.Relationships {
			if got.Relationships[i].Type != want.Type {
				t.Errorf("relationship[%d] Type: got %q, want %q", i, got.Relationships[i].Type, want.Type)
			}
			if got.Relationships[i].TargetID != want.TargetID {
				t.Errorf("relationship[%d] TargetID: got %q, want %q", i, got.Relationships[i].TargetID, want.TargetID)
			}
		}
	}

	// Verify PRs.
	if len(got.PRs) != len(tk.PRs) {
		t.Errorf("got %d PRs, want %d", len(got.PRs), len(tk.PRs))
	} else {
		for i, want := range tk.PRs {
			if got.PRs[i] != want {
				t.Errorf("pr[%d]: got %q, want %q", i, got.PRs[i], want)
			}
		}
	}
}
