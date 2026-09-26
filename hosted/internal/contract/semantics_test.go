package contract

import "testing"

func TestCompileRequiresSecurityMetadata(t *testing.T) {
	_, _, err := Compile(ContractInput{Capability: CapabilityContract{ID: "gist/a", Version: "1", InputSchema: map[string]any{}, OutputSchema: map[string]any{}}, Dialect: "https://json-schema.org/draft/2020-12/schema"})
	if err == nil {
		t.Fatal("incomplete contract compiled")
	}
}
func TestConversionLossPolicy(t *testing.T) {
	r, err := ApplyConversionLoss(LossReport{}, Loss{Path: "/description", Reason: "unsupported", Target: "draft", Security: false})
	if err != nil || len(r.Losses) != 1 {
		t.Fatal("documentation loss should be recorded")
	}
	_, err = ApplyConversionLoss(r, Loss{Path: "/required", Reason: "unsupported", Target: "draft", Security: true})
	if err == nil {
		t.Fatal("security loss must fail")
	}
}
