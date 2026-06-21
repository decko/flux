package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want %d", cfg.Server.Port, 8080)
	}
	if cfg.Database.Path != ":memory:" {
		t.Errorf("Database.Path = %q, want %q", cfg.Database.Path, ":memory:")
	}
	if cfg.CORS.Origin != "*" {
		t.Errorf("CORS.Origin = %q, want %q", cfg.CORS.Origin, "*")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "info")
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
