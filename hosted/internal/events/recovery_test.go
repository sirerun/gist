package events

import (
	"context"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func TestExpiredCursorRequiresResynchronization(t *testing.T) {
	clock := &testClock{now: time.Unix(100, 0)}
	s := NewStore(clock)
	p := ports.Principal{Issuer: "iss", Subject: "sub", Audience: "aud", WorkspaceID: "ws"}
	c, err := s.NewCursor(p, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(6 * time.Minute)
	if _, err := s.Read(context.Background(), c); err != ErrCursorExpired {
		t.Fatalf("got %v", err)
	}
}
func TestDuplicateEventsRemainSafeForReplay(t *testing.T) {
	s := NewStore(&testClock{now: time.Unix(1, 0)})
	e := ports.Event{ID: "event-1", WorkspaceID: "ws", Type: ports.EventVersionPublished, Subject: ports.ArtifactRef{WorkspaceID: "ws", Kind: ports.KindSkill, ID: "s", Version: "1"}, OccurredAt: 1}
	if err := s.Append(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if len(s.events) != 2 {
		t.Fatalf("events=%d", len(s.events))
	}
}
