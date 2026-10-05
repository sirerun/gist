package resolution

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRequiredResolutionDoesNotPersistWhenBudgetIsTooSmall(t *testing.T) {
	store := &resolutionDouble{}
	req := request()
	req.MaxBytes = 8
	r, err := NewResolver(fixture("2.3.4"), policyDouble{allowed: true}, store, clockDouble{now: time.Unix(100, 0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Resolve(context.Background(), req)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("expected enforced budget failure, got %v", err)
	}
	if store.value.ID != "" {
		t.Fatal("over-budget required resolution must not be persisted")
	}
}

func TestPrivateCapabilityDenialDoesNotResolve(t *testing.T) {
	c := fixture("2.3.4")
	resolver, err := NewResolver(c, policyDouble{allowed: true, deniedID: "cap/demo"}, &resolutionDouble{}, clockDouble{now: time.Unix(100, 0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolver.Resolve(context.Background(), request())
	if err == nil {
		t.Fatal("private capability was resolved")
	}
}
