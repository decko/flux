// Package orchestrator defines interfaces and types for integrating with
// agentic pipeline execution tools (soda, custom orchestrators). Adapters
// implementing these interfaces translate between flux domain types and the
// external orchestrator's API.
package orchestrator

import (
	"context"
	"time"

	"github.com/decko/flux/internal/model"
)

// OrchestratorAdapter defines the interface for triggering and monitoring
// agentic pipeline execution tools (soda, custom orchestrators). All methods
// accept a context for cancellation and timeout propagation.
type OrchestratorAdapter interface {
	// Name returns a human-readable name for this adapter (e.g. "soda").
	Name() string

	// Trigger starts a pipeline run. The implementation should begin
	// execution asynchronously and update the run status accordingly.
	Trigger(ctx context.Context, run model.PipelineRun) error

	// Status retrieves the current state of a pipeline run, including
	// phase results and cost data.
	Status(ctx context.Context, runID string) (*model.PipelineRun, error)

	// Cancel stops a running pipeline run. Has no effect on completed
	// or already canceled runs.
	Cancel(ctx context.Context, runID string) error

	// Logs streams log output from a pipeline run. The channel is closed
	// when the run completes or when the context is canceled.
	Logs(ctx context.Context, runID string) (<-chan LogEntry, error)

	// Health checks whether the orchestrator is reachable and functional.
	Health(ctx context.Context) error
}

// LogEntry represents a single log line from a pipeline run.
type LogEntry struct {
	// Timestamp is the time at which the log line was emitted.
	Timestamp time.Time

	// Level is the severity of the log entry ("debug", "info", "warn", "error").
	Level string

	// Message is the log line content.
	Message string
}
