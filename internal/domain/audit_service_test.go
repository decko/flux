package domain_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
	"github.com/decko/flux/pkg/authctx"
)

// ─── Setup ─────────────────────────────────────────────────────────────────

// setupAuditTestDB opens an in-memory SQLite database, configures it, migrates
// the audit_events table, and returns a SQLiteAuditRepository for testing.
func setupAuditTestDB(t *testing.T) *repository.SQLiteAuditRepository {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := repository.ConfigureSQLiteDB(db); err != nil {
		t.Fatalf("failed to configure SQLite: %v", err)
	}

	repo := repository.NewSQLiteAuditRepository(db)
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to run migration: %v", err)
	}
	return repo
}

// ─── AuditService Tests ────────────────────────────────────────────────────

func TestAuditService_Record(t *testing.T) {
	repo := setupAuditTestDB(t)
	svc := domain.NewAuditService(repo)
	ctx := authctx.WithUserID(context.Background(), "user-1")

	if err := svc.Record(ctx, "project.created", "project", "proj-1", `{}`); err != nil {
		t.Fatalf("Record returned error: %v", err)
	}

	// Verify the event was persisted with correct fields.
	events, err := repo.List(context.Background(), repository.AuditFilter{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0].ActorID != "user-1" {
		t.Errorf("ActorID = %q, want %q", events[0].ActorID, "user-1")
	}
	if events[0].Action != model.AuditAction("project.created") {
		t.Errorf("Action = %q, want %q", events[0].Action, model.AuditAction("project.created"))
	}
	if events[0].ResourceType != "project" {
		t.Errorf("ResourceType = %q, want %q", events[0].ResourceType, "project")
	}
	if events[0].ResourceID != "proj-1" {
		t.Errorf("ResourceID = %q, want %q", events[0].ResourceID, "proj-1")
	}
	if events[0].Metadata != "{}" {
		t.Errorf("Metadata = %q, want %q", events[0].Metadata, "{}")
	}
	if events[0].ID == "" {
		t.Error("expected non-empty event ID")
	}
	if events[0].CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestAuditService_Record_NoUserID(t *testing.T) {
	repo := setupAuditTestDB(t)
	svc := domain.NewAuditService(repo)
	ctx := context.Background() // no user_id set

	err := svc.Record(ctx, "project.created", "project", "proj-1", `{}`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, model.ErrInvalidAuditEvent) {
		t.Fatalf("expected ErrInvalidAuditEvent, got %v", err)
	}
}

func TestAuditService_Record_InvalidAction(t *testing.T) {
	repo := setupAuditTestDB(t)
	svc := domain.NewAuditService(repo)
	ctx := authctx.WithUserID(context.Background(), "user-1")

	err := svc.Record(ctx, "", "project", "proj-1", `{}`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, model.ErrInvalidAuditEvent) {
		t.Fatalf("expected ErrInvalidAuditEvent, got %v", err)
	}
}

func TestAuditService_Record_RepoError(t *testing.T) {
	// Open a DB without running migration — the table does not exist,
	// so Insert will fail and the service should propagate the error.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory SQLite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repo := repository.NewSQLiteAuditRepository(db)
	svc := domain.NewAuditService(repo)
	ctx := authctx.WithUserID(context.Background(), "user-1")

	err = svc.Record(ctx, "project.created", "project", "proj-1", `{}`)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
