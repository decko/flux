package model

import "fmt"

// isValidTicketSource checks if the given source is a known ticket source.
func isValidTicketSource(s TicketSource) bool {
	switch s {
	case TicketSourceGitHub, TicketSourceJira, TicketSourceLinear:
		return true
	default:
		return false
	}
}

// isValidTicketStatus checks if the given status is a known ticket status.
func isValidTicketStatus(s TicketStatus) bool {
	switch s {
	case TicketStatusOpen, TicketStatusClosed, TicketStatusInProgress:
		return true
	default:
		return false
	}
}

// isValidPRSource checks if the given source is a known PR source.
func isValidPRSource(s PRSource) bool {
	switch s {
	case PRSourceGitHub, PRSourceGitLab:
		return true
	default:
		return false
	}
}

// isValidPRStatus checks if the given status is a known PR status.
func isValidPRStatus(s PRStatus) bool {
	switch s {
	case PRStatusOpen, PRStatusMerged, PRStatusClosed:
		return true
	default:
		return false
	}
}

// isValidRunStatus checks if the given status is a known pipeline run status.
func isValidRunStatus(s RunStatus) bool {
	switch s {
	case RunStatusPending, RunStatusRunning, RunStatusCompleted, RunStatusFailed, RunStatusCanceled:
		return true
	default:
		return false
	}
}

// Validate checks that the project has all required fields populated.
func (p Project) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("project name is required")
	}
	if p.RepoURL == "" {
		return fmt.Errorf("project repo url is required")
	}
	return nil
}

// Validate checks that the ticket has all required fields populated
// and that its Source and Status are valid enum values.
func (t Ticket) Validate() error {
	if t.Title == "" {
		return fmt.Errorf("ticket title is required")
	}
	if t.ProjectID == "" {
		return fmt.Errorf("ticket project id is required")
	}
	if !isValidTicketSource(t.Source) {
		return fmt.Errorf("invalid ticket source: %s", t.Source)
	}
	if !isValidTicketStatus(t.Status) {
		return fmt.Errorf("invalid ticket status: %s", t.Status)
	}
	return nil
}

// Validate checks that the pull request has all required fields populated
// and that its Source and Status are valid enum values.
func (pr PullRequest) Validate() error {
	if pr.Title == "" {
		return fmt.Errorf("pull request title is required")
	}
	if pr.URL == "" {
		return fmt.Errorf("pull request url is required")
	}
	if !isValidPRSource(pr.Source) {
		return fmt.Errorf("invalid pull request source: %s", pr.Source)
	}
	if !isValidPRStatus(pr.Status) {
		return fmt.Errorf("invalid pull request status: %s", pr.Status)
	}
	return nil
}

// Validate checks that the pipeline run has all required fields populated
// and that its Status is a valid enum value.
func (r PipelineRun) Validate() error {
	if r.Orchestrator == "" {
		return fmt.Errorf("pipeline run orchestrator is required")
	}
	if r.Pipeline == "" {
		return fmt.Errorf("pipeline run pipeline is required")
	}
	if !isValidRunStatus(r.Status) {
		return fmt.Errorf("invalid pipeline run status: %s", r.Status)
	}
	return nil
}
