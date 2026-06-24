package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
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

// Record persists a new audit event with hash chain integrity. It:
//  1. Queries the most recent event to obtain its hash.
//  2. Computes SHA256(previous_hash + actor_id + action + resource_type + resource_id + created_at.String()).
//  3. Sets event.PreviousHash and event.Hash.
//  4. Inserts the event into the repository.
//
// event.CreatedAt is set to the current UTC time if zero.
// Returns an error if the repository insertion fails.
func (s *AuditService) Record(ctx context.Context, event *model.AuditEvent) error {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	// Query the most recent event for the previous hash.
	latest, err := s.repo.Latest(ctx)
	if err != nil && err != repository.ErrNotFound {
		return fmt.Errorf("get latest audit event: %w", err)
	}

	event.PreviousHash = latest.Hash

	// Compute the hash: SHA256(prev_hash + actor_id + action + resource_type + resource_id + created_at).
	h := sha256.New()
	h.Write([]byte(event.PreviousHash))
	h.Write([]byte(event.ActorID))
	h.Write([]byte(event.Action))
	h.Write([]byte(event.ResourceType))
	h.Write([]byte(event.ResourceID))
	h.Write([]byte(event.CreatedAt.Format(time.RFC3339Nano)))
	event.Hash = hex.EncodeToString(h.Sum(nil))

	if err := s.repo.Create(ctx, *event); err != nil {
		return fmt.Errorf("create audit event: %w", err)
	}
	return nil
}

// VerifyIntegrity iterates all audit events ordered by created_at ASC,
// recomputes each event's hash, and checks that it matches the stored hash.
// It also verifies that each event's previous_hash matches the prior event's
// hash (forming an unbroken chain).
//
// Returns valid=true and an empty firstBrokenAt when the chain is intact.
// Returns valid=false and the created_at timestamp of the first broken event
// when a mismatch is detected.
func (s *AuditService) VerifyIntegrity(ctx context.Context) (valid bool, firstBrokenAt string, err error) {
	events, err := s.repo.List(ctx)
	if err != nil {
		return false, "", fmt.Errorf("list audit events: %w", err)
	}

	if len(events) == 0 {
		return true, "", nil
	}

	var prevHash string
	for _, event := range events {
		// Verify previous_hash links to the prior event.
		if event.PreviousHash != prevHash {
			return false, event.CreatedAt.Format(time.RFC3339Nano), nil
		}

		// Recompute the expected hash.
		h := sha256.New()
		h.Write([]byte(event.PreviousHash))
		h.Write([]byte(event.ActorID))
		h.Write([]byte(event.Action))
		h.Write([]byte(event.ResourceType))
		h.Write([]byte(event.ResourceID))
		h.Write([]byte(event.CreatedAt.Format(time.RFC3339Nano)))
		expectedHash := hex.EncodeToString(h.Sum(nil))

		if event.Hash != expectedHash {
			return false, event.CreatedAt.Format(time.RFC3339Nano), nil
		}

		prevHash = event.Hash
	}

	return true, "", nil
}
