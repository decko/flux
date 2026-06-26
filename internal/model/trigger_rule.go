package model

import (
	"errors"
	"fmt"
	"time"
)

// TriggerRule defines an automatic pipeline trigger configuration for a project.
// When a ticket matches a trigger rule's label, the specified pipeline is
// automatically triggered for that ticket. Rules can be ordered by priority
// and disabled without deletion.
type TriggerRule struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Label     string    `json:"label"`
	Pipeline  string    `json:"pipeline"`
	Enabled   bool      `json:"enabled"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks that the trigger rule has all required fields populated.
func (r TriggerRule) Validate() error {
	var errs []error
	if r.Label == "" {
		errs = append(errs, fmt.Errorf("trigger rule label is required"))
	}
	if r.Pipeline == "" {
		errs = append(errs, fmt.Errorf("trigger rule pipeline is required"))
	}
	return errors.Join(errs...)
}
