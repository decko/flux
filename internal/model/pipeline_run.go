package model

import "time"

type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

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

type PhaseResult struct {
	Name      string        `json:"name"`
	Status    RunStatus     `json:"status"`
	Duration  time.Duration `json:"duration"`
	Output    string        `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	StartedAt time.Time     `json:"started_at"`
}

type CostBreakdown struct {
	Total    float64            `json:"total"`
	Currency string             `json:"currency"`
	ByPhase  map[string]float64 `json:"by_phase"`
}
