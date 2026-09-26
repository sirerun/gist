package publication

import (
	"errors"
	"fmt"
	"sync"

	"github.com/sirerun/gist/hosted/internal/packages"
)

var ErrVersionConflict = errors.New("publication: immutable version conflict")

type Published struct {
	WorkspaceID, ID, Version, Digest string
	Package                          packages.Package
	Review                           Review
	Revoked                          bool
}
type Catalog struct {
	mu      sync.RWMutex
	records map[string]Published
}

func NewCatalog() *Catalog { return &Catalog{records: make(map[string]Published)} }
func (c *Catalog) Publish(workspace string, p packages.Package, review Review) (Published, error) {
	if workspace == "" {
		return Published{}, errors.New("publication: workspace is required")
	}
	if err := review.Validate(); err != nil {
		return Published{}, err
	}
	if review.Decision != Approved {
		return Published{}, errors.New("publication: only approved packages can be published")
	}
	if p.Manifest.Publication.Provenance == "" || p.Manifest.Trust != "operator_asserted" {
		return Published{}, errors.New("publication: trust ceiling rejected")
	}
	key := workspace + "/" + p.Manifest.ID + "/" + p.Manifest.Version
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.records[key]; exists {
		return Published{}, fmt.Errorf("%w: %s", ErrVersionConflict, key)
	}
	v := Published{WorkspaceID: workspace, ID: p.Manifest.ID, Version: p.Manifest.Version, Digest: p.PackageDigest, Package: p, Review: review}
	c.records[key] = v
	return v, nil
}
func (c *Catalog) Get(workspace, id, version string) (Published, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.records[workspace+"/"+id+"/"+version]
	if !ok {
		return Published{}, errors.New("publication: not found")
	}
	return v, nil
}
func (c *Catalog) Revoke(workspace, id, version string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := workspace + "/" + id + "/" + version
	v, ok := c.records[key]
	if !ok {
		return errors.New("publication: not found")
	}
	v.Revoked = true
	c.records[key] = v
	return nil
}
