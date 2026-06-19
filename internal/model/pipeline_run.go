package model

import "time"

// RunStatus represents the lifecycle state of a pipeline run.
type RunStatus string

// Known pipeline run statuses.
const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

// PipelineRun represents a single execution of an agentic pipeline
// for a given project and ticket, including its phases and cost.
type PipelineRun struct {
	ID           string         `json:"id"`
	ProjectID    string         `json:"project_id"`
	TicketID     string         `json:"ticket_id"`
	Orchestrator string         `json:"orchestrator"`
	Pipeline     string         `json:"pipeline"`
	Status       RunStatus      `json:"status"`
	Phases       []PhaseResult  `json:"phases"`
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	Cost         *CostBreakdown `json:"cost,omitempty"`
}

// PhaseResult captures the outcome of a single phase within a pipeline run,
// including its status, duration, and any output or error messages.
type PhaseResult struct {
	Name      string        `json:"name"`
	Status    RunStatus     `json:"status"`
	Duration  time.Duration `json:"duration"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	StartedAt time.Time     `json:"started_at"`
}

// CostBreakdown tracks the cost incurred by a pipeline run,
// broken down by phase.
type CostBreakdown struct {
	Total    float64            `json:"total"`
	Currency string             `json:"currency"`
	ByPhase  map[string]float64 `json:"by_phase"`
}
