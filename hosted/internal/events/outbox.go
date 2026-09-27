package events

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

var ErrCursorExpired = errors.New("events: cursor expired; resynchronization required")
var ErrCursorBinding = errors.New("events: cursor binding mismatch")

type Clock interface{ Now() time.Time }
type Store struct {
	mu        sync.RWMutex
	clock     Clock
	retention time.Duration
	events    []ports.Event
	cursors   map[string]cursorState
	seq       uint64
}
type cursorState struct {
	cursor ports.Cursor
	next   int
	oldest time.Time
	seq    uint64
}

// MaxCursorsPerPrincipal bounds the open cursors one principal may hold.
// Opening one more evicts that principal's oldest cursor, so repeated
// cursorless GET /v1/events calls cannot grow memory without bound.
const MaxCursorsPerPrincipal = 16

func NewStore(clock Clock) *Store {
	if clock == nil {
		clock = realClock{}
	}
	return &Store{clock: clock, retention: 7 * 24 * time.Hour, cursors: make(map[string]cursorState)}
}
func (s *Store) Append(ctx context.Context, e ports.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e.WorkspaceID == "" || e.ID == "" || e.Subject.WorkspaceID != e.WorkspaceID {
		return errors.New("events: incomplete event")
	}
	if e.Type != ports.EventVersionPublished && e.Type != ports.EventVersionRevoked {
		return errors.New("events: invalid event type")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}
func (s *Store) NewCursor(principal ports.Principal, ttl time.Duration) (ports.Cursor, error) {
	if principal.WorkspaceID == "" || principal.Issuer == "" || principal.Subject == "" {
		return ports.Cursor{}, errors.New("events: incomplete principal")
	}
	if ttl <= 0 || ttl > 5*time.Minute {
		ttl = 5 * time.Minute
	}
	id, err := randomID()
	if err != nil {
		return ports.Cursor{}, err
	}
	c := ports.Cursor{ID: id, PrincipalHash: principalHash(principal), WorkspaceID: principal.WorkspaceID, ExpiresAt: s.clock.Now().Add(ttl).Unix()}
	now := s.clock.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(now)
	s.evictForPrincipalLocked(c.PrincipalHash)
	s.seq++
	s.cursors[id] = cursorState{cursor: c, next: 0, oldest: now, seq: s.seq}
	return c, nil
}

// pruneExpiredLocked drops every cursor whose TTL has passed. Callers hold
// s.mu. It runs on every cursor open and every purge, so no background
// goroutine is needed to keep the map bounded.
func (s *Store) pruneExpiredLocked(now time.Time) int {
	n := 0
	for id, state := range s.cursors {
		if now.Unix() >= state.cursor.ExpiresAt {
			delete(s.cursors, id)
			n++
		}
	}
	return n
}

// evictForPrincipalLocked removes the principal's oldest cursors until one
// more fits under MaxCursorsPerPrincipal. Callers hold s.mu.
func (s *Store) evictForPrincipalLocked(hash string) {
	for {
		count := 0
		oldestID := ""
		var oldestSeq uint64
		for id, state := range s.cursors {
			if state.cursor.PrincipalHash != hash {
				continue
			}
			count++
			if oldestID == "" || state.seq < oldestSeq {
				oldestID, oldestSeq = id, state.seq
			}
		}
		if count < MaxCursorsPerPrincipal {
			return
		}
		delete(s.cursors, oldestID)
	}
}

// OpenCursors reports how many cursors the store currently holds.
func (s *Store) OpenCursors() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cursors)
}
func (s *Store) Read(ctx context.Context, c ports.Cursor) (ports.EventPage, error) {
	if err := ctx.Err(); err != nil {
		return ports.EventPage{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.cursors[c.ID]
	if !ok || state.cursor.PrincipalHash != c.PrincipalHash || state.cursor.WorkspaceID != c.WorkspaceID {
		return ports.EventPage{}, ErrCursorBinding
	}
	if s.clock.Now().Unix() >= state.cursor.ExpiresAt {
		delete(s.cursors, c.ID)
		return ports.EventPage{}, ErrCursorExpired
	}
	if len(s.events) > 0 && state.next < len(s.events) && s.events[state.next].OccurredAt < s.clock.Now().Add(-s.retention).Unix() {
		return ports.EventPage{RetentionGap: true}, ErrCursorExpired
	}
	page := ports.EventPage{}
	for state.next < len(s.events) && len(page.Events) < 100 {
		e := s.events[state.next]
		state.next++
		if e.WorkspaceID == c.WorkspaceID {
			page.Events = append(page.Events, e)
		}
	}
	state.cursor.ExpiresAt = s.clock.Now().Add(5 * time.Minute).Unix()
	s.cursors[c.ID] = state
	page.Next = state.cursor
	return page, nil
}
func (s *Store) Purge() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(s.clock.Now())
	cutoff := s.clock.Now().Add(-s.retention).Unix()
	i := 0
	for i < len(s.events) && s.events[i].OccurredAt < cutoff {
		i++
	}
	if i == 0 {
		return 0
	}
	s.events = append([]ports.Event(nil), s.events[i:]...)
	for id, state := range s.cursors {
		state.next -= i
		if state.next < 0 {
			state.next = 0
		}
		state.oldest = s.clock.Now()
		s.cursors[id] = state
	}
	return i
}
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("events: random cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func principalHash(p ports.Principal) string { return ports.PrincipalHash(p) }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

var _ ports.EventStore = (*Store)(nil)
