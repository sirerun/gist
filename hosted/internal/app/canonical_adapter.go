package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sirerun/gist/hosted/internal/ports"
	"github.com/sirerun/gist/hosted/internal/resolution"
	"github.com/sirerun/gist/hosted/internal/rest"
	"github.com/sirerun/gist/hosted/internal/storage"
)

type canonicalResolverAdapter struct {
	resolver *resolution.Resolver
	maxBytes int
}

// NewCanonicalResolver creates the strict v1 wire adapter for the shared REST
// and MCP resolver interface. The principal is supplied by the authenticated
// transport, never by request JSON.
func NewCanonicalResolver(resolver *resolution.Resolver, maxBytes int) (rest.Resolver, error) {
	if resolver == nil || maxBytes <= 0 {
		return nil, fmt.Errorf("canonical resolver requires a resolver and positive response limit")
	}
	return canonicalResolverAdapter{resolver: resolver, maxBytes: maxBytes}, nil
}

type canonicalResolveRequest struct {
	SkillRef string            `json:"skill_ref"`
	Runtime  *canonicalRuntime `json:"runtime"`
	MaxBytes *int              `json:"max_bytes"`
}
type canonicalRuntime struct {
	ID               string          `json:"id"`
	OwnedConnections json.RawMessage `json:"owned_connections,omitempty"`
}

func (a canonicalResolverAdapter) Resolve(ctx context.Context, principal ports.Principal, raw []byte) ([]byte, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return nil, rest.ErrValidationFailed
	}
	if err := rejectNonCanonicalRequestFields(raw); err != nil {
		return nil, rest.ErrValidationFailed
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var input canonicalResolveRequest
	if err := decoder.Decode(&input); err != nil {
		return nil, rest.ErrValidationFailed
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, rest.ErrValidationFailed
	}
	if input.MaxBytes == nil || *input.MaxBytes <= 0 || input.Runtime == nil || !validRuntimeID(input.Runtime.ID) {
		return nil, rest.ErrValidationFailed
	}
	id, version, ok := strings.Cut(input.SkillRef, "@")
	if !ok || !validArtifactID(id) || version == "" || strings.Contains(version, "@") || strings.Contains(version, "/") || strings.TrimSpace(version) != version {
		return nil, rest.ErrValidationFailed
	}
	if principal.WorkspaceID == "" || principal.Subject == "" {
		return nil, rest.ErrValidationFailed
	}
	budget := *input.MaxBytes
	if budget > a.maxBytes {
		budget = a.maxBytes
	}
	owned := false
	if len(input.Runtime.OwnedConnections) > 0 {
		var assertion *bool
		if err := json.Unmarshal(input.Runtime.OwnedConnections, &assertion); err != nil || assertion == nil {
			return nil, rest.ErrValidationFailed
		}
		owned = *assertion
	}
	result, err := a.resolver.Resolve(ctx, resolution.Request{
		Principal: principal,
		Skill:     ports.ArtifactRef{WorkspaceID: principal.WorkspaceID, Kind: ports.KindSkill, ID: id, Version: version},
		RuntimeID: input.Runtime.ID, OwnedConnections: owned, MaxBytes: budget,
	})
	if err != nil {
		if errors.Is(err, resolution.ErrBudgetExceeded) {
			return nil, rest.ErrBudgetExceeded
		}
		if errors.Is(err, resolution.ErrArtifactRevoked) || errors.Is(err, storage.ErrRevoked) {
			return nil, rest.ErrArtifactRevoked
		}
		if errors.Is(err, resolution.ErrArtifactNotFound) || errors.Is(err, resolution.ErrAccessDenied) {
			return nil, rest.ErrNotFound
		}
		return nil, err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode canonical resolution response: %w", err)
	}
	if len(encoded) > budget {
		return nil, rest.ErrBudgetExceeded
	}
	return encoded, nil
}

func rejectNonCanonicalRequestFields(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if err := rejectFieldCaseVariants(fields, "skill_ref", "runtime", "max_bytes"); err != nil {
		return err
	}
	if runtimeRaw, ok := fields["runtime"]; ok {
		var runtimeFields map[string]json.RawMessage
		if err := json.Unmarshal(runtimeRaw, &runtimeFields); err != nil {
			return err
		}
		if err := rejectFieldCaseVariants(runtimeFields, "id", "owned_connections"); err != nil {
			return err
		}
	}
	return nil
}

func rejectFieldCaseVariants(fields map[string]json.RawMessage, canonical ...string) error {
	for key := range fields {
		for _, name := range canonical {
			if strings.EqualFold(key, name) && key != name {
				return fmt.Errorf("noncanonical request field %q", key)
			}
		}
	}
	return nil
}

func validArtifactID(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if i == 0 && !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || strings.ContainsRune("._/-", r)) {
			return false
		}
	}
	return true
}

func validRuntimeID(value string) bool {
	if value == "" || len(value) > 256 || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func rejectDuplicateJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("malformed JSON object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("malformed JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}
