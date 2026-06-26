package model

import (
	"errors"
	"fmt"
	"time"
)

// DefaultEvent is the event type used for trigger rules when none is specified.
// It is the default event type for backward compatibility with existing rules.
const DefaultEvent = "ticket.labeled"

// ValidEvents lists all event types that can trigger pipeline runs.
// Rules can be configured to match one of these event types.
var ValidEvents = []string{"ticket.labeled", "issues", "pull_request", "push"}

// isValidEvent returns true if the given event string is a valid event type.
// An empty string is valid (backward compatibility — treated as DefaultEvent).
func isValidEvent(event string) bool {
	if event == "" {
		return true
	}
	for _, v := range ValidEvents {
		if event == v {
			return true
		}
	}
	return false
}

// EventMatchesRule returns true if the event matches the rule's event type.
// A rule with an empty event type matches all events (backward compatibility).
func EventMatchesRule(event, ruleEvent string) bool {
	if ruleEvent == "" {
		return true
	}
	return event == ruleEvent
}

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
	Event     string    `json:"event"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks that the trigger rule has all required fields populated.
// The Event field defaults to DefaultEvent if empty.
func (r TriggerRule) Validate() error {
	var errs []error
	if r.Label == "" {
		errs = append(errs, fmt.Errorf("trigger rule label is required"))
	}
	if r.Pipeline == "" {
		errs = append(errs, fmt.Errorf("trigger rule pipeline is required"))
	}
	if !isValidEvent(r.Event) {
		errs = append(errs, fmt.Errorf("invalid event %q: must be one of %v", r.Event, ValidEvents))
	}
	return errors.Join(errs...)
}
