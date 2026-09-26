package packages

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/sirerun/gist/hosted/internal/ports"
)

var ErrDenied = errors.New("packages: retrieval denied")
var ErrBudgetExceeded = errors.New("packages: retrieval budget exceeded")

type Envelope struct {
	Record ports.CatalogRecord
	Bytes  []byte
	Size   int64
}
type Retriever struct {
	Authorizer ports.Authorizer
	Catalog    ports.CatalogStore
	Artifacts  ports.ArtifactStore
}

func (r Retriever) Retrieve(ctx context.Context, principal ports.Principal, ref ports.ArtifactRef, maxBytes int64) (Envelope, error) {
	if r.Authorizer == nil || r.Catalog == nil || r.Artifacts == nil {
		return Envelope{}, errors.New("packages: incomplete retrieval service")
	}
	decision, err := r.Authorizer.Decide(ctx, principal, ports.ActionRead, &ref)
	if err != nil {
		return Envelope{}, fmt.Errorf("packages: authorize retrieval: %w", err)
	}
	if !decision.Allowed {
		return Envelope{}, ErrDenied
	}
	record, err := r.Catalog.Get(ctx, ref)
	if err != nil {
		return Envelope{}, fmt.Errorf("packages: load record: %w", err)
	}
	if record.State == "revoked" {
		return Envelope{}, errors.New("packages: artifact revoked")
	}
	reader, err := r.Artifacts.Open(ctx, ref)
	if err != nil {
		return Envelope{}, fmt.Errorf("packages: open artifact: %w", err)
	}
	defer reader.Close()
	if maxBytes <= 0 {
		maxBytes = 64 << 20
	}
	b, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return Envelope{}, fmt.Errorf("packages: read artifact: %w", err)
	}
	if int64(len(b)) > maxBytes {
		return Envelope{}, ErrBudgetExceeded
	}
	return Envelope{Record: record, Bytes: b, Size: int64(len(b))}, nil
}
