package domain

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/decko/flux/internal/adapter/scm"
	"github.com/decko/flux/internal/adapter/ticket"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
	"github.com/decko/flux/pkg/authctx"
)

// ProjectSyncStatus holds the sync result for a single project.
type ProjectSyncStatus struct {
	ProjectID       string
	LastSyncAt      *time.Time
	LastSyncError   string
	TicketsSynced   int
	PRsSynced       int
	WebhooksHealthy bool
}

// SyncStatus holds the result of the last sync operation.
// LastSyncAt is nil when no sync has been performed yet.
// Projects contains per-project status for the last sync pass.
type SyncStatus struct {
	LastSyncAt      *time.Time
	LastSyncError   string
	TicketsSynced   int
	PRsSynced       int
	WebhooksHealthy bool
	Projects        map[string]ProjectSyncStatus
}

// WebhookVerifier checks whether the webhook for a project is still
// registered and reachable at the external source. It returns true
// if the webhook is healthy, false otherwise. An error indicates the
// check itself failed (e.g., network error), not the webhook health.
type WebhookVerifier func(ctx context.Context, projectID string) (healthy bool, err error)

// AdapterFactory creates adapters for a specific project.
// Returns nil adapters if the project has no credentials configured.
type AdapterFactory func(projectID string) (ticket.TicketAdapter, scm.SCMAdapter, error)

// SyncService periodically syncs tickets and pull requests from
// external adapters into the local repository. It supports both
// scheduled (Run) and one-shot (SyncNow) synchronization.
type SyncService struct {
	// TicketRepo is the repository for persisting synced tickets.
	TicketRepo repository.TicketRepository
	// PRRepo is the repository for persisting synced pull requests.
	PRRepo repository.PullRequestRepository
	// ProjectRepo is the repository for persisting projects.
	ProjectRepo repository.ProjectRepository
	// Factory creates per-project adapters for sync.
	Factory         AdapterFactory
	webhookVerifier WebhookVerifier
	triggerSvc      *TriggerService
	auditSvc        *AuditService
	interval        time.Duration
	logger          *slog.Logger

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
	projectRepo repository.ProjectRepository,
	factory AdapterFactory,
	interval time.Duration,
) *SyncService {
	return &SyncService{
		TicketRepo:  ticketRepo,
		PRRepo:      prRepo,
		ProjectRepo: projectRepo,
		Factory:     factory,
		interval:    interval,
		logger:      slog.Default(),
		status: SyncStatus{
			Projects: make(map[string]ProjectSyncStatus),
		},
	}
}

// WithTriggerService sets the TriggerService for automatic pipeline
// triggering after each sync pass. When set, tickets and PRs are
// evaluated against trigger rules after they are upserted.
func (s *SyncService) WithTriggerService(svc *TriggerService) {
	s.triggerSvc = svc
}

// WithSyncAuditService sets the AuditService for recording audit events
// when tickets and pull requests are created or updated during sync.
func (s *SyncService) WithSyncAuditService(audit *AuditService) {
	s.auditSvc = audit
}

