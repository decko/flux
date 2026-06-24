package model

import "time"

// AuditEvent represents a single append-only audit log entry with hash chain
// integrity protection. Each event records an action performed by an actor
// on a resource, linked to the previous event via PreviousHash for tamper
// detection. The Hash field is the SHA-256 of the concatenation of
// PreviousHash, ActorID, Action, ResourceType, ResourceID, and CreatedAt.
//
// PreviousHash and Hash are not serialized to JSON (json:"-") to keep the
// API surface clean; they are verified server-side via the integrity endpoint.
type AuditEvent struct {
	ID           string    `json:"id"`
	ActorID      string    `json:"actor_id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	Metadata     string    `json:"metadata,omitempty"`
	PreviousHash string    `json:"-"` // not exposed via API
	Hash         string    `json:"-"` // not exposed via API
	CreatedAt    time.Time `json:"created_at"`
}
