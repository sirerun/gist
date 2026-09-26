package contract

import "testing"

func TestWireReadyRequiresEveryRequiredFinding(t *testing.T) {
	if err := ValidateResolution(ResolutionWire{Aggregate: FindingReady, Required: []FindingWire{{CapabilityID: "a", Required: true, Status: FindingReady}, {CapabilityID: "b", Required: true, Status: FindingIncomplete}}}); err == nil {
		t.Fatal("partial ready resolution accepted")
	}
}
func TestWireSupportsGatewayFinding(t *testing.T) {
	if err := ValidateResolution(ResolutionWire{Aggregate: FindingRequiresGateway, Required: []FindingWire{{CapabilityID: "a", Required: true, Status: FindingRequiresGateway}}}); err != nil {
		t.Fatal(err)
	}
}
func TestErrorStatuses(t *testing.T) {
	for _, c := range []ErrorCode{Unauthorized, Forbidden, NotFound, VersionConflict, ArtifactRevoked, CursorExpired, ResolutionExpired, BudgetExceeded, ValidationFailed, IntegrityError, RateLimited, ServiceUnavailable} {
		if _, ok := HTTPStatus(c); !ok {
			t.Fatalf("missing %s", c)
		}
	}
}
