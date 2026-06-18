package model

import "time"

type TicketSource string

const (
	TicketSourceGitHub TicketSource = "github"
	TicketSourceJira   TicketSource = "jira"
	TicketSourceLinear TicketSource = "linear"
)

type TicketStatus string

const (
	TicketStatusOpen     TicketStatus = "open"
	TicketStatusClosed   TicketStatus = "closed"
	TicketStatusInProgress TicketStatus = "in_progress"
)

type RelationType string

const (
	RelationBlocks    RelationType = "blocks"
	RelationBlockedBy RelationType = "blocked_by"
	RelationRelatesTo RelationType = "relates_to"
	RelationParent    RelationType = "parent"
	RelationChild     RelationType = "child"
)

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

type Relationship struct {
	Type     RelationType `json:"type"`
	TargetID string       `json:"target_id"`
}
