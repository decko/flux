package model

import (
	"errors"
	"time"
)

// AuditAction categorizes the type of operation recorded in an audit event.
// Convention follows "<resource>.<action>" format, e.g., "project.created".
type AuditAction string

// ErrInvalidAuditEvent is returned when an audit event fails validation.
var ErrInvalidAuditEvent = errors.New("invalid audit event")

// AuditEvent represents an immutable record of an action performed by an actor
// on a resource. Audit events are append-only and provide an authoritative log
// of changes within the system.
type AuditEvent struct {
	ID           string      `json:"id"`
	ActorID      string      `json:"actor_id"`
	Action       AuditAction `json:"action"`
	ResourceType string      `json:"resource_type"`
	ResourceID   string      `json:"resource_id"`
	Metadata     string      `json:"metadata"`
	CreatedAt    time.Time   `json:"created_at"`
}

// Validate checks that the audit event has all required fields populated.
// Returns ErrInvalidAuditEvent if ActorID, Action, or ResourceType is empty.
func (e AuditEvent) Validate() error {
	if e.ActorID == "" || e.Action == "" || e.ResourceType == "" {
		return ErrInvalidAuditEvent
	}
	return nil
}
