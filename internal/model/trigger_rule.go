package model

<<<<<<< HEAD
import "time"

// TriggerRule defines a condition under which a pipeline is automatically
// triggered for a project. Each rule specifies a label filter and the
// target pipeline to run when a ticket matching the label is moved to
// an actionable state.
=======
import (
	"errors"
	"fmt"
	"time"
)

// TriggerRule defines an automatic pipeline trigger configuration for a project.
// When a ticket matches a trigger rule's label, the specified pipeline is
// automatically triggered for that ticket. Rules can be ordered by priority
// and disabled without deletion.
>>>>>>> origin/main
type TriggerRule struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Label     string    `json:"label"`
	Pipeline  string    `json:"pipeline"`
<<<<<<< HEAD
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
=======
	Enabled   bool      `json:"enabled"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks that the trigger rule has all required fields populated.
// Returns all validation errors joined together.
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
>>>>>>> origin/main
