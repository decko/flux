// Package config provides configuration loading for the flux server.
// It supports loading from a YAML file with defaults and environment variable overrides.
package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int `yaml:"port"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	Origin string `yaml:"origin"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level string `yaml:"level"`
}

// Config is the top-level configuration for the flux server.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	CORS     CORSConfig     `yaml:"cors"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// Load reads configuration from a YAML file at path, applies defaults for zero-valued
// fields, and overrides with environment variables. If path is empty or the file does
// not exist, Load returns a configuration with defaults applied.
func Load(path string) (*Config, error) {
	cfg := &Config{}

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read config file: %w", err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse config file: %w", err)
			}
		}
	}

	// Apply defaults for zero-valued fields.
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = ":memory:"
	}
	if cfg.CORS.Origin == "" {
		cfg.CORS.Origin = "*"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}

	// Override with environment variables.
	if v := os.Getenv("FLUX_SERVER_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid FLUX_SERVER_PORT: %w", err)
		}
		cfg.Server.Port = port
	}
	if v := os.Getenv("FLUX_DATABASE_PATH"); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv("FLUX_CORS_ORIGIN"); v != "" {
		cfg.CORS.Origin = v
	}
	if v := os.Getenv("FLUX_LOGGING_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}

	return cfg, nil
}

// Validate checks that the configuration values are within acceptable ranges.
// It returns an error if any field is invalid.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server port %d out of range [1-65535]", c.Server.Port)
	}
	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
		// valid
	default:
		return fmt.Errorf("invalid log level %q, must be one of: debug, info, warn, error", c.Logging.Level)
	}
	return nil
}
