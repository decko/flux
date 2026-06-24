package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
	"github.com/decko/flux/pkg/authctx"
)

// AuditService provides business logic for the append-only audit log with
// hash chain integrity protection. Each call to Record computes a SHA-256
// hash over the event payload and the previous event's hash, forming a chain
// that can be verified later via VerifyIntegrity.
type AuditService struct {
	repo repository.AuditRepository
}

// NewAuditService creates a new AuditService backed by the given repository.
func NewAuditService(repo repository.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

// AuditIntegrityResult contains the outcome of a hash chain integrity check.
type AuditIntegrityResult struct {
	Valid         bool
	FirstBrokenAt *time.Time
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
// Before inserting, the previous event's hash is loaded and a new hash is
// computed as SHA256(prevHash + actorID + action + resourceType + resourceID +
// createdAt string). The event's PreviousHash and Hash fields are set before
// insertion.
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

	// Load previous hash for chain linking.
	prevHash := ""
	latest, err := s.repo.Latest(ctx)
	if err != nil && err != repository.ErrNotFound {
		return fmt.Errorf("audit record: %w", err)
	}
	if latest != nil {
		prevHash = latest.Hash
	}

	event.PreviousHash = prevHash
	event.Hash = hashEvent(event)

	if err := s.repo.Insert(ctx, event); err != nil {
		return fmt.Errorf("audit record: %w", err)
	}

	return nil
}

// hashEvent computes SHA256(prevHash + actorID + action + resourceType +
// resourceID + createdAt string) and returns the hex-encoded digest.
func hashEvent(e model.AuditEvent) string {
	h := sha256.New()
	h.Write([]byte(e.PreviousHash))
	h.Write([]byte(e.ActorID))
	h.Write([]byte(string(e.Action)))
	h.Write([]byte(e.ResourceType))
	h.Write([]byte(e.ResourceID))
	h.Write([]byte(e.CreatedAt.String()))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyIntegrity iterates all audit events ordered by created_at ASC,
// recomputes each event's hash, and checks that it matches the stored hash.
// It also verifies that each event's PreviousHash matches the prior event's
// hash (forming an unbroken chain).
//
// Returns AuditIntegrityResult with Valid=true when the chain is intact.
// When a mismatch is detected, Valid=false and FirstBrokenAt points to the
// timestamp of the first event with a broken hash.
func (s *AuditService) VerifyIntegrity(ctx context.Context) (*AuditIntegrityResult, error) {
	// List all events ordered by created_at ASC.
	events, err := s.repo.List(ctx, repository.AuditFilter{Limit: 0})
	if err != nil {
		return nil, fmt.Errorf("verify integrity: %w", err)
	}

	if len(events) == 0 {
		return &AuditIntegrityResult{Valid: true}, nil
	}

	// Reverse the list since List returns DESC.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	var prevHash string
	for _, event := range events {
		// Verify PreviousHash links to the prior event's hash.
		if event.PreviousHash != prevHash {
			return &AuditIntegrityResult{Valid: false, FirstBrokenAt: &event.CreatedAt}, nil
		}

		// Recompute the expected hash and compare.
		expected := hashEvent(model.AuditEvent{
			PreviousHash: event.PreviousHash,
			ActorID:      event.ActorID,
			Action:       event.Action,
			ResourceType: event.ResourceType,
			ResourceID:   event.ResourceID,
			CreatedAt:    event.CreatedAt,
		})
		if event.Hash != expected {
			return &AuditIntegrityResult{Valid: false, FirstBrokenAt: &event.CreatedAt}, nil
		}

		prevHash = event.Hash
	}

	return &AuditIntegrityResult{Valid: true}, nil
}
