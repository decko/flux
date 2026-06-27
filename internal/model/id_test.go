package model_test

import (
	"testing"

	"github.com/decko/flux/internal/model"
)

func TestTicketID_Deterministic(t *testing.T) {
	// Same inputs should produce same ID every time
	id1 := model.TicketID("github", "42")
	id2 := model.TicketID("github", "42")
	if id1 != id2 {
		t.Errorf("TicketID not deterministic: %q != %q", id1, id2)
	}
}

func TestTicketID_DifferentInputs(t *testing.T) {
	// Different external IDs produce different internal IDs
	id1 := model.TicketID("github", "42")
	id2 := model.TicketID("github", "43")
	if id1 == id2 {
		t.Error("different external IDs should produce different internal IDs")
	}
}

func TestTicketID_DifferentSources(t *testing.T) {
	// Different sources produce different IDs even for same external ID
	id1 := model.TicketID("github", "42")
	id2 := model.TicketID("jira", "42")
	if id1 == id2 {
		t.Error("different sources should produce different internal IDs")
	}
}

func TestTicketID_NonEmpty(t *testing.T) {
	id := model.TicketID("github", "42")
	if id == "" {
		t.Error("TicketID should not be empty")
	}
	// Should be a valid UUID format (36 chars with hyphens)
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d: %q", len(id), id)
	}
}

func TestPRID_Deterministic(t *testing.T) {
	id1 := model.PRID("github", "42")
	id2 := model.PRID("github", "42")
	if id1 != id2 {
		t.Errorf("PRID not deterministic: %q != %q", id1, id2)
	}
}

func TestPRID_DifferentFromTicketID(t *testing.T) {
	// Same inputs for TicketID and PRID should produce DIFFERENT IDs
	// (they should use different namespaces so they don't collide)
	tid := model.TicketID("github", "42")
	pid := model.PRID("github", "42")
	if tid == pid {
		t.Error("TicketID and PRID must not collide for same inputs")
	}
}

func TestPRID_NonEmpty(t *testing.T) {
	id := model.PRID("github", "42")
	if id == "" {
		t.Error("PRID should not be empty")
	}
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d: %q", len(id), id)
	}
}
