package ports

import "context"

type ConnectionStatus string

const (
	ConnectionPending   ConnectionStatus = "pending"
	ConnectionConnected ConnectionStatus = "connected"
	ConnectionFailed    ConnectionStatus = "failed"
	ConnectionCanceled  ConnectionStatus = "canceled"
)

type Connection struct {
	ID         string
	Principal  Principal
	Capability ArtifactRef
	Status     ConnectionStatus
	ConnectURL string
	CreatedAt  int64
	UpdatedAt  int64
}

type ConnectionInitiator interface {
	Begin(context.Context, Principal, ArtifactRef) (Connection, error)
	Get(context.Context, Principal, string) (Connection, error)
}
