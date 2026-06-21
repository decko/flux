// Package ticket defines the TicketAdapter interface and related
// types for reading and writing tickets from external sources.
package ticket

import (
	"context"

	"github.com/decko/flux/internal/model"
)

// TicketAdapter defines the interface for reading and writing tickets
// from external sources (GitHub, Jira, Linear, etc.). All methods
// accept a context for cancellation and timeout propagation.
type TicketAdapter interface {
	// Name returns a human-readable name for this adapter (e.g. "github", "jira").
	Name() string

	// ListTickets returns all tickets belonging to the given project.
	ListTickets(ctx context.Context, projectID string) ([]model.Ticket, error)

	// GetTicket retrieves a single ticket by its external ID.
	GetTicket(ctx context.Context, projectID, externalID string) (*model.Ticket, error)

	// CreateTicket creates a new ticket in the external source.
	CreateTicket(ctx context.Context, ticket *model.Ticket) error

	// UpdateTicket modifies an existing ticket in the external source.
	UpdateTicket(ctx context.Context, ticket *model.Ticket) error

	// SyncRelationships synchronizes ticket dependency relationships
	// (blocks, blocked_by, relates_to, etc.) for all tickets in a project.
	SyncRelationships(ctx context.Context, projectID string) error

	// Health checks whether the external source is reachable and responsive.
	Health(ctx context.Context) error
}
