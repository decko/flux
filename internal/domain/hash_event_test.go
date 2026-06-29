package domain

import (
	"testing"
	"time"

	"github.com/decko/flux/internal/model"
)

func TestHashEvent_IncludesMetadata(t *testing.T) {
	e := model.AuditEvent{
		PreviousHash: "abc",
		ActorID:      "user-1",
		Action:       "user.created",
		ResourceType: "user",
		ResourceID:   "new-1",
		Metadata:     `{"ip":"10.0.0.1"}`,
		CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	h1 := hashEvent(e)

	// Change metadata, verify hash changes.
	e2 := e
	e2.Metadata = `{"ip":"192.168.1.1"}`
	h2 := hashEvent(e2)

	if h1 == h2 {
		t.Fatal("hash must change when Metadata changes — but both produced same hash")
	}
}

func TestHashEvent_NoCollision(t *testing.T) {
	// ActorID="ab", Action="cd" must NOT collide with ActorID="a", Action="bcd".
	e1 := model.AuditEvent{
		PreviousHash: "abc",
		ActorID:      "ab",
		Action:       "cd",
		ResourceType: "ticket",
		ResourceID:   "42",
		CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	e2 := e1
	e2.ActorID = "a"
	e2.Action = "bcd"

	h1 := hashEvent(e1)
	h2 := hashEvent(e2)

	if h1 == h2 {
		t.Fatal("hash collision: ActorID='ab',Action='cd' must differ from ActorID='a',Action='bcd'")
	}
}

func TestHashEvent_Deterministic(t *testing.T) {
	e := model.AuditEvent{
		PreviousHash: "abc",
		ActorID:      "user-1",
		Action:       "user.created",
		ResourceType: "user",
		ResourceID:   "new-1",
		Metadata:     `{"ip":"10.0.0.1"}`,
		CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	h1 := hashEvent(e)
	h2 := hashEvent(e)

	if h1 != h2 {
		t.Fatalf("hash must be deterministic: h1=%s, h2=%s", h1, h2)
	}
}
