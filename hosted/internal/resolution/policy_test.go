package resolution

import (
	"context"
	"testing"
	"time"

	"github.com/sirerun/gist/hosted/internal/ports"
)

func TestUnsupportedAndGatewayFindingsAreNotReady(t *testing.T) {
	base := fixture()
	base.records["binding:b@1.0.0"] = ports.CatalogRecord{Ref: ref(ports.KindBinding, "b", "1.0.0"), Metadata: metadata(map[string]any{"capability_ref": "gist/cap@1.0.0", "supported_runtimes": []string{"other"}})}
	r, _ := NewResolver(base, &policyDouble{allowed: true}, &resolutionDouble{}, clockDouble{now: baseTime()}, nil)
	got, err := r.Resolve(context.Background(), Request{Principal: ports.Principal{Subject: "p", WorkspaceID: "w"}, Skill: ref(ports.KindSkill, "s", "1"), RuntimeID: "client", SelectedBindings: map[string]string{"gist/cap@1.0.0": "b"}, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == StatusReady || got.Findings[0].Status != StatusUnsupportedRuntime {
		t.Fatalf("unexpected unsupported result: %+v", got)
	}
}

func TestRequiredResolutionDoesNotPersistWhenBudgetIsTooSmall(t *testing.T) {
	store := &resolutionDouble{}
	r, _ := NewResolver(fixture(), &policyDouble{allowed: true}, store, clockDouble{now: baseTime()}, nil)
	_, err := r.Resolve(context.Background(), Request{Principal: ports.Principal{Subject: "p", WorkspaceID: "w"}, Skill: ref(ports.KindSkill, "s", "1"), RuntimeID: "client", SelectedBindings: map[string]string{"gist/cap@1.0.0": "b"}, ConnectionAsserted: map[string]ConnectionAssertion{"gist/cap@1.0.0": {Status: "connected"}}, MaxBytes: 8})
	if err == nil {
		t.Fatal("expected enforced budget failure")
	}
	if store.value.ID != "" {
		t.Fatal("over-budget required resolution must not be persisted")
	}
}

func baseTime() (t time.Time) { return time.Unix(100, 0) }
