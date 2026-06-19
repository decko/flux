package model

import "time"

// PRSource identifies the origin system for a pull request.
type PRSource string

// Supported pull request source systems.
const (
	PRSourceGitHub PRSource = "github"
	PRSourceGitLab PRSource = "gitlab"
)

// PRStatus represents the lifecycle state of a pull request.
type PRStatus string

// Known pull request statuses.
const (
	PRStatusOpen   PRStatus = "open"
	PRStatusMerged PRStatus = "merged"
	PRStatusClosed PRStatus = "closed"
)

// PullRequest represents a pull request or merge request
// tracked from a source system like GitHub or GitLab.
type PullRequest struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	ExternalID string    `json:"external_id"`
	Source     PRSource  `json:"source"`
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	Status     PRStatus  `json:"status"`
	TicketIDs  []string  `json:"ticket_ids"`
	Reviews    []Review  `json:"reviews"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Review represents a code review submitted on a pull request.
type Review struct {
	Author    string    `json:"author"`
	Status    string    `json:"status"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}
