package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/sirerun/gist/catalog/ports"
	"github.com/sirerun/gist/catalog/preparation"
	"github.com/sirerun/gist/catalog/skills/inboxbriefing"
	"github.com/sirerun/gist/tools/evaluation/fixture"
)

const policySchema = "gist.evaluation.task-policy/v1"
const maxPolicyBytes = 1 << 20
const defaultPolicyIdentity = "explicit-source-task-admission-v1"

// Evaluation policies are caller-supplied synthetic fixtures, never live authority.
type taskPolicy struct {
	Schema   string        `json:"schema"`
	Kind     string        `json:"kind"`
	Entries  []policyEntry `json:"entries"`
	identity string
}
type policyEntry struct {
	IntentSHA256         string              `json:"intent_sha256"`
	Principal            ports.Principal     `json:"principal"`
	RuntimeID            string              `json:"runtime_id"`
	Skill                ports.ArtifactPin   `json:"skill"`
	RequiredCapabilities []ports.ArtifactRef `json:"required_capabilities"`
	RecipeVersion        string              `json:"recipe_version"`
}

func loadPolicy(path, expected string) (*taskPolicy, error) {
	if !digestString(expected) {
		return nil, errors.New("external task policy requires exact SHA256")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxPolicyBytes {
		return nil, errors.New("task policy must be a bounded regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("task policy file changed before snapshot")
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxPolicyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxPolicyBytes || fixture.Hash(raw).Value != expected {
		return nil, errors.New("task policy digest/size mismatch")
	}
	return decodePolicy(raw, expected)
}
func decodePolicy(raw []byte, expected string) (*taskPolicy, error) {
	if len(raw) > maxPolicyBytes || fixture.Hash(raw).Value != expected {
		return nil, errors.New("task policy digest/size mismatch")
	}
	if err := rejectDuplicateFields(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var p taskPolicy
	if err := d.Decode(&p); err != nil {
		return nil, err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return nil, errors.New("trailing policy JSON")
	}
	if p.Schema != policySchema || p.Kind != "evaluation_fixture_not_live_authority" || len(p.Entries) == 0 || len(p.Entries) > 1000 {
		return nil, errors.New("invalid evaluation task policy schema/kind/budget")
	}
	c, err := fixture.New("ready")
	if err != nil {
		return nil, err
	}
	r := c.Records[fixture.Ref(ports.KindSkill, inboxbriefing.SkillID)]
	pin := ports.ArtifactPin{Ref: r.Ref, Digest: r.Digest, ManifestDigest: r.ManifestDigest}
	caps := []ports.ArtifactRef{fixture.Ref(ports.KindCapability, inboxbriefing.CapabilityID)}
	seen := map[string]bool{}
	for _, e := range p.Entries {
		if !digestString(e.IntentSHA256) || !reflect.DeepEqual(e.Principal, fixture.Principal) || e.RuntimeID != fixture.Request("").RuntimeID || e.Skill != pin || !reflect.DeepEqual(e.RequiredCapabilities, caps) || e.RecipeVersion != preparation.RecipeVersion || seen[e.IntentSHA256] {
			return nil, errors.New("invalid/duplicate task policy scope: exact maintained synthetic caller contract required")
		}
		seen[e.IntentSHA256] = true
	}
	p.identity = "sha256:" + expected
	return &p, nil
}
func (p *taskPolicy) AdmitTask(_ context.Context, q preparation.TaskAdmissionRequest) (preparation.TaskAdmissionDecision, error) {
	allowed := false
	for _, e := range p.Entries {
		if e.IntentSHA256 == fixture.Hash([]byte(q.Intent)).Value && reflect.DeepEqual(e.Principal, q.Principal) && e.RuntimeID == q.RuntimeID && e.Skill == q.Skill && reflect.DeepEqual(e.RequiredCapabilities, q.RequiredCapabilities) && e.RecipeVersion == q.RecipeVersion {
			allowed = true
			break
		}
	}
	now := time.Now().UTC()
	return preparation.TaskAdmissionDecision{ScopeDigest: q.ScopeDigest, Admitted: allowed, Generation: p.identity, CheckedAt: now, ValidUntil: now.Add(time.Minute)}, nil
}

// Export is a deliberately empty admission template: operators independently
// review intent hashes; no input queries or labels create positive entries.
func policyTemplate() (taskPolicy, error) {
	c, err := fixture.New("ready")
	if err != nil {
		return taskPolicy{}, err
	}
	r := c.Records[fixture.Ref(ports.KindSkill, inboxbriefing.SkillID)]
	return taskPolicy{Schema: policySchema, Kind: "evaluation_fixture_not_live_authority", Entries: []policyEntry{{IntentSHA256: "REPLACE_WITH_INDEPENDENTLY_REVIEWED_EXACT_INTENT_SHA256", Principal: fixture.Principal, RuntimeID: fixture.Request("").RuntimeID, Skill: ports.ArtifactPin{Ref: r.Ref, Digest: r.Digest, ManifestDigest: r.ManifestDigest}, RequiredCapabilities: []ports.ArtifactRef{fixture.Ref(ports.KindCapability, inboxbriefing.CapabilityID)}, RecipeVersion: preparation.RecipeVersion}}}, nil
}

// Encoding/json otherwise silently accepts duplicate object keys. A policy
// snapshot must have only one interpretation, including nested caller fields.
func rejectDuplicateFields(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate/invalid policy JSON field")
			}
			seen[name] = true
			if err := rejectDuplicateFields(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := rejectDuplicateFields(d); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid policy JSON delimiter")
	}
	_, err = d.Token()
	return err
}
