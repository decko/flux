package model

import "time"

// TicketSource identifies the origin system for a ticket.
type TicketSource string

// Supported ticket source systems.
const (
	TicketSourceGitHub TicketSource = "github"
	TicketSourceJira   TicketSource = "jira"
	TicketSourceLinear TicketSource = "linear"
)

// TicketStatus represents the lifecycle state of a ticket.
type TicketStatus string

// Known ticket statuses.
const (
	TicketStatusOpen       TicketStatus = "open"
	TicketStatusClosed     TicketStatus = "closed"
	TicketStatusInProgress TicketStatus = "in_progress"
)

// RelationType defines the semantic relationship between two tickets.
type RelationType string

// Supported ticket relationship types.
const (
	RelationBlocks    RelationType = "blocks"
	RelationBlockedBy RelationType = "blocked_by"
	RelationRelatesTo RelationType = "relates_to"
	RelationParent    RelationType = "parent"
	RelationChild     RelationType = "child"
)

// Ticket represents an issue or task tracked by a ticket source
// (e.g., GitHub Issue, Jira ticket, Linear issue). It links to
// related pull requests and other tickets via typed relationships.
type Ticket struct {
	ID            string         `json:"id"`
	ProjectID     string         `json:"project_id"`
	ExternalID    string         `json:"external_id"`
	Source        TicketSource   `json:"source"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	Status        TicketStatus   `json:"status"`
	Labels        []string       `json:"labels"`
	Relationships []Relationship `json:"relationships"`
	PRs           []string       `json:"prs"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// Relationship links a ticket to another ticket with a typed relation.
type Relationship struct {
	Type     RelationType `json:"type"`
	TargetID string       `json:"target_id"`
}
