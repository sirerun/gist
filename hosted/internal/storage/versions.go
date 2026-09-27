package storage

import (
	"errors"
	"sort"
	"sync"

	"github.com/sirerun/gist/hosted/internal/ports"
)

type Version struct {
	Ref       ports.ArtifactRef
	State     string
	Digest    ports.Digest
	CreatedAt int64
	RevokedAt int64
}
type VersionStore struct {
	mu       sync.RWMutex
	versions map[string][]Version
}

func NewVersionStore() *VersionStore { return &VersionStore{versions: make(map[string][]Version)} }
func (s *VersionStore) Add(v Version) error {
	if v.Ref.WorkspaceID == "" || v.Ref.ID == "" || v.Ref.Version == "" {
		return errors.New("versions: incomplete reference")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := v.Ref.WorkspaceID + "/" + string(v.Ref.Kind) + "/" + v.Ref.ID
	for _, existing := range s.versions[key] {
		if existing.Ref.Version == v.Ref.Version {
			return ErrConflict
		}
	}
	s.versions[key] = append(s.versions[key], v)
	list := s.versions[key]
	sort.SliceStable(list, func(i, j int) bool { return compareSemver(list[i].Ref.Version, list[j].Ref.Version) < 0 })
	return nil
}
func (s *VersionStore) List(workspace string, kind ports.ArtifactKind, id string) []Version {
	s.mu.RLock()
	defer s.mu.RUnlock()
	in := s.versions[workspace+"/"+string(kind)+"/"+id]
	return append([]Version(nil), in...)
}
