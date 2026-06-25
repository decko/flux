package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Database.Path != "flux.db" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, "flux.db")
	}
	if cfg.CORS.Origin != "*" {
		t.Errorf("CORS.Origin = %q, want %q", cfg.CORS.Origin, "*")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Audit.RetentionDays != 90 {
		t.Errorf("Audit.RetentionDays = %d, want %d", cfg.Audit.RetentionDays, 90)
	}
}

func TestLoadFromFile(t *testing.T) {
	content := []byte(`
server:
  port: 9090
database:
  path: /tmp/test.db
cors:
  origin: http://localhost:3000
logging:
  level: debug
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, "/tmp/test.db")
	}
	if cfg.CORS.Origin != "http://localhost:3000" {
		t.Errorf("CORS.Origin = %q, want %q", cfg.CORS.Origin, "http://localhost:3000")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("FLUX_SERVER_PORT", "3000")
	t.Setenv("FLUX_DATABASE_PATH", "/env/test.db")
	t.Setenv("FLUX_CORS_ORIGIN", "http://env.example.com")
	t.Setenv("FLUX_LOGGING_LEVEL", "warn")

	content := []byte(`
server:
  port: 8080
database:
  path: ":memory:"
cors:
  origin: "*"
logging:
  level: info
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 3000)
	}
	if cfg.Database.Path != "/env/test.db" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, "/env/test.db")
	}
	if cfg.CORS.Origin != "http://env.example.com" {
		t.Errorf("CORS.Origin = %q, want %q", cfg.CORS.Origin, "http://env.example.com")
	}
	if cfg.Logging.Level != "warn" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "warn")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Server:   ServerConfig{Port: 8080},
				Database: DatabaseConfig{Path: ":memory:"},
				CORS:     CORSConfig{Origin: "*"},
				Logging:  LoggingConfig{Level: "info"},
			},
			wantErr: false,
		},
		{
			name: "invalid port - zero",
			config: Config{
				Server: ServerConfig{Port: 0},
			},
			wantErr: true,
		},
		{
			name: "invalid port - negative",
			config: Config{
				Server: ServerConfig{Port: -1},
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: Config{
				Server: ServerConfig{Port: 65536},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			config: Config{
				Server:  ServerConfig{Port: 8080},
				Logging: LoggingConfig{Level: "invalid"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestExampleConfig(t *testing.T) {
	cfg, err := Load("../../flux.yaml.example")
	if err != nil {
		t.Fatalf("Load(flux.yaml.example) error = %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

// ---- NEW TESTS for Issue #50: adapter and sync configuration ----

func TestConfig_AdapterParsing(t *testing.T) {
	t.Parallel()

	yamlData := []byte("adapters:\n  - type: github\n    owner: decko\n    repo: flux\n")
	var cfg Config
	if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}
	if len(cfg.Adapters) != 1 {
		t.Fatalf("len(Adapters) = %d, want 1", len(cfg.Adapters))
	}
	if cfg.Adapters[0].Type != "github" {
		t.Errorf("Adapters[0].Type = %q, want %q", cfg.Adapters[0].Type, "github")
	}
	if cfg.Adapters[0].Owner != "decko" {
		t.Errorf("Adapters[0].Owner = %q, want %q", cfg.Adapters[0].Owner, "decko")
	}
	if cfg.Adapters[0].Repo != "flux" {
		t.Errorf("Adapters[0].Repo = %q, want %q", cfg.Adapters[0].Repo, "flux")
	}
}

func TestConfig_SyncParsing(t *testing.T) {
	t.Parallel()

	yamlData := []byte("sync:\n  interval: 5m\n")
	var cfg Config
	if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}
	if cfg.Sync.Interval != "5m" {
		t.Errorf("Sync.Interval = %q, want %q", cfg.Sync.Interval, "5m")
	}
}

func TestConfig_AdapterDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Adapters == nil {
		t.Error("Adapters is nil, want empty slice (not nil)")
	}
	if len(cfg.Adapters) != 0 {
		t.Errorf("len(Adapters) = %d, want 0", len(cfg.Adapters))
	}
}

func TestConfig_SyncDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Sync.Interval != "5m" {
		t.Errorf("Sync.Interval = %q, want %q", cfg.Sync.Interval, "5m")
	}
}

func TestConfig_Validate_AdapterMissingFields(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Adapters: []AdapterEntry{
			{Type: "github", Repo: "flux"}, // missing Owner
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for adapter with empty Owner, got nil")
	}
}

func TestConfig_Validate_AdapterMissingRepo(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Adapters: []AdapterEntry{
			{Type: "github", Owner: "decko"}, // missing Repo
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for adapter with empty Repo, got nil")
	}
}

func TestConfig_Validate_InvalidSyncInterval(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Sync:     SyncConfig{Interval: "xyz"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for invalid sync interval \"xyz\", got nil")
	}
}

func TestConfig_Validate_EmptySyncInterval(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Sync:     SyncConfig{Interval: ""},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfig_Validate_UnknownAdapterType(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Adapters: []AdapterEntry{
			{Type: "unknown", Owner: "decko", Repo: "flux"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for unknown adapter type \"unknown\", got nil")
	}
}

func TestConfig_Validate_EmptyAdapterType(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Adapters: []AdapterEntry{
			{Type: "", Owner: "decko", Repo: "flux"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for empty adapter type, got nil")
	}
}

func TestConfig_FullConfigWithAdapters(t *testing.T) {
	t.Parallel()

	content := []byte(`
server:
  port: 9090
database:
  path: /tmp/test.db
cors:
  origin: http://localhost:3000
logging:
  level: debug
adapters:
  - type: github
    owner: decko
    repo: flux
sync:
  interval: 10m
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify existing fields still parse correctly.
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, "/tmp/test.db")
	}
	if cfg.CORS.Origin != "http://localhost:3000" {
		t.Errorf("CORS.Origin = %q, want %q", cfg.CORS.Origin, "http://localhost:3000")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}

	// Verify new adapter and sync fields.
	if len(cfg.Adapters) != 1 {
		t.Fatalf("len(Adapters) = %d, want 1", len(cfg.Adapters))
	}
	if cfg.Adapters[0].Type != "github" {
		t.Errorf("Adapters[0].Type = %q, want %q", cfg.Adapters[0].Type, "github")
	}
	if cfg.Adapters[0].Owner != "decko" {
		t.Errorf("Adapters[0].Owner = %q, want %q", cfg.Adapters[0].Owner, "decko")
	}
	if cfg.Adapters[0].Repo != "flux" {
		t.Errorf("Adapters[0].Repo = %q, want %q", cfg.Adapters[0].Repo, "flux")
	}
	if cfg.Sync.Interval != "10m" {
		t.Errorf("Sync.Interval = %q, want %q", cfg.Sync.Interval, "10m")
	}
}

func TestConfig_EnvOverrides(t *testing.T) {
	t.Setenv("FLUX_SERVER_PORT", "3000")

	content := []byte(`
server:
  port: 8080
adapters:
  - type: github
    owner: decko
    repo: flux
sync:
  interval: 5m
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// FLUX_SERVER_PORT env var must override the YAML value.
	if cfg.Server.Port != 3000 {
		t.Errorf("Server.Port = %d, want %d (env override)", cfg.Server.Port, 3000)
	}
	// New fields must still be parsed alongside env overrides.
	if len(cfg.Adapters) != 1 {
		t.Errorf("len(Adapters) = %d, want 1", len(cfg.Adapters))
	}
	if cfg.Sync.Interval != "5m" {
		t.Errorf("Sync.Interval = %q, want %q", cfg.Sync.Interval, "5m")
	}
}

func TestConfig_NoTokenInConfig(t *testing.T) {
	t.Parallel()

	// Ensure no token field leaks into the config structs.
	// The GitHub token is read from the GITHUB_TOKEN env var at
	// adapter construction time, never from config.
	types := []any{
		Config{},
		ServerConfig{},
		DatabaseConfig{},
		CORSConfig{},
		LoggingConfig{},
		AdapterEntry{},
		SyncConfig{},
		OrchestratorEntry{},
		PipelineDef{},
		AuditConfig{},
	}
	for _, v := range types {
		checkNoTokenField(t, v)
	}
}

// ---- NEW TESTS for Issue #65: orchestrator configuration ----

func TestConfig_OrchestratorParsing(t *testing.T) {
	t.Parallel()

	yamlData := []byte("orchestrators:\n  - type: soda\n    path: /usr/local/bin/soda\n    pipelines:\n      - name: default\n        config:\n          model: claude-sonnet\n")
	var cfg Config
	if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}
	if len(cfg.Orchestrators) != 1 {
		t.Fatalf("len(Orchestrators) = %d, want 1", len(cfg.Orchestrators))
	}
	if cfg.Orchestrators[0].Type != "soda" {
		t.Errorf("Orchestrators[0].Type = %q, want %q", cfg.Orchestrators[0].Type, "soda")
	}
	if cfg.Orchestrators[0].Path != "/usr/local/bin/soda" {
		t.Errorf("Orchestrators[0].Path = %q, want %q", cfg.Orchestrators[0].Path, "/usr/local/bin/soda")
	}
	if len(cfg.Orchestrators[0].Pipelines) != 1 {
		t.Fatalf("len(Orchestrators[0].Pipelines) = %d, want 1", len(cfg.Orchestrators[0].Pipelines))
	}
	if cfg.Orchestrators[0].Pipelines[0].Name != "default" {
		t.Errorf("Pipelines[0].Name = %q, want %q", cfg.Orchestrators[0].Pipelines[0].Name, "default")
	}
	if cfg.Orchestrators[0].Pipelines[0].Config["model"] != "claude-sonnet" {
		t.Errorf("Pipelines[0].Config[\"model\"] = %q, want %q", cfg.Orchestrators[0].Pipelines[0].Config["model"], "claude-sonnet")
	}
}

func TestConfig_OrchestratorDefaults(t *testing.T) {
	t.Parallel()

	content := []byte(`
server:
  port: 8080
database:
  path: ":memory:"
cors:
  origin: "*"
logging:
  level: info
orchestrators:
  - type: soda
    pipelines:
      - name: default
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Orchestrators) != 1 {
		t.Fatalf("len(Orchestrators) = %d, want 1", len(cfg.Orchestrators))
	}
	if cfg.Orchestrators[0].Path != "soda" {
		t.Errorf("Orchestrators[0].Path = %q, want %q (default)", cfg.Orchestrators[0].Path, "soda")
	}
}

func TestConfig_OrchestratorEmptySlice(t *testing.T) {
	t.Parallel()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Orchestrators == nil {
		t.Error("Orchestrators is nil, want empty slice (not nil)")
	}
	if len(cfg.Orchestrators) != 0 {
		t.Errorf("len(Orchestrators) = %d, want 0", len(cfg.Orchestrators))
	}
}

func TestConfig_Validate_OrchestratorMissingType(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Orchestrators: []OrchestratorEntry{
			{Type: ""},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for orchestrator with empty Type, got nil")
	}
}

func TestConfig_Validate_OrchestratorUnknownType(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Orchestrators: []OrchestratorEntry{
			{Type: "unknown"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for unknown orchestrator type \"unknown\", got nil")
	}
}

func TestConfig_Validate_OrchestratorPipelineMissingName(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Orchestrators: []OrchestratorEntry{
			{
				Type: "soda",
				Pipelines: []PipelineDef{
					{Name: ""},
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for pipeline with empty Name, got nil")
	}
}

func TestConfig_FullConfigWithOrchestrators(t *testing.T) {
	t.Parallel()

	content := []byte(`
server:
  port: 9090
database:
  path: /tmp/test.db
cors:
  origin: http://localhost:3000
logging:
  level: debug
adapters:
  - type: github
    owner: decko
    repo: flux
sync:
  interval: 10m
orchestrators:
  - type: soda
    path: /usr/local/bin/soda
    pipelines:
      - name: default
        config:
          model: claude-sonnet
          max_iterations: "10"
      - name: quick-fix
        config:
          model: claude-hyena
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify existing fields still parse correctly.
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 9090)
	}
	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, "/tmp/test.db")
	}
	if cfg.CORS.Origin != "http://localhost:3000" {
		t.Errorf("CORS.Origin = %q, want %q", cfg.CORS.Origin, "http://localhost:3000")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "debug")
	}
	if len(cfg.Adapters) != 1 {
		t.Fatalf("len(Adapters) = %d, want 1", len(cfg.Adapters))
	}
	if cfg.Adapters[0].Type != "github" {
		t.Errorf("Adapters[0].Type = %q, want %q", cfg.Adapters[0].Type, "github")
	}
	if cfg.Adapters[0].Owner != "decko" {
		t.Errorf("Adapters[0].Owner = %q, want %q", cfg.Adapters[0].Owner, "decko")
	}
	if cfg.Adapters[0].Repo != "flux" {
		t.Errorf("Adapters[0].Repo = %q, want %q", cfg.Adapters[0].Repo, "flux")
	}
	if cfg.Sync.Interval != "10m" {
		t.Errorf("Sync.Interval = %q, want %q", cfg.Sync.Interval, "10m")
	}

	// Verify orchestrator fields.
	if len(cfg.Orchestrators) != 1 {
		t.Fatalf("len(Orchestrators) = %d, want 1", len(cfg.Orchestrators))
	}
	if cfg.Orchestrators[0].Type != "soda" {
		t.Errorf("Orchestrators[0].Type = %q, want %q", cfg.Orchestrators[0].Type, "soda")
	}
	if cfg.Orchestrators[0].Path != "/usr/local/bin/soda" {
		t.Errorf("Orchestrators[0].Path = %q, want %q", cfg.Orchestrators[0].Path, "/usr/local/bin/soda")
	}
	if len(cfg.Orchestrators[0].Pipelines) != 2 {
		t.Fatalf("len(Orchestrators[0].Pipelines) = %d, want 2", len(cfg.Orchestrators[0].Pipelines))
	}
	if cfg.Orchestrators[0].Pipelines[0].Name != "default" {
		t.Errorf("Pipelines[0].Name = %q, want %q", cfg.Orchestrators[0].Pipelines[0].Name, "default")
	}
	if cfg.Orchestrators[0].Pipelines[0].Config["model"] != "claude-sonnet" {
		t.Errorf("Pipelines[0].Config[\"model\"] = %q, want %q", cfg.Orchestrators[0].Pipelines[0].Config["model"], "claude-sonnet")
	}
	if cfg.Orchestrators[0].Pipelines[0].Config["max_iterations"] != "10" {
		t.Errorf("Pipelines[0].Config[\"max_iterations\"] = %q, want %q", cfg.Orchestrators[0].Pipelines[0].Config["max_iterations"], "10")
	}
	if cfg.Orchestrators[0].Pipelines[1].Name != "quick-fix" {
		t.Errorf("Pipelines[1].Name = %q, want %q", cfg.Orchestrators[0].Pipelines[1].Name, "quick-fix")
	}
	if cfg.Orchestrators[0].Pipelines[1].Config["model"] != "claude-hyena" {
		t.Errorf("Pipelines[1].Config[\"model\"] = %q, want %q", cfg.Orchestrators[0].Pipelines[1].Config["model"], "claude-hyena")
	}
}

// ─── Audit Config Tests ────────────────────────────────────────────────────

func TestConfig_AuditDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Audit.RetentionDays != 90 {
		t.Errorf("Audit.RetentionDays = %d, want 90", cfg.Audit.RetentionDays)
	}
}

