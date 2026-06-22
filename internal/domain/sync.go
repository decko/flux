package domain

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/decko/flux/internal/adapter/scm"
	"github.com/decko/flux/internal/adapter/ticket"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// SyncStatus holds the result of the last sync operation.
type SyncStatus struct {
	LastSyncAt    time.Time
	LastSyncError string
	TicketsSynced int
	PRsSynced     int
}

// SyncService periodically syncs tickets and pull requests from
// external adapters into the local repository. It supports both
// scheduled (Run) and one-shot (SyncNow) synchronization.
type SyncService struct {
	// TicketRepo is the repository for persisting synced tickets.
	TicketRepo repository.TicketRepository
	// PRRepo is the repository for persisting synced pull requests.
	PRRepo repository.PullRequestRepository
	// TicketAdapter is the external ticket source adapter.
	TicketAdapter ticket.TicketAdapter
	// SCMAdapter is the external SCM source adapter.
	SCMAdapter scm.SCMAdapter
	interval   time.Duration
	logger     *slog.Logger

	mu      sync.Mutex
	status  SyncStatus
	ticker  *time.Ticker
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
}

// ticketKey is a composite uniqueness key for upsert matching.
type ticketKey struct {
	ProjectID  string
	Source     model.TicketSource
	ExternalID string
}

// prKey is a composite uniqueness key for pull request upsert matching.
type prKey struct {
	ProjectID  string
	Source     model.PRSource
	ExternalID string
}

// NewSyncService creates a new SyncService with the given dependencies.
// The interval controls how often Run performs periodic syncs.
func NewSyncService(
	ticketRepo repository.TicketRepository,
	prRepo repository.PullRequestRepository,
	ticketAdapter ticket.TicketAdapter,
	scmAdapter scm.SCMAdapter,
	interval time.Duration,
) *SyncService {
	return &SyncService{
		TicketRepo:    ticketRepo,
		PRRepo:        prRepo,
		TicketAdapter: ticketAdapter,
		SCMAdapter:    scmAdapter,
		interval:      interval,
		logger:        slog.Default(),
	}
}

// Status returns the result of the last sync operation. It is safe
// for concurrent use.
func (s *SyncService) Status() SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Run starts the periodic sync loop. It performs an immediate sync
// on start, then syncs every interval until the context is canceled
// or Stop is called. This method blocks until the loop exits.
// Calling Run multiple times is safe: subsequent calls are no-ops.
func (s *SyncService) Run(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	s.cancel = cancel
	s.ticker = time.NewTicker(s.interval)
	s.done = make(chan struct{})
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.ticker = nil
		s.done = nil
		s.mu.Unlock()
	}()

	// Immediate first sync.
	_ = s.syncOnce(ctx, "")

	for {
		select {
		case <-s.ticker.C:
			_ = s.syncOnce(ctx, "")
		case <-ctx.Done():
			close(s.done)
			return
		}
	}
}

// Stop cancels the periodic sync loop and waits for it to exit
// (with a 5-second timeout). Safe to call multiple times and
// safe to call when Run has not been called.
func (s *SyncService) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	ticker := s.ticker
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}

	if ticker != nil {
		ticker.Stop()
	}
}

// SyncNow performs a one-shot synchronization for the given project.
// It returns an error only if the context is canceled; adapter errors
// are recorded in the sync status but not propagated.
func (s *SyncService) SyncNow(ctx context.Context, projectID string) error {
	return s.syncOnce(ctx, projectID)
}

