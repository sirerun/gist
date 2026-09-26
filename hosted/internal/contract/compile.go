package contract

import (
	"fmt"
	"time"
)

type Effects struct {
	VocabularyVersion string
	Class             string
	Sensitivity       string
}
type Digest struct {
	Algorithm string
	Value     string
}
type CapabilityContract struct {
	ID, Version               string
	InputSchema, OutputSchema map[string]any
	Effects                   Effects
	RequiredScopes            []string
	CostCurrency, CostUnit    string
	CostUpperBound            any
	TimeoutMS                 int
	RetrySupported            bool
	RetryMaxAttempts          int
	IdempotencySupported      bool
	AsyncSupported            bool
}
type ProviderBinding struct {
	ProviderID, ActionID, ActionVersion string
	Destinations                        []string
	CredentialType                      string
	CredentialScopes                    []string
	CaptureTime                         time.Time
	Digest                              Digest
	Conformance                         string
}
type ContractInput struct {
	Capability                                                             CapabilityContract
	Binding                                                                ProviderBinding
	ToolID, ToolVersion, Dialect, Provenance, ExecutionLocation, Lifecycle string
	PackageDigest, BindingDigest                                           Digest
}
type RuntimeContract struct {
	CapabilityID, CapabilityVersion, ToolID, ToolVersion, Dialect                     string
	InputSchema, OutputSchema                                                         map[string]any
	Effects                                                                           Effects
	Destinations, RequiredScopes, CredentialScopes                                    []string
	CredentialType                                                                    string
	CostCurrency, CostUnit                                                            string
	CostUpperBound                                                                    any
	TimeoutMS, RetryMaxAttempts                                                       int
	RetrySupported, IdempotencySupported, AsyncSupported                              bool
	ProviderActionID, ProviderActionVersion, ExecutionLocation, Lifecycle, Provenance string
	PackageDigest, BindingDigest, ToolDigest                                          Digest
	CaptureTime                                                                       time.Time
}
type Loss struct {
	Path, Target, Reason string
	Security             bool
}
type LossReport struct {
	Losses   []Loss
	Degraded bool
}

func Compile(input ContractInput) (RuntimeContract, LossReport, error) {
	c, b := input.Capability, input.Binding
	if c.ID == "" || c.Version == "" || input.ToolID == "" || input.ToolVersion == "" || input.Dialect == "" {
		return RuntimeContract{}, LossReport{}, fmt.Errorf("missing identity or schema dialect")
	}
	if input.Dialect != "https://json-schema.org/draft/2020-12/schema" {
		return RuntimeContract{}, LossReport{}, fmt.Errorf("unsupported schema dialect")
	}
	if c.InputSchema == nil || c.OutputSchema == nil {
		return RuntimeContract{}, LossReport{}, fmt.Errorf("input and output schemas are required")
	}
	if c.Effects.VocabularyVersion == "" || c.Effects.Class == "" || c.Effects.Sensitivity == "" {
		return RuntimeContract{}, LossReport{}, fmt.Errorf("effects metadata is required")
	}
	if len(c.RequiredScopes) == 0 || b.CredentialType == "" || len(b.Destinations) == 0 || b.ActionID == "" || b.ActionVersion == "" || c.TimeoutMS <= 0 || c.CostCurrency == "" || c.CostUnit == "" || c.CostUpperBound == nil {
		return RuntimeContract{}, LossReport{}, fmt.Errorf("security and execution metadata is incomplete")
	}
	if input.ExecutionLocation != "client" && input.ExecutionLocation != "runtime" && input.ExecutionLocation != "gist_gateway" {
		return RuntimeContract{}, LossReport{}, fmt.Errorf("invalid execution location")
	}
	if input.Lifecycle == "" || input.Provenance == "" || b.Conformance != "passed" {
		return RuntimeContract{}, LossReport{}, fmt.Errorf("lifecycle, provenance, and conformance are required")
	}
	return RuntimeContract{CapabilityID: c.ID, CapabilityVersion: c.Version, ToolID: input.ToolID, ToolVersion: input.ToolVersion, Dialect: input.Dialect, InputSchema: c.InputSchema, OutputSchema: c.OutputSchema, Effects: c.Effects, Destinations: b.Destinations, RequiredScopes: c.RequiredScopes, CredentialScopes: b.CredentialScopes, CredentialType: b.CredentialType, CostCurrency: c.CostCurrency, CostUnit: c.CostUnit, CostUpperBound: c.CostUpperBound, TimeoutMS: c.TimeoutMS, RetryMaxAttempts: c.RetryMaxAttempts, RetrySupported: c.RetrySupported, IdempotencySupported: c.IdempotencySupported, AsyncSupported: c.AsyncSupported, ProviderActionID: b.ActionID, ProviderActionVersion: b.ActionVersion, ExecutionLocation: input.ExecutionLocation, Lifecycle: input.Lifecycle, Provenance: input.Provenance, PackageDigest: input.PackageDigest, BindingDigest: input.BindingDigest, ToolDigest: b.Digest, CaptureTime: b.CaptureTime}, LossReport{}, nil
}

func ApplyConversionLoss(report LossReport, loss Loss) (LossReport, error) {
	report.Losses = append(report.Losses, loss)
	if loss.Security {
		report.Degraded = true
		return report, fmt.Errorf("security-semantic schema loss at %s", loss.Path)
	}
	return report, nil
}
