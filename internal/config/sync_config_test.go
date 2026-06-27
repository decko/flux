package config

import "testing"

func TestSyncConfig_IsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{
			name:    "nil defaults to true",
			enabled: nil,
			want:    true,
		},
		{
			name:    "explicit true",
			enabled: boolPtr(true),
			want:    true,
		},
		{
			name:    "explicit false",
			enabled: boolPtr(false),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := SyncConfig{Enabled: tt.enabled}
			if got := sc.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }
