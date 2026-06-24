package domain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
	"github.com/decko/flux/pkg/authctx"
)

// AuditService provides business logic for recording audit events.
// It validates inputs, assigns IDs and timestamps, and delegates
// persistence to an AuditRepository.
type AuditService struct {
	repo repository.AuditRepository
}

// NewAuditService creates a new AuditService backed by the given repository.
func NewAuditService(repo repository.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

// List returns audit events matching the given filter criteria.
// Events are ordered by created_at descending (most recent first).
func (s *AuditService) List(ctx context.Context, filter repository.AuditFilter) ([]model.AuditEvent, error) {
	events, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("audit list: %w", err)
	}
	return events, nil
}

// Record creates an audit event from the given parameters. The actor's user ID
// is extracted from the context via authctx.UserID. A UUID is generated for
// the event ID and CreatedAt is set to the current UTC time.
//
// Returns an error wrapping ErrInvalidAuditEvent if validation fails or a
// repository error if the write fails.
func (s *AuditService) Record(ctx context.Context, action model.AuditAction, resourceType, resourceID, metadata string) error {
	event := model.AuditEvent{
		ID:           uuid.New().String(),
		ActorID:      authctx.UserID(ctx),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     metadata,
		CreatedAt:    time.Now().UTC(),
	}

	if err := event.Validate(); err != nil {
		return fmt.Errorf("audit record: %w", err)
	}

	if err := s.repo.Insert(ctx, event); err != nil {
		return fmt.Errorf("audit record: %w", err)
	}

	return nil
}

// PurgeOldEvents deletes audit events older than retentionDays and logs
// the count of deleted records. If retentionDays is <= 0, nothing is deleted.
func (s *AuditService) PurgeOldEvents(ctx context.Context, retentionDays int) error {
	if retentionDays <= 0 {
		return nil
	}

	before := time.Now().UTC().AddDate(0, 0, -retentionDays)
	count, err := s.repo.PurgeOlderThan(ctx, before)
	if err != nil {
		return fmt.Errorf("purge old events: %w", err)
	}
	if count > 0 {
		slog.Info("purged old audit events", "count", count, "retention_days", retentionDays, "before", before.Format(time.DateOnly))
	}
	return nil
}
