package packages

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type authDouble struct {
	allowed bool
	calls   int
}

func (a *authDouble) Decide(context.Context, ports.Principal, ports.Action, *ports.ArtifactRef) (ports.Decision, error) {
	a.calls++
	return ports.Decision{Allowed: a.allowed}, nil
}

type catalogDouble struct{ record ports.CatalogRecord }

func (c catalogDouble) Get(context.Context, ports.ArtifactRef) (ports.CatalogRecord, error) {
	return c.record, nil
}
func (c catalogDouble) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{}, nil
}

type artifactDouble struct{ value string }

func (a artifactDouble) Put(context.Context, ports.Digest, io.Reader, int64) error { return nil }
func (a artifactDouble) Open(context.Context, ports.ArtifactRef) (ports.ArtifactReader, error) {
	return &retrieveReader{Reader: strings.NewReader(a.value)}, nil
}

type retrieveReader struct{ *strings.Reader }

func (r *retrieveReader) Close() error { return nil }
func TestRetrieveAuthorizesBeforeOpening(t *testing.T) {
	a := &authDouble{}
	r := Retriever{Authorizer: a, Catalog: catalogDouble{record: ports.CatalogRecord{State: "published"}}, Artifacts: artifactDouble{value: "secret"}}
	_, err := r.Retrieve(context.Background(), ports.Principal{WorkspaceID: "ws"}, ports.ArtifactRef{WorkspaceID: "ws", Kind: ports.KindSkill, ID: "s", Version: "1"}, 10)
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("got %v", err)
	}
	if a.calls != 1 {
		t.Fatalf("calls=%d", a.calls)
	}
}
