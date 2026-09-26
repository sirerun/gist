package resolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirerun/gist/hosted/internal/ports"
)

var ErrServiceUnavailable = errors.New("connection broker unavailable")

type ConnectionRequest struct {
	Principal  ports.Principal
	Capability ports.ArtifactRef
}
type ConnectionService struct{ broker ports.ConnectionInitiator }

func NewConnectionService(broker ports.ConnectionInitiator) *ConnectionService {
	return &ConnectionService{broker: broker}
}

func (s *ConnectionService) Begin(ctx context.Context, req ConnectionRequest) (ports.Connection, error) {
	if s == nil || s.broker == nil {
		return ports.Connection{}, fmt.Errorf("begin connection: %w", ErrServiceUnavailable)
	}
	if req.Capability.ID == "" || req.Capability.Version == "" {
		return ports.Connection{}, fmt.Errorf("begin connection: invalid capability reference")
	}
	return s.broker.Begin(ctx, req.Principal, req.Capability)
}

// Initiate is the explicit runtime-less connection path. It is never called
// as a side effect of resolution, which prevents a second consent flow for a
// runtime that owns its own connection lifecycle.
func (s *ConnectionService) Initiate(ctx context.Context, req ConnectionRequest) (ports.Connection, error) {
	return s.Begin(ctx, req)
}

func (s *ConnectionService) Get(ctx context.Context, principal ports.Principal, id string) (ports.Connection, error) {
	if s == nil || s.broker == nil {
		return ports.Connection{}, fmt.Errorf("get connection: %w", ErrServiceUnavailable)
	}
	if id == "" {
		return ports.Connection{}, fmt.Errorf("get connection: invalid id")
	}
	return s.broker.Get(ctx, principal, id)
}
