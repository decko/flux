package model

import "time"

type Project struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	RepoURL    string            `json:"repo_url"`
	Definition ProjectDefinition `json:"definition"`
	Adapters   []AdapterConfig   `json:"adapters"`
	Pipelines  []PipelineConfig  `json:"pipelines"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type ProjectDefinition struct {
	Language     string   `json:"language"`
	Framework    string   `json:"framework"`
	Conventions  []string `json:"conventions"`
	Architecture string   `json:"architecture"`
}

type AdapterConfig struct {
	Type   string            `json:"type"`
	Config map[string]string `json:"config"`
}

type PipelineConfig struct {
	Type   string            `json:"type"`
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
}
