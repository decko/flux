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
