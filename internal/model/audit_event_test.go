package model

import (
	"testing"
	"time"
)

func TestAuditEvent_Validation(t *testing.T) {
	tests := []struct {
		name    string
		event   AuditEvent
		wantErr bool
	}{
		{
			name: "valid event",
			event: AuditEvent{
				ID:           "aev-1",
				ActorID:      "user-1",
				Action:       AuditAction("project.created"),
				ResourceType: "project",
				ResourceID:   "proj-1",
				Metadata:     `{"ip":"10.0.0.1"}`,
				CreatedAt:    time.Now(),
			},
			wantErr: false,
		},
		{
			name: "empty actor id",
			event: AuditEvent{
				ID:           "aev-2",
				Action:       AuditAction("project.created"),
				ResourceType: "project",
				ResourceID:   "proj-1",
				CreatedAt:    time.Now(),
			},
			wantErr: true,
		},
		{
			name: "empty action",
			event: AuditEvent{
				ID:           "aev-3",
				ActorID:      "user-1",
				ResourceType: "project",
				ResourceID:   "proj-1",
				CreatedAt:    time.Now(),
			},
			wantErr: true,
		},
		{
			name: "empty resource type",
			event: AuditEvent{
				ID:         "aev-4",
				ActorID:    "user-1",
				Action:     AuditAction("project.created"),
				ResourceID: "proj-1",
				CreatedAt:  time.Now(),
			},
			wantErr: true,
		},
		{
			name:    "all fields empty",
			event:   AuditEvent{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAuditEvent_Fields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	event := AuditEvent{
		ID:           "aev-1",
		ActorID:      "user-1",
		Action:       AuditAction("project.created"),
		ResourceType: "project",
		ResourceID:   "proj-1",
		Metadata:     `{"ip":"10.0.0.1"}`,
		CreatedAt:    now,
	}

	if event.ID != "aev-1" {
		t.Errorf("ID = %q, want %q", event.ID, "aev-1")
	}
	if event.ActorID != "user-1" {
		t.Errorf("ActorID = %q, want %q", event.ActorID, "user-1")
	}
	if event.Action != AuditAction("project.created") {
		t.Errorf("Action = %q, want %q", event.Action, AuditAction("project.created"))
	}
	if event.ResourceType != "project" {
		t.Errorf("ResourceType = %q, want %q", event.ResourceType, "project")
	}
	if event.ResourceID != "proj-1" {
		t.Errorf("ResourceID = %q, want %q", event.ResourceID, "proj-1")
	}
	if event.Metadata != `{"ip":"10.0.0.1"}` {
		t.Errorf("Metadata = %q, want %q", event.Metadata, `{"ip":"10.0.0.1"}`)
	}
	if !event.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", event.CreatedAt, now)
	}
}
