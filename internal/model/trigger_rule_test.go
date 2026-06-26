package model

import (
	"testing"
)

func TestTriggerRule_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rule    TriggerRule
		wantErr bool
	}{
		{
			name: "valid rule",
			rule: TriggerRule{
				Label:    "bug",
				Pipeline: "fix-pipeline",
			},
			wantErr: false,
		},
		{
			name: "missing label",
			rule: TriggerRule{
				Pipeline: "fix-pipeline",
			},
			wantErr: true,
		},
		{
			name: "missing pipeline",
			rule: TriggerRule{
				Label: "bug",
			},
			wantErr: true,
		},
		{
			name:    "all fields empty",
			rule:    TriggerRule{},
			wantErr: true,
		},
		{
			name: "valid with default event",
			rule: TriggerRule{
				Label:    "bug",
				Pipeline: "fix-pipeline",
				Event:    "ticket.labeled",
			},
			wantErr: false,
		},
		{
			name: "valid with pull_request event",
			rule: TriggerRule{
				Label:    "bug",
				Pipeline: "fix-pipeline",
				Event:    "pull_request",
			},
			wantErr: false,
		},
		{
			name: "invalid event",
			rule: TriggerRule{
				Label:    "bug",
				Pipeline: "fix-pipeline",
				Event:    "invalid-event",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTriggerRule_Validate_DefaultsToTicketLabeled(t *testing.T) {
	// A rule with no event should still be valid (backward compat).
	rule := TriggerRule{
		Label:    "bug",
		Pipeline: "fix-pipeline",
	}
	if err := rule.Validate(); err != nil {
		t.Fatalf("unexpected error for rule without event: %v", err)
	}
}

func TestIsValidEvent(t *testing.T) {
	tests := []struct {
		event string
		valid bool
	}{
		{"", true},
		{"ticket.labeled", true},
		{"issues", true},
		{"pull_request", true},
		{"push", true},
		{"invalid-event", false},
		{"issues.labeled", false},
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			if got := isValidEvent(tt.event); got != tt.valid {
				t.Errorf("isValidEvent(%q) = %v, want %v", tt.event, got, tt.valid)
			}
		})
	}
}

func TestEventMatchesRule(t *testing.T) {
	tests := []struct {
		event     string
		ruleEvent string
		match     bool
	}{
		{"ticket.labeled", "", true},
		{"ticket.labeled", "ticket.labeled", true},
		{"pull_request", "pull_request", true},
		{"pull_request", "ticket.labeled", false},
		{"issues", "ticket.labeled", false},
		{"push", "push", true},
		{"push", "", true},
	}
	for _, tt := range tests {
		name := tt.event + "/" + tt.ruleEvent
		t.Run(name, func(t *testing.T) {
			if got := EventMatchesRule(tt.event, tt.ruleEvent); got != tt.match {
				t.Errorf("EventMatchesRule(%q, %q) = %v, want %v", tt.event, tt.ruleEvent, got, tt.match)
			}
		})
	}
}
