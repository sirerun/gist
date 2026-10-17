package composition

import (
	"errors"
	"fmt"
	"strings"
)

func validLimits(l Limits) bool {
	return l.MaxSteps > 0 && l.MaxSteps <= 64 && l.MaxIterations > 0 && l.MaxIterations <= 20 && l.MaxCalls > 0 && l.MaxCalls <= 128 && l.MaxInputBytes > 0 && l.MaxInputBytes <= 1<<20 && l.MaxOutputBytes > 0 && l.MaxOutputBytes <= 4<<20 && l.MaxGuideBytes > 0 && l.MaxGuideBytes <= 64<<10
}
func Validate(d Document, contracts map[string]Contract) error {
	bad := func(s string) error { return fmt.Errorf("composition: %s", s) }
	if d.SchemaVersion != Version || !identifier(d.SkillID, true) || !identifier(d.SkillVersion, false) {
		return bad("unsupported version or skill identity")
	}
	if !validLimits(d.Limits) || len(d.Requirements) < 1 || len(d.Requirements) > 64 || len(d.Steps) < 1 || len(d.Steps) > d.Limits.MaxSteps || len(d.ConnectionGroups) > 32 {
		return bad("structure/limit bounds")
	}
	n := 0
	if e := schemaValid(d.TaskSchema, 0, &n); e != nil {
		return e
	}
	if d.TaskSchema.Type != "object" {
		return bad("task must be an object")
	}
	reqs := map[string]Requirement{}
	capIDs := map[string]bool{}
	for _, r := range d.Requirements {
		id := r.CapabilityID + "@" + r.ContractVersion
		if !identifier(r.Key, false) || !identifier(r.CapabilityID, true) || !identifier(r.ContractVersion, false) || !digestValid(r.ContractDigest) || reqs[r.Key].Key != "" || capIDs[id] {
			return bad("duplicate or invalid requirement")
		}
		c, ok := contracts[r.Key]
		if !ok || c.ID != r.CapabilityID || c.Version != r.ContractVersion || hash(c.Bytes) != r.ContractDigest {
			return bad("contract identity or digest mismatch")
		}
		checked, e := ParseContract(c.Bytes)
		if e != nil {
			return e
		}
		if !equal(checked, c) {
			return bad("contract schema differs from pinned bytes")
		}
		reqs[r.Key] = r
		capIDs[id] = true
	}
	grouped := map[string]bool{}
	groups := map[string]bool{}
	for _, g := range d.ConnectionGroups {
		if !identifier(g.ID, false) || groups[g.ID] || len(g.Requirements) < 2 || len(g.Requirements) > 64 {
			return bad("invalid connection group")
		}
		groups[g.ID] = true
		for _, key := range g.Requirements {
			if _, ok := reqs[key]; !ok || grouped[key] {
				return bad("missing or overlapping group requirement")
			}
			grouped[key] = true
		}
	}
	steps := map[string]Step{}
	used := map[string]bool{}
	for _, s := range d.Steps {
		if !identifier(s.ID, false) || s.ID == "$item" || steps[s.ID].ID != "" || len(s.After) > 64 || len(s.Inputs) > 64 {
			return bad("invalid or duplicate step")
		}
		if _, ok := reqs[s.Requirement]; !ok {
			return bad("unknown step requirement")
		}
		steps[s.ID] = s
		used[s.Requirement] = true
	}
	for key := range reqs {
		if !used[key] {
			return bad("unused requirement")
		}
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return bad("cycle")
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		seen := map[string]bool{}
		s := steps[id]
		for _, p := range s.After {
			ps, ok := steps[p]
			if !ok || seen[p] {
				return bad("unknown or duplicate predecessor")
			}
			seen[p] = true
			if reqs[ps.Requirement].Optional && !reqs[s.Requirement].Optional {
				return bad("required step depends on optional step")
			}
			if e := visit(p); e != nil {
				return e
			}
		}
		state[id] = 2
		return nil
	}
	calls, iterations := 0, 0
	for _, s := range d.Steps {
		if e := visit(s.ID); e != nil {
			return e
		}
		c := contracts[s.Requirement]
		count := 1
		var item *Schema
		source := func(step, p string) (Schema, error) {
			if step == "" {
				return at(d.TaskSchema, p)
			}
			if !contains(s.After, step) {
				return Schema{}, bad("source not a predecessor")
			}
			return at(contracts[steps[step].Requirement].Output, p)
		}
		if s.Foreach != nil {
			f := s.Foreach
			if f.Step == "" || f.Max < 1 || f.Max > d.Limits.MaxIterations {
				return bad("invalid foreach")
			}
			arr, e := source(f.Step, f.Pointer)
			if e != nil {
				return e
			}
			if arr.Type != "array" || arr.MaxItems > f.Max {
				return bad("foreach bound cannot cover source array")
			}
			if steps[f.Step].Foreach != nil {
				return bad("nested/aggregated iteration source unsupported")
			}
			item = arr.Items
			count = f.Max
			iterations += count
		}
		calls += count
		for _, k := range c.Input.Required {
			if _, ok := s.Inputs[k]; !ok {
				return bad("missing required input")
			}
		}
		for k, m := range s.Inputs {
			dst, ok := c.Input.Properties[k]
			if !ok {
				return bad("unknown input property")
			}
			var src Schema
			var e error
			if m.Step == "$item" {
				if item == nil {
					return bad("item outside iteration")
				}
				src, e = at(*item, m.Pointer)
			} else {
				if m.Step != "" && steps[m.Step].Foreach != nil {
					return bad("iteration outputs cannot be consumed as scalar")
				}
				src, e = source(m.Step, m.Pointer)
			}
			if e != nil {
				return e
			}
			if !assignable(src, dst) {
				return bad("incompatible dataflow types or bounds")
			}
		}
	}
	if calls > d.Limits.MaxCalls || iterations > d.Limits.MaxIterations {
		return bad("recipe exceeds call/iteration budget")
	}
	return nil
}
func refParts(s string) (string, string, error) {
	p := strings.Split(s, "@")
	if len(p) != 2 || !identifier(p[0], true) || !identifier(p[1], false) {
		return "", "", errors.New("composition: invalid exact reference")
	}
	return p[0], p[1], nil
}

// Usage is caller-owned runtime accounting. Consume rejects an operation before
// it exceeds the admitted aggregate bounds; a rejected operation changes nothing.
// The caller reports the rejected operation/has_more/failed reads as partial.
type Usage struct{ Calls, Iterations, InputBytes, OutputBytes int }

func (u *Usage) Consume(l Limits, iteration bool, inputBytes, outputBytes int) error {
	if u == nil || !validLimits(l) || inputBytes < 0 || outputBytes < 0 || u.Calls < 0 || u.Iterations < 0 || u.InputBytes < 0 || u.OutputBytes < 0 {
		return errors.New("composition: invalid usage")
	}
	i := 0
	if iteration {
		i = 1
	}
	if u.Calls >= l.MaxCalls || u.Iterations > l.MaxIterations-i || inputBytes > l.MaxInputBytes-u.InputBytes || outputBytes > l.MaxOutputBytes-u.OutputBytes {
		return errors.New("composition: partial result: runtime budget exhausted")
	}
	u.Calls++
	u.Iterations += i
	u.InputBytes += inputBytes
	u.OutputBytes += outputBytes
	return nil
}
