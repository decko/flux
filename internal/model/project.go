package model

import "time"

// Project represents a software project managed by the flux control plane.
// It includes the project definition, adapter configurations, and pipeline
// configurations that drive the agentic SDLC workflow.
type Project struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	RepoURL        string            `json:"repo_url"`
	InstallationID int               `json:"installation_id"`
<<<<<<< HEAD
	WebhookID      int               `json:"webhook_id"`
=======
	WebhookID      int               `json:"webhook_id,omitempty"`
>>>>>>> origin/main
	Definition     ProjectDefinition `json:"definition"`
	Adapters       []AdapterConfig   `json:"adapters"`
	Pipelines      []PipelineConfig  `json:"pipelines"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// ProjectDefinition describes the technical profile of a project,
// including language, framework, conventions, and architecture.
type ProjectDefinition struct {
	Language     string   `json:"language"`
	Framework    string   `json:"framework"`
	Conventions  []string `json:"conventions"`
	Architecture string   `json:"architecture"`
}

// AdapterConfig configures an external integration adapter
// (e.g., ticket source, orchestrator) for a project.
type AdapterConfig struct {
	Type   string            `json:"type"`
	Config map[string]string `json:"config"`
}

// PipelineConfig defines a named pipeline configuration
// with type-specific settings for orchestrator integration.
type PipelineConfig struct {
	Type   string            `json:"type"`
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
}
