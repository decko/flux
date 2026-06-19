package domain

import (
	"context"
	"fmt"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// TicketService provides business logic for ticket CRUD operations.
// It validates inputs before delegating to the repository and wraps
// repository errors with additional context.
type TicketService struct {
	repo repository.TicketRepository
}

// NewTicketService creates a new TicketService backed by the given repository.
func NewTicketService(repo repository.TicketRepository) *TicketService {
	return &TicketService{repo: repo}
}

// Create validates the ticket and persists it.
// Returns validation errors directly; wraps repository errors.
func (s *TicketService) Create(ctx context.Context, t model.Ticket) error {
	if err := t.Validate(); err != nil {
		return err
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	return nil
}

// Get retrieves a ticket by ID.
// Returns ErrNotFound if the ticket does not exist.
func (s *TicketService) Get(ctx context.Context, id string) (model.Ticket, error) {
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return model.Ticket{}, fmt.Errorf("get ticket: %w", err)
	}
	return t, nil
}

// List returns all tickets matching the given filter criteria.
func (s *TicketService) List(ctx context.Context, filter repository.TicketFilter) ([]model.Ticket, error) {
	tickets, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list tickets: %w", err)
	}
	return tickets, nil
}

// Update validates the ticket and modifies it in the store.
// Returns validation errors directly; wraps repository errors.
// Returns ErrNotFound if the ticket does not exist.
func (s *TicketService) Update(ctx context.Context, t model.Ticket) error {
	if err := t.Validate(); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, t); err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}
	return nil
}

// Delete removes a ticket by ID.
// Returns ErrNotFound if the ticket does not exist.
func (s *TicketService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete ticket: %w", err)
	}
	return nil
}
