package publication

import (
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/packages"
)

func TestPublicationIsImmutableAndReviewFindingsBlockVisibility(t *testing.T) {
	c := NewCatalog()
	p := packages.Package{Manifest: packages.Manifest{ID: "acme/skill", Version: "1.0.0", Trust: "operator_asserted", Publication: struct {
		Status     string `json:"status"`
		Provenance string `json:"provenance"`
	}{Status: "approved", Provenance: "publisher_asserted"}}, PackageDigest: "digest"}
	r := Review{ReviewerID: "reviewer", Decision: Approved, Evidence: map[string]string{"screen": "pass"}, At: time.Now().UTC()}
	if _, err := c.Publish("ws-a", p, r); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Publish("ws-a", p, r); err == nil {
		t.Fatal("expected immutable conflict")
	}
	if _, err := c.Get("ws-b", p.Manifest.ID, p.Manifest.Version); err == nil {
		t.Fatal("cross-workspace lookup returned a record")
	}
}
