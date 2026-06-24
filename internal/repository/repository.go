// Package repository defines repository interfaces for persisting domain entities.
//
// The repository pattern abstracts data access behind interfaces, allowing
// implementations to switch between storage backends (e.g., SQLite, PostgreSQL)
// without affecting business logic. Each domain entity has its own repository
// interface with CRUD operations appropriate to its lifecycle.
//
// Filter types provide structured query parameters for list operations,
// enabling precise data retrieval without leaking implementation details.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/decko/flux/internal/model"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// ErrDuplicateEmail is returned when attempting to create a user with an
// email that already exists in the store.
var ErrDuplicateEmail = errors.New("email already exists")

// ProjectFilter defines criteria for listing projects.
// Currently empty; extensible for future filtering needs.
type ProjectFilter struct{}

// TicketFilter defines criteria for listing tickets.
// Zero values are ignored; only non-zero fields are used for filtering.
type TicketFilter struct {
	ProjectID string
	Status    model.TicketStatus
	Source    model.TicketSource
	Labels    []string
}

// PullRequestFilter defines criteria for listing pull requests.
// Zero values are ignored; only non-zero fields are used for filtering.
type PullRequestFilter struct {
	ProjectID string
	Status    model.PRStatus
	TicketID  string
}

// PipelineRunFilter defines criteria for listing pipeline runs.
// Zero values are ignored; only non-zero fields are used for filtering.
type PipelineRunFilter struct {
	ProjectID string
	TicketID  string
	Status    model.RunStatus
}

// ProjectRepository defines the contract for project persistence.
type ProjectRepository interface {
	// Create persists a new project. Returns an error if a project with
	// the same ID already exists.
	Create(ctx context.Context, project model.Project) error

	// Get retrieves a project by ID. Returns ErrNotFound if no project
	// with the given ID exists.
	Get(ctx context.Context, id string) (model.Project, error)

	// List returns all projects matching the given filter criteria.
	// An empty filter returns all projects.
	List(ctx context.Context, filter ProjectFilter) ([]model.Project, error)

	// Update modifies an existing project. Returns ErrNotFound if no
	// project with the project's ID exists.
	Update(ctx context.Context, project model.Project) error

	// Delete removes a project by ID. Returns ErrNotFound if no project
	// with the given ID exists.
	Delete(ctx context.Context, id string) error
}

// TicketRepository defines the contract for ticket persistence.
type TicketRepository interface {
	// Create persists a new ticket. Returns an error if a ticket with
	// the same ID already exists.
	Create(ctx context.Context, ticket model.Ticket) error

	// Get retrieves a ticket by ID. Returns ErrNotFound if no ticket
	// with the given ID exists.
	Get(ctx context.Context, id string) (model.Ticket, error)

	// List returns all tickets matching the given filter criteria.
	// Zero values in the filter are ignored.
	List(ctx context.Context, filter TicketFilter) ([]model.Ticket, error)

	// Update modifies an existing ticket. Returns ErrNotFound if no
	// ticket with the ticket's ID exists.
	Update(ctx context.Context, ticket model.Ticket) error

	// Delete removes a ticket by ID. Returns ErrNotFound if no ticket
	// with the given ID exists.
	Delete(ctx context.Context, id string) error
}

// PullRequestRepository defines the contract for pull request persistence.
type PullRequestRepository interface {
	// Create persists a new pull request. Returns an error if a pull
	// request with the same ID already exists.
	Create(ctx context.Context, pr model.PullRequest) error

	// Get retrieves a pull request by ID. Returns ErrNotFound if no pull
	// request with the given ID exists.
	Get(ctx context.Context, id string) (model.PullRequest, error)

	// List returns all pull requests matching the given filter criteria.
	// Zero values in the filter are ignored.
	List(ctx context.Context, filter PullRequestFilter) ([]model.PullRequest, error)

	// Update modifies an existing pull request. Returns ErrNotFound if no
	// pull request with the pull request's ID exists.
	Update(ctx context.Context, pr model.PullRequest) error

	// Delete removes a pull request by ID. Returns ErrNotFound if no pull
	// request with the given ID exists.
	Delete(ctx context.Context, id string) error
}

// PipelineRunRepository defines the contract for pipeline run persistence.
// Pipeline runs are immutable records; there is no Delete method.
type PipelineRunRepository interface {
	// Create persists a new pipeline run. Returns an error if a run
	// with the same ID already exists.
	Create(ctx context.Context, run model.PipelineRun) error

	// Get retrieves a pipeline run by ID. Returns ErrNotFound if no run
	// with the given ID exists.
	Get(ctx context.Context, id string) (model.PipelineRun, error)

	// List returns all pipeline runs matching the given filter criteria.
	// Zero values in the filter are ignored.
	List(ctx context.Context, filter PipelineRunFilter) ([]model.PipelineRun, error)

	// Update modifies an existing pipeline run. Returns ErrNotFound if no
	// run with the run's ID exists.
	Update(ctx context.Context, run model.PipelineRun) error
}

// AuditFilter defines criteria for listing audit events.
// Zero values are ignored; only non-zero fields are used for filtering.
type AuditFilter struct {
	ActorID      string
	ResourceType string
	ResourceID   string
	Action       string
	Since        time.Time
	Until        time.Time
	Limit        int
	Offset       int
}

// AuditRepository defines the contract for audit event persistence.
// Audit records are append-only — there are no Update or Delete operations.
type AuditRepository interface {
	// Insert persists a new audit event. If the event's ID is empty, a UUID
	// is generated automatically.
	Insert(ctx context.Context, event model.AuditEvent) error

	// List returns audit events matching the given filter criteria.
	// Events are ordered by created_at descending (most recent first).
	// Zero values in the filter are ignored.
	List(ctx context.Context, filter AuditFilter) ([]model.AuditEvent, error)

	// Latest returns the most recent audit event (by created_at), or nil if
	// no events exist. Used by the hash chain to link consecutive events.
	Latest(ctx context.Context) (*model.AuditEvent, error)

	// PurgeOlderThan deletes audit events older than the given time.
	// Returns the count of deleted rows.
	PurgeOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// UserRepository defines the contract for user persistence.
type UserRepository interface {
	// Create persists a new user. Returns ErrDuplicateEmail if a user with
	// the same email already exists.
	Create(ctx context.Context, user model.User) error

	// GetByEmail retrieves a user by email. Returns ErrNotFound if no user
	// with the given email exists.
	GetByEmail(ctx context.Context, email string) (model.User, error)

	// GetByID retrieves a user by ID. Returns ErrNotFound if no user
	// with the given ID exists.
	GetByID(ctx context.Context, id string) (model.User, error)
}
