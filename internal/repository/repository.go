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

	"github.com/decko/flux/internal/model"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

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
