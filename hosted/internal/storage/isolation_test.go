package storage

import (
	"context"
	"testing"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func TestVersionStoreDoesNotOverwriteAnotherVersion(t *testing.T) {
	s := NewVersionStore()
	v := Version{Ref: ports.ArtifactRef{WorkspaceID: "ws-a", Kind: ports.KindSkill, ID: "skill", Version: "1.0.0"}, State: "published"}
	if err := s.Add(v); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(v); err != ErrConflict {
		t.Fatalf("got %v", err)
	}
	if got := len(s.List("ws-a", ports.KindSkill, "skill")); got != 1 {
		t.Fatalf("versions=%d", got)
	}
}
func TestTenantTransactionRejectsMissingWorkspace(t *testing.T) {
	if err := WithTenant(context.Background(), nil, "", nil); err == nil {
		t.Fatal("expected invalid tenant transaction")
	}
	if err := validateRef(ports.ArtifactRef{WorkspaceID: "ws-a", Kind: ports.KindSkill, ID: "s", Version: "1"}); err != nil {
		t.Fatal(err)
	}
}
