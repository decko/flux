package ticket

import (
	"context"
	"errors"
	"testing"

	"github.com/decko/flux/internal/adapter"
	"github.com/decko/flux/internal/model"
)

func TestStubSatisfiesTicketAdapter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T, a TicketAdapter)
	}{
		{
			name: "Name",
			run: func(t *testing.T, a TicketAdapter) {
				got := a.Name()
				if got != "test-stub" {
					t.Errorf("Name() = %q, want %q", got, "test-stub")
				}
			},
		},
		{
			name: "ListTickets returns ErrNotImplemented",
			run: func(t *testing.T, a TicketAdapter) {
				tickets, err := a.ListTickets(context.Background(), "proj-1")
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Fatalf("ListTickets err = %v, want %v", err, adapter.ErrNotImplemented)
				}
				if len(tickets) != 0 {
					t.Errorf("ListTickets returned %d tickets, want 0", len(tickets))
				}
			},
		},
		{
			name: "GetTicket returns ErrNotImplemented",
			run: func(t *testing.T, a TicketAdapter) {
				got, err := a.GetTicket(context.Background(), "proj-1", "ext-1")
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Fatalf("GetTicket err = %v, want %v", err, adapter.ErrNotImplemented)
				}
				if got != nil {
					t.Errorf("GetTicket returned non-nil ticket: %v", got)
				}
			},
		},
		{
			name: "CreateTicket returns ErrNotImplemented",
			run: func(t *testing.T, a TicketAdapter) {
				got, err := a.CreateTicket(context.Background(), &model.Ticket{})
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Fatalf("CreateTicket err = %v, want %v", err, adapter.ErrNotImplemented)
				}
				if got != nil {
					t.Errorf("CreateTicket returned non-nil ticket: %v", got)
				}
			},
		},
		{
			name: "UpdateTicket returns ErrNotImplemented",
			run: func(t *testing.T, a TicketAdapter) {
				err := a.UpdateTicket(context.Background(), &model.Ticket{})
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Fatalf("UpdateTicket err = %v, want %v", err, adapter.ErrNotImplemented)
				}
			},
		},
		{
			name: "SyncRelationships returns ErrNotImplemented",
			run: func(t *testing.T, a TicketAdapter) {
				err := a.SyncRelationships(context.Background(), "proj-1")
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Fatalf("SyncRelationships err = %v, want %v", err, adapter.ErrNotImplemented)
				}
			},
		},
		{
			name: "Health returns ErrNotImplemented",
			run: func(t *testing.T, a TicketAdapter) {
				err := a.Health(context.Background())
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Fatalf("Health err = %v, want %v", err, adapter.ErrNotImplemented)
				}
			},
		},
	}

	stub := &StubTicketAdapter{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t, stub)
		})
	}
}
