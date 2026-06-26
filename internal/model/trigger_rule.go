package model

import "time"

// TriggerRule defines a condition under which a pipeline is automatically
// triggered for a project. Each rule specifies a label filter and the
// target pipeline to run when a ticket matching the label is moved to
// an actionable state.
type TriggerRule struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Label     string    `json:"label"`
	Pipeline  string    `json:"pipeline"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
