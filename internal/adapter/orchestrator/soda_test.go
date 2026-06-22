package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/decko/flux/internal/model"
)

// writeSodaScript creates a temporary executable shell script with the given
// body (excluding shebang) and returns its path. The file is placed in a
// t.TempDir() directory and is automatically removed at test completion.
func writeSodaScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "soda.sh")
	content := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write soda script: %v", err)
	}
	return path
}

// Compile-time check: SodaAdapter satisfies OrchestratorAdapter.
var _ OrchestratorAdapter = (*SodaAdapter)(nil)

func TestNewSodaAdapter(t *testing.T) {
	t.Parallel()

	a := NewSodaAdapter("/usr/local/bin/soda")
	if a == nil {
		t.Fatal("NewSodaAdapter() returned nil")
	}
	if got := a.Name(); got != "soda" {
		t.Errorf("Name() = %q, want %q", got, "soda")
	}
}

func TestSodaAdapter_Health_Success(t *testing.T) {
	t.Parallel()

	sodaPath := writeSodaScript(t, `exit 0`)
	a := NewSodaAdapter(sodaPath)

	err := a.Health(context.Background())
	if err != nil {
		t.Fatalf("Health() = %v, want nil", err)
	}
}

func TestSodaAdapter_Health_NotFound(t *testing.T) {
	t.Parallel()

	a := NewSodaAdapter("/nonexistent/soda/binary")

	err := a.Health(context.Background())
	if err == nil {
		t.Fatal("Health() = nil, want error")
	}
}

func TestSodaAdapter_Trigger(t *testing.T) {
	t.Parallel()

	argsFile := filepath.Join(t.TempDir(), "trigger_args.txt")
	script := `echo "$*" > '` + argsFile + `'
echo '{"jsonrpc":"2.0","id":1,"result":{"status":"accepted"}}'`
	sodaPath := writeSodaScript(t, script)

	a := NewSodaAdapter(sodaPath)
	run := model.PipelineRun{
		ID:       "run-1",
		Pipeline: "default",
	}

	err := a.Trigger(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	args := string(data)
	if !strings.Contains(args, "trigger") {
		t.Errorf("soda args = %q, want to contain 'trigger'", args)
	}
	if !strings.Contains(args, "default") {
		t.Errorf("soda args = %q, want to contain 'default'", args)
	}
}

func TestSodaAdapter_Trigger_ContextCanceled(t *testing.T) {
	t.Parallel()

	sodaPath := writeSodaScript(t, `sleep 10 && echo '{}'`)
	a := NewSodaAdapter(sodaPath)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling Trigger

	err := a.Trigger(ctx, model.PipelineRun{})
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}

func TestSodaAdapter_Status(t *testing.T) {
	t.Parallel()

	script := `echo '{"jsonrpc":"2.0","id":1,"result":{"id":"run-123","project_id":"proj-1","ticket_id":"ticket-1","orchestrator":"soda","pipeline":"default","status":"running","phases":[{"name":"plan","status":"completed","duration":5000000000,"output":"plan output","error":"","started_at":"2025-06-22T10:00:00Z"}],"started_at":"2025-06-22T10:00:00Z"}}'`
	sodaPath := writeSodaScript(t, script)

	a := NewSodaAdapter(sodaPath)
	run, err := a.Status(context.Background(), "run-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run == nil {
		t.Fatal("expected non-nil run, got nil")
	}
	if run.ID != "run-123" {
		t.Errorf("run.ID = %q, want %q", run.ID, "run-123")
	}
	if run.Status != model.RunStatusRunning {
		t.Errorf("run.Status = %q, want %q", run.Status, model.RunStatusRunning)
	}
	if len(run.Phases) != 1 {
		t.Fatalf("len(run.Phases) = %d, want 1", len(run.Phases))
	}
	if run.Phases[0].Name != "plan" {
		t.Errorf("run.Phases[0].Name = %q, want %q", run.Phases[0].Name, "plan")
	}
	if run.Phases[0].Duration != 5*time.Second {
		t.Errorf("run.Phases[0].Duration = %v, want 5s", run.Phases[0].Duration)
	}
}

func TestSodaAdapter_Status_NotFound(t *testing.T) {
	t.Parallel()

	script := `echo '{"jsonrpc":"2.0","id":1,"error":{"code":-1,"message":"run not found"}}'`
	sodaPath := writeSodaScript(t, script)

	a := NewSodaAdapter(sodaPath)
	run, err := a.Status(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if run != nil {
		t.Errorf("expected nil run on error, got %v", run)
	}
}

func TestSodaAdapter_Cancel(t *testing.T) {
	t.Parallel()

	argsFile := filepath.Join(t.TempDir(), "cancel_args.txt")
	script := `echo "$*" > '` + argsFile + `'`
	sodaPath := writeSodaScript(t, script)

	a := NewSodaAdapter(sodaPath)
	err := a.Cancel(context.Background(), "run-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	args := string(data)
	if !strings.Contains(args, "cancel") {
		t.Errorf("soda args = %q, want to contain 'cancel'", args)
	}
	if !strings.Contains(args, "run-123") {
		t.Errorf("soda args = %q, want to contain 'run-123'", args)
	}
}

func TestSodaAdapter_Logs(t *testing.T) {
	t.Parallel()

	script := `echo '{"timestamp":"2025-06-22T10:00:00Z","level":"info","message":"started"}'
echo '{"timestamp":"2025-06-22T10:00:01Z","level":"debug","message":"processing phase 1"}'
echo '{"timestamp":"2025-06-22T10:00:02Z","level":"info","message":"completed"}'`
	sodaPath := writeSodaScript(t, script)

	a := NewSodaAdapter(sodaPath)
	ctx := context.Background()

	ch, err := a.Logs(ctx, "run-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("Logs() returned nil channel")
	}

	var entries []LogEntry
	for entry := range ch {
		entries = append(entries, entry)
	}

	if len(entries) != 3 {
		t.Fatalf("got %d log entries, want 3", len(entries))
	}

	want := []struct {
		level   string
		message string
	}{
		{level: "info", message: "started"},
		{level: "debug", message: "processing phase 1"},
		{level: "info", message: "completed"},
	}
	for i, w := range want {
		if entries[i].Level != w.level {
			t.Errorf("entries[%d].Level = %q, want %q", i, entries[i].Level, w.level)
		}
		if entries[i].Message != w.message {
			t.Errorf("entries[%d].Message = %q, want %q", i, entries[i].Message, w.message)
		}
		if entries[i].Timestamp.IsZero() {
			t.Errorf("entries[%d].Timestamp is zero", i)
		}
	}
}
