package model

import "testing"

func TestProject_InstallationID(t *testing.T) {
	p := Project{}
	if p.InstallationID != 0 {
		t.Errorf("default InstallationID = %d, want 0", p.InstallationID)
	}
	p.InstallationID = 42
	if p.InstallationID != 42 {
		t.Errorf("InstallationID = %d, want 42", p.InstallationID)
	}
}

func TestProject_ValidateInstallationID(t *testing.T) {
	tests := []struct {
		name    string
		id      int
		wantErr bool
	}{
		{name: "zero", id: 0, wantErr: false},
		{name: "positive", id: 42, wantErr: false},
		{name: "negative", id: -1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Project{
				Name:    "test",
				RepoURL: "https://github.com/org/repo",
			}
			p.InstallationID = tt.id
			err := p.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
