package ports

import (
	"context"
	"io"
	"testing"
)

type compilePortProbe struct{}

func (compilePortProbe) Get(context.Context, ArtifactRef) (CatalogRecord, error) {
	return CatalogRecord{}, nil
}
func (compilePortProbe) Search(context.Context, SearchQuery) (SearchPage, error) {
	return SearchPage{}, nil
}
func (compilePortProbe) Put(context.Context, Digest, io.Reader, int64) error       { return nil }
func (compilePortProbe) Open(context.Context, ArtifactRef) (ArtifactReader, error) { return nil, nil }

func TestPortsExposeConsumerOwnedBoundaries(t *testing.T) {
	var _ CatalogStore = compilePortProbe{}
	var _ ArtifactStore = compilePortProbe{}
	var _ LexicalSearcher = compilePortProbe{}
	if (ArtifactRef{WorkspaceID: "workspace-a", Kind: KindSkill, ID: "sk_1", Version: "1.0.0"}).WorkspaceID == "" {
		t.Fatal("artifact reference lost workspace boundary")
	}
	if EventVersionRevoked == EventVersionPublished {
		t.Fatal("event types must remain distinct")
	}
}
