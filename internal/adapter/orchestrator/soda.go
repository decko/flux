package orchestrator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/decko/flux/internal/model"
)

// Sentinel errors returned by SodaAdapter methods.
var (
	ErrSodaNotFound = errors.New("soda binary not found")
	ErrSodaFailed   = errors.New("soda command failed")
)

// SodaAdapter implements OrchestratorAdapter by executing the soda CLI binary
// as a subprocess. All commands use exec.CommandContext for context propagation
// and timeout support.
type SodaAdapter struct {
	path       string // path to the soda binary
	configPath string // path to soda config file
}

// SodaOption configures a SodaAdapter.
type SodaOption func(*SodaAdapter)

// WithSodaConfig sets the soda config file path (--config flag).
func WithSodaConfig(configPath string) SodaOption {
	return func(a *SodaAdapter) { a.configPath = configPath }
}

// NewSodaAdapter creates a new SodaAdapter that invokes the binary at sodaPath.
func NewSodaAdapter(sodaPath string, opts ...SodaOption) *SodaAdapter {
	a := &SodaAdapter{path: sodaPath}
	for _, o := range opts {
		o(a)
	}
	return a
}

// Name returns the adapter identifier "soda".
func (a *SodaAdapter) Name() string { return "soda" }

// sodaArgs builds the argument list for a soda command, including the config flag.
func (a *SodaAdapter) sodaArgs(args ...string) []string {
	if a.configPath != "" {
		return append([]string{"--config", a.configPath}, args...)
	}
	return args
}

// Health verifies that the soda binary exists and is executable by running
// "soda --version".
func (a *SodaAdapter) Health(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, a.path, a.sodaArgs("--version")...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("soda health: %w", err)
	}
	return nil
}

// Trigger starts a pipeline run by invoking "soda run <ticket>".
// soda executes asynchronously; this method waits for the command to complete.
func (a *SodaAdapter) Trigger(ctx context.Context, run model.PipelineRun) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("soda trigger: %w", err)
	}
	args := []string{"run", run.TicketID}
	if run.Pipeline != "" {
		args = append(args, "--pipeline", run.Pipeline)
	}
	cmd := exec.CommandContext(ctx, a.path, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("soda trigger: %w", err)
	}
	return nil
}

// Status retrieves the current state of a pipeline run. It parses the JSON-RPC
// response from "soda status --run-id <runID>".
func (a *SodaAdapter) Status(ctx context.Context, runID string) (*model.PipelineRun, error) {
	cmd := exec.CommandContext(ctx, a.path, a.sodaArgs("status", "--run-id", runID)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("soda status: %w", err)
	}

	var resp sodaResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("soda status: parse: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("soda status: %s", resp.Error.Message)
	}
	return resp.Result, nil
}

// Cancel stops a running pipeline run by executing "soda cancel --run-id <runID>".
func (a *SodaAdapter) Cancel(ctx context.Context, runID string) error {
	cmd := exec.CommandContext(ctx, a.path, a.sodaArgs("cancel", "--run-id", runID)...)
	return cmd.Run()
}

// Logs streams log output from a pipeline run. It parses each stdout line as a
// JSON LogEntry and sends it to the returned channel. The channel is closed
// when the run completes or the context is canceled.
func (a *SodaAdapter) Logs(ctx context.Context, runID string) (<-chan LogEntry, error) {
	cmd := exec.CommandContext(ctx, a.path, a.sodaArgs("logs", "--run-id", runID)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("soda logs: pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("soda logs: start: %w", err)
	}

	ch := make(chan LogEntry)
	go func() {
		defer close(ch)
		defer cmd.Wait() //nolint:errcheck // reap child process

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var entry sodaLogEntry
			if err := json.Unmarshal(line, &entry); err != nil {
				continue
			}
			select {
			case ch <- LogEntry(entry):
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

// sodaResponse wraps the JSON-RPC response envelope returned by soda commands.
type sodaResponse struct {
	JSONRPC string             `json:"jsonrpc"`
	ID      int                `json:"id"`
	Result  *model.PipelineRun `json:"result,omitempty"`
	Error   *sodaError         `json:"error,omitempty"`
}

// sodaError represents a JSON-RPC error object.
type sodaError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// sodaLogEntry is a helper for unmarshaling individual JSON log lines from
// the soda logs stream into the domain LogEntry type.
type sodaLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}
