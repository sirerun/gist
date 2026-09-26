package events

import (
	"context"
	"errors"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type Feed struct{ store *Store }

func NewFeed(store *Store) (*Feed, error) {
	if store == nil {
		return nil, errors.New("events: nil store")
	}
	return &Feed{store: store}, nil
}
func (f *Feed) Read(ctx context.Context, principal ports.Principal, cursor ports.Cursor) (ports.EventPage, error) {
	if cursor.PrincipalHash != principalHash(principal) || cursor.WorkspaceID != principal.WorkspaceID {
		return ports.EventPage{}, ErrCursorBinding
	}
	return f.store.Read(ctx, cursor)
}
