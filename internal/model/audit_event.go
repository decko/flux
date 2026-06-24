package model

import (
	"errors"
	"fmt"
	"time"
)

// AuditAction is the type for audit event action strings.
type AuditAction string

// ErrInvalidAuditEvent is returned when an audit event fails validation.
var ErrInvalidAuditEvent = errors.New("invalid audit event")

// AuditEvent represents a single append-only audit log entry with hash chain
// integrity protection. Each event records an action performed by an actor
// on a resource. The PreviousHash and Hash fields form a tamper-evident chain
// across all events.
//
// The Hash is computed as SHA256(previous_hash + actor_id + action +
// resource_type + resource_id + created_at). The PreviousHash of the first
// event is the empty string.
type AuditEvent struct {
	ID           string      `json:"id"`
	ActorID      string      `json:"actor_id"`
	Action       AuditAction `json:"action"`
	ResourceType string      `json:"resource_type"`
	ResourceID   string      `json:"resource_id"`
	Metadata     string      `json:"metadata,omitempty"`
	PreviousHash string      `json:"previous_hash"`
	Hash         string      `json:"hash"`
	CreatedAt    time.Time   `json:"created_at"`
}

// Validate checks that the audit event has all required fields.
// ActorID and Action must be non-empty. Returns ErrInvalidAuditEvent
// (wrapped with field errors) when validation fails.
func (e AuditEvent) Validate() error {
	var errs []error
	if e.ActorID == "" {
		errs = append(errs, fmt.Errorf("actor_id is required"))
	}
	if e.Action == "" {
		errs = append(errs, fmt.Errorf("action is required"))
	}
	if len(errs) > 0 {
		return errors.Join(append([]error{ErrInvalidAuditEvent}, errs...)...)
	}
	return nil
}
