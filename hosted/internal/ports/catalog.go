package ports

import (
	"context"
	"io"
)

type ArtifactKind string

const (
	KindSkill      ArtifactKind = "skill"
	KindCapability ArtifactKind = "capability"
	KindTool       ArtifactKind = "tool"
	KindProvider   ArtifactKind = "provider"
	KindBinding    ArtifactKind = "binding"
	KindTaxonomy   ArtifactKind = "taxonomy"
	KindConnection ArtifactKind = "connection"
	KindResolution ArtifactKind = "resolution"
)

type ArtifactRef struct {
	WorkspaceID string
	Kind        ArtifactKind
	ID          string
	Version     string
}

type Digest struct{ Algorithm, Value string }

type CatalogRecord struct {
	Ref            ArtifactRef
	State          string
	Digest         Digest
	ManifestDigest Digest
	Metadata       []byte
}

type SearchQuery struct {
	Principal Principal
	Text      string
	Kinds     []ArtifactKind
	Tags      []string
	Cursor    Cursor
	Limit     int
	MaxBytes  int
}

type SearchPage struct {
	Records []CatalogRecord
	Next    Cursor
}

type CatalogStore interface {
	Get(context.Context, ArtifactRef) (CatalogRecord, error)
	Search(context.Context, SearchQuery) (SearchPage, error)
}

type ArtifactReader interface{ io.ReadSeekCloser }

type ArtifactStore interface {
	Put(context.Context, Digest, io.Reader, int64) error
	Open(context.Context, ArtifactRef) (ArtifactReader, error)
}
