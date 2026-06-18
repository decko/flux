package model

import "time"

type PRSource string

const (
	PRSourceGitHub PRSource = "github"
	PRSourceGitLab PRSource = "gitlab"
)

type PRStatus string

const (
	PRStatusOpen   PRStatus = "open"
	PRStatusMerged PRStatus = "merged"
	PRStatusClosed PRStatus = "closed"
)

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

type Review struct {
	Author    string    `json:"author"`
	Status    string    `json:"status"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}