// syncOnce is the core sync logic. It fetches tickets and pull requests
// from the respective adapters and upserts them into the local repository
// using a single-pass map lookup for O(1) duplicate detection.
// Errors from one adapter do not block the other. Returns an error only
// if the context has been canceled.
func (s *SyncService) syncOnce(ctx context.Context, projectID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var lastErr error
	ticketCount := 0
	prCount := 0

	// Fetch and upsert tickets.
	if s.TicketAdapter != nil {
		tickets, err := s.TicketAdapter.ListTickets(ctx, projectID)
		if err != nil {
			lastErr = err
		} else {
			// Build lookup map of existing tickets for O(1) upsert matching.
			existingTickets, listErr := s.TicketRepo.List(ctx, repository.TicketFilter{ProjectID: projectID})
			byKey := make(map[ticketKey]model.Ticket, len(existingTickets))
			if listErr != nil {
				lastErr = listErr
				s.logger.Error("list existing tickets failed", "project_id", projectID, "err", listErr)
			} else {
				for _, t := range existingTickets {
					key := ticketKey{ProjectID: t.ProjectID, Source: t.Source, ExternalID: t.ExternalID}
					byKey[key] = t
				}
			}

			for _, t := range tickets {
				if err := ctx.Err(); err != nil {
					return err
				}
				key := ticketKey{ProjectID: t.ProjectID, Source: t.Source, ExternalID: t.ExternalID}
				if _, ok := byKey[key]; ok {
					if err := s.updateTicket(ctx, byKey[key], t); err != nil {
						s.logger.Error("upsert ticket failed", "external_id", t.ExternalID, "err", err)
					} else {
						ticketCount++
					}
				} else {
					if err := s.createTicket(ctx, t); err != nil {
						s.logger.Error("create ticket failed", "external_id", t.ExternalID, "err", err)
					} else {
						ticketCount++
					}
				}
			}
		}
	}

	// Fetch and upsert pull requests.
	if s.SCMAdapter != nil {
		prs, err := s.SCMAdapter.ListPullRequests(ctx, projectID)
		if err != nil {
			lastErr = err
		} else {
			// Build lookup map of existing PRs for O(1) upsert matching.
			existingPRs, listErr := s.PRRepo.List(ctx, repository.PullRequestFilter{ProjectID: projectID})
			byKey := make(map[prKey]model.PullRequest, len(existingPRs))
			if listErr != nil {
				lastErr = listErr
				s.logger.Error("list existing PRs failed", "project_id", projectID, "err", listErr)
			} else {
				for _, pr := range existingPRs {
					key := prKey{ProjectID: pr.ProjectID, Source: pr.Source, ExternalID: pr.ExternalID}
					byKey[key] = pr
				}
			}

			for _, pr := range prs {
				if err := ctx.Err(); err != nil {
					return err
				}
				key := prKey{ProjectID: pr.ProjectID, Source: pr.Source, ExternalID: pr.ExternalID}
				if _, ok := byKey[key]; ok {
					if err := s.updatePR(ctx, byKey[key], pr); err != nil {
						s.logger.Error("upsert PR failed", "external_id", pr.ExternalID, "err", err)
					} else {
						prCount++
					}
				} else {
					if err := s.createPR(ctx, pr); err != nil {
						s.logger.Error("create PR failed", "external_id", pr.ExternalID, "err", err)
					} else {
						prCount++
					}
				}
			}
		}
	}

	// Update status under lock.
	s.mu.Lock()
	s.status.LastSyncAt = time.Now()
	if lastErr != nil {
		s.status.LastSyncError = lastErr.Error()
	} else {
		s.status.LastSyncError = ""
	}
	s.status.TicketsSynced = ticketCount
	s.status.PRsSynced = prCount
	s.mu.Unlock()

	s.logger.Info("sync complete",
		"tickets", ticketCount,
		"prs", prCount,
		"err", lastErr,
	)

	return nil
}

// updateTicket applies fields from an incoming ticket to an existing one
// and persists the update. Preserves the existing ID and CreatedAt.
func (s *SyncService) updateTicket(ctx context.Context, existing model.Ticket, incoming model.Ticket) error {
	existing.Title = incoming.Title
	existing.Description = incoming.Description
	existing.Status = incoming.Status
	existing.Labels = incoming.Labels
	existing.Source = incoming.Source
	existing.Relationships = incoming.Relationships
	existing.PRs = incoming.PRs
	existing.UpdatedAt = time.Now()
	if err := s.TicketRepo.Update(ctx, existing); err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}
	return nil
}

// createTicket assigns a new ID and timestamps, then persists the ticket.
func (s *SyncService) createTicket(ctx context.Context, t model.Ticket) error {
	t.ID = uuid.New().String()
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	if err := s.TicketRepo.Create(ctx, t); err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	return nil
}

// updatePR applies fields from an incoming PR to an existing one
// and persists the update. Preserves the existing ID and CreatedAt.
func (s *SyncService) updatePR(ctx context.Context, existing model.PullRequest, incoming model.PullRequest) error {
	existing.Title = incoming.Title
	existing.URL = incoming.URL
	existing.Status = incoming.Status
	existing.Source = incoming.Source
	existing.TicketIDs = incoming.TicketIDs
	existing.Reviews = incoming.Reviews
	existing.UpdatedAt = time.Now()
	if err := s.PRRepo.Update(ctx, existing); err != nil {
		return fmt.Errorf("update PR: %w", err)
	}
	return nil
}

// createPR assigns a new ID and timestamps, then persists the PR.
func (s *SyncService) createPR(ctx context.Context, pr model.PullRequest) error {
	pr.ID = uuid.New().String()
	now := time.Now()
	pr.CreatedAt = now
	pr.UpdatedAt = now
	if err := s.PRRepo.Create(ctx, pr); err != nil {
		return fmt.Errorf("create PR: %w", err)
	}
	return nil
}
