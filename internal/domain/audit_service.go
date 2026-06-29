package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	// Build hash chain: link this event to the previous event's hash.
	latest, err := s.repo.Latest(ctx)
	if err != nil {
		return fmt.Errorf("audit record: %w", err)
	}
	if latest != nil {
		event.PreviousHash = latest.Hash
	}
	event.Hash = hashEvent(event)

	if err := s.repo.Insert(ctx, event); err != nil {
		return fmt.Errorf("audit record: %w", err)
	}

	return nil
}

// AuditIntegrityResult describes the outcome of a hash chain integrity check.
type AuditIntegrityResult struct {
	Valid         bool    `json:"valid"`
	FirstBrokenAt *string `json:"first_broken_at,omitempty"`
}

// VerifyIntegrity walks all audit events in chronological order and verifies
// that each event's hash matches a recomputation of its fields linked to the
// previous event's hash. Returns the first broken event ID, or nil if the
// chain is intact.
func (s *AuditService) VerifyIntegrity(ctx context.Context) (*AuditIntegrityResult, error) {
	events, err := s.repo.List(ctx, repository.AuditFilter{})
	if err != nil {
		return nil, fmt.Errorf("verify integrity: %w", err)
	}

	// List returns DESC order; iterate backwards for ASC.
	var previousHash string
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		expected := hashEvent(model.AuditEvent{
			PreviousHash: previousHash,
			ActorID:      e.ActorID,
			Action:       e.Action,
			ResourceType: e.ResourceType,
			ResourceID:   e.ResourceID,
			Metadata:     e.Metadata,
			CreatedAt:    e.CreatedAt,
		})
		if expected != e.Hash {
			id := e.ID
			return &AuditIntegrityResult{Valid: false, FirstBrokenAt: &id}, nil
		}
		previousHash = e.Hash
	}

	return &AuditIntegrityResult{Valid: true}, nil
}

// hashEvent computes the SHA-256 hash of an audit event using its PreviousHash,
// ActorID, Action, ResourceType, ResourceID, Metadata, and CreatedAt fields.
// Fields are separated by null bytes to prevent concatenation collisions.
func hashEvent(e model.AuditEvent) string {
	input := e.PreviousHash + "\x00" +
		e.ActorID + "\x00" +
		string(e.Action) + "\x00" +
		e.ResourceType + "\x00" +
		e.ResourceID + "\x00" +
		e.Metadata + "\x00" +
		e.CreatedAt.String()
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}

// PurgeOldEvents deletes audit events older than retentionDays and logs the count.
// If retentionDays is <= 0, nothing is deleted.
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
		slog.Info("purged old audit events", "count", count, "retention_days", retentionDays)
	}
	return nil
}
