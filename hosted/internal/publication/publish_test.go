package publication

import (
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/packages"
)

func TestReviewRequiresApprovalBeforePublication(t *testing.T) {
	c := NewCatalog()
	p := packages.Package{Manifest: packages.Manifest{ID: "acme/skill", Version: "1.0.0", Trust: "operator_asserted", Publication: struct {
		Status     string `json:"status"`
		Provenance string `json:"provenance"`
	}{Status: "approved", Provenance: "publisher_asserted"}}, PackageDigest: "digest"}
	r := Review{ReviewerID: "r", Decision: Held, At: time.Now().UTC()}
	if _, err := c.Publish("ws", p, r); err == nil {
		t.Fatal("held review was published")
	}
}
