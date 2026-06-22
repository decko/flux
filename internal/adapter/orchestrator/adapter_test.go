package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/decko/flux/internal/adapter"
	"github.com/decko/flux/internal/model"
)

// stubOrchestratorAdapter is a minimal implementation of OrchestratorAdapter
// used for testing interface compliance. All methods return ErrNotImplemented
// except Name, which returns "test-stub".
type stubOrchestratorAdapter struct{}

func (s *stubOrchestratorAdapter) Name() string { return "test-stub" }

func (s *stubOrchestratorAdapter) Trigger(_ context.Context, _ model.PipelineRun) error {
	return adapter.ErrNotImplemented
}

func (s *stubOrchestratorAdapter) Status(_ context.Context, _ string) (*model.PipelineRun, error) {
	return nil, adapter.ErrNotImplemented
}

func (s *stubOrchestratorAdapter) Cancel(_ context.Context, _ string) error {
	return adapter.ErrNotImplemented
}

func (s *stubOrchestratorAdapter) Logs(_ context.Context, _ string) (<-chan LogEntry, error) {
	return nil, adapter.ErrNotImplemented
}

func (s *stubOrchestratorAdapter) Health(_ context.Context) error {
	return adapter.ErrNotImplemented
}

// Compile-time check: verify stubOrchestratorAdapter satisfies OrchestratorAdapter.
var _ OrchestratorAdapter = (*stubOrchestratorAdapter)(nil)

func TestStubSatisfiesOrchestratorAdapter(t *testing.T) {
	t.Parallel()

	stub := &stubOrchestratorAdapter{}
	ctx := context.Background()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Name returns test-stub",
			run: func(t *testing.T) {
				if got := stub.Name(); got != "test-stub" {
					t.Errorf("Name() = %q, want %q", got, "test-stub")
				}
			},
		},
		{
			name: "Trigger returns ErrNotImplemented",
			run: func(t *testing.T) {
				err := stub.Trigger(ctx, model.PipelineRun{})
				if err == nil {
					t.Fatal("Trigger() = nil, want ErrNotImplemented")
				}
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Errorf("Trigger() err = %v, want ErrNotImplemented", err)
				}
			},
		},
		{
			name: "Status returns nil and ErrNotImplemented",
			run: func(t *testing.T) {
				run, err := stub.Status(ctx, "test-run-id")
				if run != nil {
					t.Errorf("Status() run = %v, want nil", run)
				}
				if err == nil {
					t.Fatal("Status() err = nil, want ErrNotImplemented")
				}
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Errorf("Status() err = %v, want ErrNotImplemented", err)
				}
			},
		},
		{
			name: "Cancel returns ErrNotImplemented",
			run: func(t *testing.T) {
				err := stub.Cancel(ctx, "test-run-id")
				if err == nil {
					t.Fatal("Cancel() = nil, want ErrNotImplemented")
				}
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Errorf("Cancel() err = %v, want ErrNotImplemented", err)
				}
			},
		},
		{
			name: "Logs returns nil channel and ErrNotImplemented",
			run: func(t *testing.T) {
				ch, err := stub.Logs(ctx, "test-run-id")
				if ch != nil {
					t.Errorf("Logs() channel = %v, want nil", ch)
				}
				if err == nil {
					t.Fatal("Logs() err = nil, want ErrNotImplemented")
				}
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Errorf("Logs() err = %v, want ErrNotImplemented", err)
				}
			},
		},
		{
			name: "Health returns ErrNotImplemented",
			run: func(t *testing.T) {
				err := stub.Health(ctx)
				if err == nil {
					t.Fatal("Health() = nil, want ErrNotImplemented")
				}
				if !errors.Is(err, adapter.ErrNotImplemented) {
					t.Errorf("Health() err = %v, want ErrNotImplemented", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestLogEntry(t *testing.T) {
	t.Parallel()

	now := time.Date(2025, 6, 22, 10, 0, 0, 0, time.UTC)
	entry := LogEntry{
		Timestamp: now,
		Level:     "info",
		Message:   "pipeline run started",
	}

	t.Run("fields are writable and readable", func(t *testing.T) {
		if entry.Timestamp != now {
			t.Errorf("Timestamp = %v, want %v", entry.Timestamp, now)
		}
		if entry.Level != "info" {
			t.Errorf("Level = %q, want %q", entry.Level, "info")
		}
		if entry.Message != "pipeline run started" {
			t.Errorf("Message = %q, want %q", entry.Message, "pipeline run started")
		}
	})

	t.Run("field types are correct", func(t *testing.T) {
		var zero LogEntry
		if _, ok := any(zero.Timestamp).(time.Time); !ok {
			t.Error("Timestamp field is not of type time.Time")
		}
		if _, ok := any(zero.Level).(string); !ok {
			t.Error("Level field is not of type string")
		}
		if _, ok := any(zero.Message).(string); !ok {
			t.Error("Message field is not of type string")
		}
	})
}
