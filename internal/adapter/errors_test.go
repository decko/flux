package adapter

import (
	"errors"
	"testing"
)

func TestErrNotImplemented(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "is non-nil",
			run: func(t *testing.T) {
				if ErrNotImplemented == nil {
					t.Fatal("ErrNotImplemented is nil, want a non-nil error")
				}
			},
		},
		{
			name: "implements error interface",
			run: func(t *testing.T) {
				err := ErrNotImplemented
				if err == nil {
					t.Fatal("ErrNotImplemented assigned to error interface is nil")
				}
			},
		},
		{
			name: "is a sentinel usable with errors.Is",
			run: func(t *testing.T) {
				if !errors.Is(ErrNotImplemented, ErrNotImplemented) {
					t.Fatal("errors.Is(ErrNotImplemented, ErrNotImplemented) = false, want true")
				}
			},
		},
		{
			name: "has expected Error() message",
			run: func(t *testing.T) {
				want := "not implemented"
				if got := ErrNotImplemented.Error(); got != want {
					t.Errorf("ErrNotImplemented.Error() = %q, want %q", got, want)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}
