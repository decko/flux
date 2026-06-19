package model

import (
	"testing"
)

func TestProject_Validate(t *testing.T) {
	tests := []struct {
		name    string
		project Project
		wantErr bool
	}{
		{
			name: "valid project",
			project: Project{
				Name:    "test-project",
				RepoURL: "https://github.com/org/repo",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			project: Project{
				RepoURL: "https://github.com/org/repo",
			},
			wantErr: true,
		},
		{
			name: "missing repo url",
			project: Project{
				Name: "test-project",
			},
			wantErr: true,
		},
		{
			name:    "all fields empty",
			project: Project{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.project.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTicket_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ticket  Ticket
		wantErr bool
	}{
		{
			name: "valid ticket",
			ticket: Ticket{
				Title:     "Implement feature X",
				ProjectID: "proj-1",
				Source:    TicketSourceGitHub,
				Status:    TicketStatusOpen,
			},
			wantErr: false,
		},
		{
			name: "missing title",
			ticket: Ticket{
				ProjectID: "proj-1",
				Source:    TicketSourceGitHub,
				Status:    TicketStatusOpen,
			},
			wantErr: true,
		},
		{
			name: "missing project id",
			ticket: Ticket{
				Title:  "Implement feature X",
				Source: TicketSourceGitHub,
				Status: TicketStatusOpen,
			},
			wantErr: true,
		},
		{
			name: "invalid source",
			ticket: Ticket{
				Title:     "Implement feature X",
				ProjectID: "proj-1",
				Source:    "unknown",
				Status:    TicketStatusOpen,
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			ticket: Ticket{
				Title:     "Implement feature X",
				ProjectID: "proj-1",
				Source:    TicketSourceGitHub,
				Status:    "unknown",
			},
			wantErr: true,
		},
		{
			name:    "all fields empty",
			ticket:  Ticket{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ticket.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPullRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		pr      PullRequest
		wantErr bool
	}{
		{
			name: "valid pull request",
			pr: PullRequest{
				Title:     "Add feature X",
				ProjectID: "proj-1",
				URL:       "https://github.com/org/repo/pull/1",
				Source:    PRSourceGitHub,
				Status:    PRStatusOpen,
			},
			wantErr: false,
		},
		{
			name: "missing title",
			pr: PullRequest{
				URL:    "https://github.com/org/repo/pull/1",
				Source: PRSourceGitHub,
				Status: PRStatusOpen,
			},
			wantErr: true,
		},
		{
			name: "missing project id",
			pr: PullRequest{
				Title:  "Add feature X",
				URL:    "https://github.com/org/repo/pull/1",
				Source: PRSourceGitHub,
				Status: PRStatusOpen,
			},
			wantErr: true,
		},
		{
			name: "missing url",
			pr: PullRequest{
				Title:  "Add feature X",
				Source: PRSourceGitHub,
				Status: PRStatusOpen,
			},
			wantErr: true,
		},
		{
			name: "invalid source",
			pr: PullRequest{
				Title:  "Add feature X",
				URL:    "https://github.com/org/repo/pull/1",
				Source: "unknown",
				Status: PRStatusOpen,
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			pr: PullRequest{
				Title:  "Add feature X",
				URL:    "https://github.com/org/repo/pull/1",
				Source: PRSourceGitHub,
				Status: "unknown",
			},
			wantErr: true,
		},
		{
			name:    "all fields empty",
			pr:      PullRequest{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pr.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestPipelineRun_Validate(t *testing.T) {
	tests := []struct {
		name    string
		run     PipelineRun
		wantErr bool
	}{
		{
			name: "valid pipeline run",
			run: PipelineRun{
				ProjectID:    "proj-1",
				TicketID:     "ticket-1",
				Orchestrator: "soda",
				Pipeline:     "test-and-deploy",
				Status:       RunStatusPending,
			},
			wantErr: false,
		},
		{
			name: "missing project id",
			run: PipelineRun{
				TicketID:     "ticket-1",
				Orchestrator: "soda",
				Pipeline:     "test-and-deploy",
				Status:       RunStatusPending,
			},
			wantErr: true,
		},
		{
			name: "missing ticket id",
			run: PipelineRun{
				ProjectID:    "proj-1",
				Orchestrator: "soda",
				Pipeline:     "test-and-deploy",
				Status:       RunStatusPending,
			},
			wantErr: true,
		},
		{
			name: "missing orchestrator",
			run: PipelineRun{
				Pipeline: "test-and-deploy",
				Status:   RunStatusPending,
			},
			wantErr: true,
		},
		{
			name: "missing pipeline",
			run: PipelineRun{
				Orchestrator: "soda",
				Status:       RunStatusPending,
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			run: PipelineRun{
				Orchestrator: "soda",
				Pipeline:     "test-and-deploy",
				Status:       "unknown",
			},
			wantErr: true,
		},
		{
			name:    "all fields empty",
			run:     PipelineRun{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
