package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func TestNewCursorCapsOpenCursorsPerPrincipal(t *testing.T) {
	clock := &testClock{now: time.Unix(1000, 0)}
	store := NewStore(clock)
	p := ports.Principal{Issuer: "iss", Subject: "sub", Audience: "aud", WorkspaceID: "ws-a"}
	first, err := store.NewCursor(p, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	var last ports.Cursor
	for i := 0; i < 10*MaxCursorsPerPrincipal; i++ {
		if last, err = store.NewCursor(p, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if got := store.OpenCursors(); got != MaxCursorsPerPrincipal {
		t.Fatalf("open cursors = %d, want %d", got, MaxCursorsPerPrincipal)
	}
	if _, err := store.Read(context.Background(), first); !errors.Is(err, ErrCursorBinding) {
		t.Fatalf("oldest cursor should be evicted, got %v", err)
	}
	if _, err := store.Read(context.Background(), last); err != nil {
		t.Fatalf("newest cursor should survive: %v", err)
	}
	other := p
	other.Subject = "other"
	if _, err := store.NewCursor(other, time.Minute); err != nil {
		t.Fatal(err)
	}
	if got := store.OpenCursors(); got != MaxCursorsPerPrincipal+1 {
		t.Fatalf("another principal must not evict: open = %d", got)
	}
}

func TestExpiredCursorsArePruned(t *testing.T) {
	clock := &testClock{now: time.Unix(1000, 0)}
	store := NewStore(clock)
	a := ports.Principal{Issuer: "iss", Subject: "a", Audience: "aud", WorkspaceID: "ws-a"}
	b := ports.Principal{Issuer: "iss", Subject: "b", Audience: "aud", WorkspaceID: "ws-a"}
	for i := 0; i < 5; i++ {
		if _, err := store.NewCursor(a, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	stale, err := store.NewCursor(b, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	clock.now = clock.now.Add(2 * time.Minute)
	if _, err := store.Read(context.Background(), stale); !errors.Is(err, ErrCursorExpired) {
		t.Fatalf("got %v", err)
	}
	if got := store.OpenCursors(); got != 5 {
		t.Fatalf("expired cursor read should delete it: open = %d", got)
	}
	if _, err := store.NewCursor(b, time.Minute); err != nil {
		t.Fatal(err)
	}
	if got := store.OpenCursors(); got != 1 {
		t.Fatalf("opening a cursor should prune expired ones: open = %d", got)
	}
	clock.now = clock.now.Add(2 * time.Minute)
	store.Purge()
	if got := store.OpenCursors(); got != 0 {
		t.Fatalf("purge should prune expired cursors: open = %d", got)
	}
}