// WithWebhookVerifier sets the webhook verification function that checks
// whether registered webhooks are still present at the external source.
// The verifier is called once per project during sync. A failed check
// (returning false) marks the project's webhooks as unhealthy but does
// not prevent the sync from proceeding. When no verifier is configured,
// webhooks are assumed healthy.
func (s *SyncService) WithWebhookVerifier(v WebhookVerifier) {
	s.webhookVerifier = v
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

	// Immediate first sync of all projects.
	_ = s.SyncNow(ctx)

	for {
		select {
		case <-s.ticker.C:
			_ = s.SyncNow(ctx)
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

// SyncProject performs a one-shot synchronization for a single project.
// It returns an error only if the project is not found or the context
// is canceled; adapter errors are recorded in per-project sync status
// but not propagated.
func (s *SyncService) SyncProject(ctx context.Context, projectID string) error {
	return s.syncOnce(ctx, projectID)
}

// SyncNow performs a one-shot synchronization for all registered projects.
// It returns an error only if the project list cannot be retrieved or the
// context is canceled; individual project failures are isolated and do not
// block other projects.
func (s *SyncService) SyncNow(ctx context.Context) error {
	projects, err := s.ProjectRepo.List(ctx, repository.ProjectFilter{})
	if err != nil {
		return err
	}
	for _, p := range projects {
		if err := ctx.Err(); err != nil {
			return err
		}
		_ = s.syncOnce(ctx, p.ID)
	}
	// Recompute aggregate status from per-project statuses.
	s.mu.Lock()
	var totalTickets, totalPRs int
	var firstErr string
	var latestSync *time.Time
	allWebhooksHealthy := true
	for _, ps := range s.status.Projects {
		totalTickets += ps.TicketsSynced
		totalPRs += ps.PRsSynced
		if ps.LastSyncError != "" && firstErr == "" {
			firstErr = ps.LastSyncError
		}
		if ps.LastSyncAt != nil && (latestSync == nil || ps.LastSyncAt.After(*latestSync)) {
			t := *ps.LastSyncAt
			latestSync = &t
		}
		if !ps.WebhooksHealthy {
			allWebhooksHealthy = false
		}
	}
	s.status.TicketsSynced = totalTickets
	s.status.PRsSynced = totalPRs
	s.status.LastSyncError = firstErr
	s.status.WebhooksHealthy = allWebhooksHealthy
	if latestSync != nil {
		s.status.LastSyncAt = latestSync
	}
	s.mu.Unlock()
	return nil
}

// syncOnce is the core sync logic for a single project. It verifies the
// project exists, gets per-project adapters from the factory, then fetches
// tickets and pull requests from the respective adapters and upserts them
// into the local repository using a single-pass map lookup for O(1) duplicate
// detection. Errors from one adapter do not block the other. Returns an error
// only if the project is not found or the context has been canceled.
func (s *SyncService) syncOnce(ctx context.Context, projectID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Verify the project exists.
	if _, err := s.ProjectRepo.Get(ctx, projectID); err != nil {
		return fmt.Errorf("sync project %s: %w", projectID, err)
	}

	// Get per-project adapters from the factory.
	ticketAdapter, scmAdapter, err := s.Factory(projectID)
	if err != nil {
		s.logger.Warn("skipping project: no adapters available",
			"project_id", projectID,
			"err", err,
		)
		// Mark per-project status with the error and return.
		s.mu.Lock()
		now := time.Now()
		ps := ProjectSyncStatus{
			ProjectID:     projectID,
			LastSyncAt:    &now,
			LastSyncError: err.Error(),
		}
		s.status.Projects[projectID] = ps
		s.status.LastSyncAt = &now
		s.status.LastSyncError = err.Error()
		s.status.TicketsSynced = 0
		s.status.PRsSynced = 0
		s.mu.Unlock()
		return nil
	}

	var lastErr error
	ticketCount := 0
	prCount := 0

	// Fetch and upsert tickets.
	if ticketAdapter != nil {
		tickets, listErr := ticketAdapter.ListTickets(ctx, projectID)
		if listErr != nil {
			lastErr = listErr
		} else {
			// Ensure ProjectID is set on returned tickets (some adapters omit it).
			for i := range tickets {
				tickets[i].ProjectID = projectID
			}
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
				if existing, ok := byKey[key]; ok {
					if err := s.updateTicket(ctx, existing, t); err != nil {
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

		// Trigger pipelines for newly synced tickets.
		if s.triggerSvc != nil {
			for _, t := range tickets {
				if err := s.triggerSvc.CheckAndTrigger(ctx, t, model.DefaultEvent); err != nil {
					s.logger.Warn("trigger check for ticket failed", "ticket_id", t.ID, "err", err)
				}
			}
		}
	}

	// Fetch and upsert pull requests.
	if scmAdapter != nil {
		prs, listErr := scmAdapter.ListPullRequests(ctx, projectID)
		if listErr != nil {
			lastErr = listErr
		} else {
			// Ensure ProjectID is set on returned PRs (some adapters omit it).
			for i := range prs {
				prs[i].ProjectID = projectID
			}
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
				if existing, ok := byKey[key]; ok {
					if err := s.updatePR(ctx, existing, pr); err != nil {
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

	// Verify webhook health if a verifier is configured.
	// This is non-blocking: a failed check does not prevent sync from
	// proceeding. Errors from the verifier itself are logged but do
	// not affect the webhook health status.
	webhooksHealthy := true // default: assume healthy when unchecked
	if s.webhookVerifier != nil {
		healthy, verifyErr := s.webhookVerifier(ctx, projectID)
		if verifyErr != nil {
			s.logger.Warn("webhook verification call failed",
				"project_id", projectID, "err", verifyErr)
		} else {
			webhooksHealthy = healthy
		}
	}

	// Update per-project and aggregate status under lock.
	s.mu.Lock()
	now := time.Now()
	ps := ProjectSyncStatus{
		ProjectID:       projectID,
		LastSyncAt:      &now,
		TicketsSynced:   ticketCount,
		PRsSynced:       prCount,
		WebhooksHealthy: webhooksHealthy,
	}
	if lastErr != nil {
		ps.LastSyncError = lastErr.Error()
	}
	s.status.Projects[projectID] = ps
	s.status.LastSyncAt = &now
	if lastErr != nil {
		s.status.LastSyncError = lastErr.Error()
	} else {
		s.status.LastSyncError = ""
	}
	s.status.TicketsSynced = ticketCount
	s.status.PRsSynced = prCount
	s.mu.Unlock()

	s.logger.Info("sync complete",
		"project_id", projectID,
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
	if s.auditSvc != nil {
		auditCtx := authctx.WithUserID(ctx, "system:sync")
		if err := s.auditSvc.Record(auditCtx, model.AuditActionTicketUpdatedSync, "ticket", existing.ID, "origin=sync"); err != nil {
			s.logger.Error("sync: failed to record ticket update audit event", "ticket_id", existing.ID, "error", err)
		}
	}
	return nil
}

// createTicket assigns a new ID and timestamps, then persists the ticket.
func (s *SyncService) createTicket(ctx context.Context, t model.Ticket) error {
	t.ID = model.TicketID(t.Source, t.ExternalID)
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now
	if err := s.TicketRepo.Create(ctx, t); err != nil {
		return fmt.Errorf("create ticket: %w", err)
	}
	if s.auditSvc != nil {
		auditCtx := authctx.WithUserID(ctx, "system:sync")
		if err := s.auditSvc.Record(auditCtx, model.AuditActionTicketCreatedSync, "ticket", t.ID, "origin=sync"); err != nil {
			s.logger.Error("sync: failed to record ticket create audit event", "ticket_id", t.ID, "error", err)
		}
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
	if s.auditSvc != nil {
		auditCtx := authctx.WithUserID(ctx, "system:sync")
		if err := s.auditSvc.Record(auditCtx, model.AuditActionPRUpdatedSync, "pull_request", existing.ID, "origin=sync"); err != nil {
			s.logger.Error("sync: failed to record PR update audit event", "pr_id", existing.ID, "error", err)
		}
	}
	return nil
}

// createPR assigns a new ID and timestamps, then persists the PR.
func (s *SyncService) createPR(ctx context.Context, pr model.PullRequest) error {
	pr.ID = model.PRID(pr.Source, pr.ExternalID)
	now := time.Now()
	pr.CreatedAt = now
	pr.UpdatedAt = now
	if err := s.PRRepo.Create(ctx, pr); err != nil {
		return fmt.Errorf("create PR: %w", err)
	}
	if s.auditSvc != nil {
		auditCtx := authctx.WithUserID(ctx, "system:sync")
		if err := s.auditSvc.Record(auditCtx, model.AuditActionPRCreatedSync, "pull_request", pr.ID, "origin=sync"); err != nil {
			s.logger.Error("sync: failed to record PR create audit event", "pr_id", pr.ID, "error", err)
		}
	}
	return nil
}
