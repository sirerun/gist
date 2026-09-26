package events

import (
	"context"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type testClock struct{ now time.Time }

func (c *testClock) Now() time.Time { return c.now }
func TestFeedBindsCursorToWorkspaceAndPrincipal(t *testing.T) {
	clock := &testClock{now: time.Unix(100, 0)}
	store := NewStore(clock)
	p := ports.Principal{Issuer: "iss", Subject: "sub", Audience: "aud", WorkspaceID: "ws-a"}
	c, err := store.NewCursor(p, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Read(context.Background(), ports.Cursor{ID: c.ID, WorkspaceID: "ws-b", PrincipalHash: c.PrincipalHash}); err != ErrCursorBinding {
		t.Fatalf("got %v", err)
	}
	e := ports.Event{ID: "1", WorkspaceID: "ws-a", Type: ports.EventVersionPublished, Subject: ports.ArtifactRef{WorkspaceID: "ws-a", Kind: ports.KindSkill, ID: "s", Version: "1"}, OccurredAt: 100}
	if err := store.Append(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	page, err := store.Read(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 1 {
		t.Fatalf("events=%d", len(page.Events))
	}
}