func TestConfig_AuditParsing(t *testing.T) {
	t.Parallel()

	content := []byte(`
server:
  port: 8080
database:
  path: ":memory:"
cors:
  origin: "*"
logging:
  level: info
audit:
  retention_days: 30
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Audit.RetentionDays != 30 {
		t.Errorf("Audit.RetentionDays = %d, want 30", cfg.Audit.RetentionDays)
	}
}

func TestConfig_AuditCustomRetention(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Audit:    AuditConfig{RetentionDays: 7},
	}

	err := cfg.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Audit.RetentionDays != 7 {
		t.Errorf("Audit.RetentionDays = %d, want 7", cfg.Audit.RetentionDays)
	}
}

// ─── Installation ID Tests ──────────────────────────────────────────────────

func TestConfig_AdapterWithInstallationID(t *testing.T) {
	t.Parallel()

	yamlData := []byte("adapters:\n  - type: github\n    owner: decko\n    repo: flux\n    installation_id: 42\n")
	var cfg Config
	if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal error = %v", err)
	}
	if len(cfg.Adapters) != 1 {
		t.Fatalf("len(Adapters) = %d, want 1", len(cfg.Adapters))
	}
	if cfg.Adapters[0].InstallationID != 42 {
		t.Errorf("Adapters[0].InstallationID = %d, want %d", cfg.Adapters[0].InstallationID, 42)
	}
}

// checkNoTokenField verifies that a struct type has no field whose name
// or yaml tag contains "token".
func checkNoTokenField(t *testing.T, v any) {
	t.Helper()

	typ := reflect.TypeOf(v)
	for i := range typ.NumField() {
		field := typ.Field(i)
		name := strings.ToLower(field.Name)
		tag := field.Tag.Get("yaml")
		if strings.Contains(name, "token") || strings.Contains(tag, "token") {
			t.Errorf(
				"type %s has field %q containing 'token' (yaml tag: %q)",
				typ.Name(), field.Name, tag,
			)
		}
	}
}

// ─── Orchestrator self_user validation ──────────────────────────────────────

func TestConfig_Validate_OrchestratorMissingSelfUser(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Orchestrators: []OrchestratorEntry{
			{
				Type: "soda",
				Pipelines: []PipelineDef{
					{Name: "review"},
				},
				// SelfUser intentionally empty
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error for missing self_user with pipelines, got nil")
	}
}

func TestConfig_Validate_OrchestratorWithSelfUser(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Orchestrators: []OrchestratorEntry{
			{
				Type:     "soda",
				SelfUser: "flux-bot",
				Pipelines: []PipelineDef{
					{Name: "review"},
				},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_Validate_OrchestratorNoPipelinesNoSelfUser(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: ":memory:"},
		CORS:     CORSConfig{Origin: "*"},
		Logging:  LoggingConfig{Level: "info"},
		Orchestrators: []OrchestratorEntry{
			{
				Type: "soda",
				// No pipelines, no self_user — ok
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── IsBotActor ──────────────────────────────────────────────────────────────

func TestIsBotActor_Match(t *testing.T) {
	if !IsBotActor("flux-bot", "flux-bot") {
		t.Error("IsBotActor should match identical strings")
	}
}

func TestIsBotActor_CaseInsensitive(t *testing.T) {
	if !IsBotActor("Flux-Bot", "flux-bot") {
		t.Error("IsBotActor should be case-insensitive")
	}
}

func TestIsBotActor_DifferentActor(t *testing.T) {
	if IsBotActor("flux-bot", "decko") {
		t.Error("IsBotActor should not match different actor")
	}
}

func TestIsBotActor_EmptySelfUser(t *testing.T) {
	if IsBotActor("", "flux-bot") {
		t.Error("IsBotActor with empty selfUser should return false")
	}
}
