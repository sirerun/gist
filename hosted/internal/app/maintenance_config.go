package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

// DecodeMaintenanceTargets rejects ambiguous or noncanonical authority input.
// Parser errors never echo the operator's supplied configuration.
func DecodeMaintenanceTargets(raw []byte) ([]MaintenanceTarget, error) {
	invalid := errors.New("app: invalid event maintenance target configuration")
	if !validAuthorityJSON(raw) {
		return nil, invalid
	}
	var entries []json.RawMessage
	d := json.NewDecoder(bytes.NewReader(raw))
	if d.Decode(&entries) != nil || len(entries) == 0 || len(entries) > 100 {
		return nil, invalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, invalid
	}
	targets := make([]MaintenanceTarget, 0, len(entries))
	seenWorkspaces := map[string]bool{}
	for _, entry := range entries {
		dec := json.NewDecoder(bytes.NewReader(entry))
		token, err := dec.Token()
		if err != nil || token != json.Delim('{') {
			return nil, invalid
		}
		seen := map[string]bool{}
		var target MaintenanceTarget
		for dec.More() {
			token, err := dec.Token()
			if err != nil {
				return nil, invalid
			}
			key, ok := token.(string)
			if !ok || seen[key] {
				return nil, invalid
			}
			seen[key] = true
			switch key {
			case "workspace_id":
				if dec.Decode(&target.WorkspaceID) != nil {
					return nil, invalid
				}
			case "subject":
				if dec.Decode(&target.Subject) != nil {
					return nil, invalid
				}
			default:
				return nil, invalid
			}
		}
		if token, err = dec.Token(); err != nil || token != json.Delim('}') {
			return nil, invalid
		}
		if target.WorkspaceID == "" || target.Subject == "" || seenWorkspaces[target.WorkspaceID] {
			return nil, invalid
		}
		seenWorkspaces[target.WorkspaceID] = true
		targets = append(targets, target)
	}
	return targets, nil
}

// encoding/json replaces malformed Unicode with U+FFFD. Authority identities
// must retain exact valid strings, including correctly paired escaped units.
func validAuthorityJSON(raw []byte) bool {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		for i++; i < len(raw) && raw[i] != '"'; i++ {
			if raw[i] != '\\' {
				continue
			}
			i++
			if raw[i] != 'u' {
				continue
			}
			unit, ok := authorityUnicodeUnit(raw, i+1)
			if !ok {
				return false
			}
			i += 4
			if unit >= 0xd800 && unit <= 0xdbff {
				if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
					return false
				}
				low, valid := authorityUnicodeUnit(raw, i+3)
				if !valid || low < 0xdc00 || low > 0xdfff {
					return false
				}
				i += 6
			} else if unit >= 0xdc00 && unit <= 0xdfff {
				return false
			}
		}
	}
	return true
}
func authorityUnicodeUnit(raw []byte, start int) (uint64, bool) {
	if start+4 > len(raw) {
		return 0, false
	}
	unit, err := strconv.ParseUint(string(raw[start:start+4]), 16, 16)
	return unit, err == nil
}
