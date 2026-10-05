package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/resolution"
	"github.com/sirerun/gist/hosted/internal/storage"
)

type resolutionCatalogProbe struct {
	record ports.CatalogRecord
	err    error
}

func (s resolutionCatalogProbe) Get(context.Context, ports.ArtifactRef) (ports.CatalogRecord, error) {
	return s.record, s.err
}
func (resolutionCatalogProbe) Search(context.Context, ports.SearchQuery) (ports.SearchPage, error) {
	return ports.SearchPage{}, nil
}

func TestResolutionCatalogNormalizesActualStorageErrors(t *testing.T) {
	dbFailure := errors.New("database unavailable")
	for _, tc := range []struct {
		name      string
		state     string
		err, want error
	}{
		{"missing", "", storage.ErrNotFound, resolution.ErrArtifactNotFound},
		{"wrapped missing", "", fmt.Errorf("lookup: %w", storage.ErrNotFound), resolution.ErrArtifactNotFound},
		{"revoked error", "", storage.ErrRevoked, resolution.ErrArtifactRevoked},
		{"revoked state", "revoked", nil, resolution.ErrArtifactRevoked},
		{"unavailable", "", dbFailure, dbFailure},
		{"published", "published", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := unrevokedCatalog{CatalogStore: resolutionCatalogProbe{record: ports.CatalogRecord{State: tc.state}, err: tc.err}}
			record, err := s.Get(context.Background(), ports.ArtifactRef{WorkspaceID: "workspace", Kind: ports.KindSkill, ID: "skill", Version: "1"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("error %v; want %v", err, tc.want)
			}
			if tc.want == resolution.ErrArtifactNotFound || tc.want == resolution.ErrArtifactRevoked {
				if record.State != "" || len(record.Metadata) != 0 {
					t.Fatal("denied catalog result exposed metadata")
				}
			}
			if tc.want == nil && record.State != "published" {
				t.Fatal("published record lost")
			}
		})
	}
}
