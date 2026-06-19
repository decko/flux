package model

import (
	"errors"
	"fmt"
)

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

// isValidRelationType checks if the given relation type is a known type.
func isValidRelationType(r RelationType) bool {
	switch r {
	case RelationBlocks, RelationBlockedBy, RelationRelatesTo, RelationParent, RelationChild:
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

// isValidReviewStatus checks if the given status is a known review status.
func isValidReviewStatus(s ReviewStatus) bool {
	switch s {
	case ReviewStatusApproved, ReviewStatusChangesRequested, ReviewStatusCommented:
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
// Returns all validation errors joined together.
func (p Project) Validate() error {
	var errs []error
	if p.Name == "" {
		errs = append(errs, fmt.Errorf("project name is required"))
	}
	if p.RepoURL == "" {
		errs = append(errs, fmt.Errorf("project repo url is required"))
	}
	return errors.Join(errs...)
}

// Validate checks that the ticket has all required fields populated,
// that its Source and Status are valid enum values, and that its
// Relationships have valid types and target IDs.
func (t Ticket) Validate() error {
	var errs []error
	if t.Title == "" {
		errs = append(errs, fmt.Errorf("ticket title is required"))
	}
	if t.ProjectID == "" {
		errs = append(errs, fmt.Errorf("ticket project id is required"))
	}
	if !isValidTicketSource(t.Source) {
		errs = append(errs, fmt.Errorf("invalid ticket source: %s", t.Source))
	}
	if !isValidTicketStatus(t.Status) {
		errs = append(errs, fmt.Errorf("invalid ticket status: %s", t.Status))
	}
	for i, rel := range t.Relationships {
		if !isValidRelationType(rel.Type) {
			errs = append(errs, fmt.Errorf("invalid relation type at index %d: %s", i, rel.Type))
		}
		if rel.TargetID == "" {
			errs = append(errs, fmt.Errorf("empty target id in relationship at index %d", i))
		}
	}
	return errors.Join(errs...)
}

// Validate checks that the pull request has all required fields populated,
// that its Source, Status, and Review statuses are valid enum values.
func (pr PullRequest) Validate() error {
	var errs []error
	if pr.Title == "" {
		errs = append(errs, fmt.Errorf("pull request title is required"))
	}
	if pr.ProjectID == "" {
		errs = append(errs, fmt.Errorf("pull request project id is required"))
	}
	if pr.URL == "" {
		errs = append(errs, fmt.Errorf("pull request url is required"))
	}
	if !isValidPRSource(pr.Source) {
		errs = append(errs, fmt.Errorf("invalid pull request source: %s", pr.Source))
	}
	if !isValidPRStatus(pr.Status) {
		errs = append(errs, fmt.Errorf("invalid pull request status: %s", pr.Status))
	}
	for i, rev := range pr.Reviews {
		if !isValidReviewStatus(rev.Status) {
			errs = append(errs, fmt.Errorf("invalid review status at index %d: %s", i, rev.Status))
		}
	}
	return errors.Join(errs...)
}

// Validate checks that the pipeline run has all required fields populated,
// that its Status is a valid enum value, and that all phases have valid statuses.
func (r PipelineRun) Validate() error {
	var errs []error
	if r.ProjectID == "" {
		errs = append(errs, fmt.Errorf("pipeline run project id is required"))
	}
	if r.TicketID == "" {
		errs = append(errs, fmt.Errorf("pipeline run ticket id is required"))
	}
	if r.Orchestrator == "" {
		errs = append(errs, fmt.Errorf("pipeline run orchestrator is required"))
	}
	if r.Pipeline == "" {
		errs = append(errs, fmt.Errorf("pipeline run pipeline is required"))
	}
	if !isValidRunStatus(r.Status) {
		errs = append(errs, fmt.Errorf("invalid pipeline run status: %s", r.Status))
	}
	for i, phase := range r.Phases {
		if phase.Name == "" {
			errs = append(errs, fmt.Errorf("phase name is required at index %d", i))
		}
		if !isValidRunStatus(phase.Status) {
			errs = append(errs, fmt.Errorf("invalid phase status at index %d: %s", i, phase.Status))
		}
	}
	return errors.Join(errs...)
}
