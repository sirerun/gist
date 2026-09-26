package storage

import (
	"errors"
	"sync"
)

type TaxonomyEdition struct {
	WorkspaceID, ID, Version string
	Attribution              map[string]string
	License                  string
	Nodes                    []TaxonomyNode
}
type TaxonomyNode struct {
	ID, ParentID, Label, APQCRef string
	Level                        int
}
type TaxonomyStore struct {
	mu       sync.RWMutex
	editions map[string]TaxonomyEdition
}

func NewTaxonomyStore() *TaxonomyStore {
	return &TaxonomyStore{editions: make(map[string]TaxonomyEdition)}
}
func (s *TaxonomyStore) Put(e TaxonomyEdition) error {
	if e.WorkspaceID == "" || e.ID == "" || e.Version == "" {
		return errors.New("taxonomy: incomplete edition")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := e.WorkspaceID + "/" + e.ID + "/" + e.Version
	if _, ok := s.editions[key]; ok {
		return ErrConflict
	}
	e.Nodes = append([]TaxonomyNode(nil), e.Nodes...)
	s.editions[key] = e
	return nil
}
func (s *TaxonomyStore) Get(workspace, id, version string) (TaxonomyEdition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.editions[workspace+"/"+id+"/"+version]
	if !ok {
		return TaxonomyEdition{}, ErrNotFound
	}
	e.Nodes = append([]TaxonomyNode(nil), e.Nodes...)
	return e, nil
}
