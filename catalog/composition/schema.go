package composition

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// checkJSON detects duplicate keys before decoding would overwrite them.
func checkJSON(b []byte, ceiling int) error {
	if len(b) == 0 || len(b) > ceiling {
		return errors.New("composition: JSON byte limit")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return errors.New("composition: JSON depth limit")
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return e
					}
					s, ok := k.(string)
					if !ok || seen[s] {
						return errors.New("composition: duplicate JSON key")
					}
					seen[s] = true
					if e = walk(depth + 1); e != nil {
						return e
					}
				}
			case '[':
				for d.More() {
					if e := walk(depth + 1); e != nil {
						return e
					}
				}
			default:
				return errors.New("composition: unexpected JSON delimiter")
			}
			_, e = d.Token()
			return e
		}
		return nil
	}
	if e := walk(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("composition: trailing JSON")
	}
	return nil
}
func decode(b []byte, v any, strict bool, ceiling int) error {
	if e := checkJSON(b, ceiling); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	if strict {
		d.DisallowUnknownFields()
	}
	return d.Decode(v)
}
func Parse(b []byte) (Document, error) {
	var d Document
	e := decode(b, &d, true, MaxJSONBytes)
	return d, e
}
func ParseContract(b []byte) (Contract, error) {
	var raw struct {
		ID      string          `json:"id"`
		Version string          `json:"version"`
		Input   json.RawMessage `json:"input_schema"`
		Output  json.RawMessage `json:"output_schema"`
	}
	if e := decode(b, &raw, false, MaxJSONBytes); e != nil {
		return Contract{}, e
	}
	c := Contract{ID: raw.ID, Version: raw.Version, Bytes: append([]byte(nil), b...)}
	if e := decode(raw.Input, &c.Input, true, MaxJSONBytes); e != nil {
		return c, e
	}
	if e := decode(raw.Output, &c.Output, true, MaxJSONBytes); e != nil {
		return c, e
	}
	n := 0
	if e := schemaValid(c.Input, 0, &n); e != nil {
		return c, e
	}
	if e := schemaValid(c.Output, 0, &n); e != nil {
		return c, e
	}
	if !identifier(c.ID, true) || !identifier(c.Version, false) || c.Input.Type != "object" || c.Output.Type != "object" {
		return c, errors.New("composition: invalid contract identity or root")
	}
	return c, nil
}
func schemaValid(s Schema, depth int, n *int) error {
	*n++
	if depth > 16 || *n > 4096 {
		return errors.New("composition: schema complexity limit")
	}
	if s.Type != "object" && (len(s.Properties) > 0 || len(s.Required) > 0 || s.AdditionalProperties != nil) {
		return errors.New("composition: object keywords on non-object")
	}
	if s.Type != "array" && (s.Items != nil || s.MaxItems != 0) {
		return errors.New("composition: array keywords on non-array")
	}
	if s.Type != "string" && s.MaxLength != 0 {
		return errors.New("composition: string bound on non-string")
	}
	switch s.Type {
	case "object":
		if s.AdditionalProperties == nil || *s.AdditionalProperties {
			return errors.New("composition: objects must be explicitly closed")
		}
		seen := map[string]bool{}
		for _, k := range s.Required {
			if _, ok := s.Properties[k]; !ok || seen[k] {
				return errors.New("composition: invalid required property")
			}
			seen[k] = true
		}
		for k, p := range s.Properties {
			if k == "" || len(k) > 256 {
				return errors.New("composition: invalid property")
			}
			if e := schemaValid(p, depth+1, n); e != nil {
				return e
			}
		}
	case "array":
		if s.Items == nil || s.MaxItems < 1 || s.MaxItems > 1024 {
			return errors.New("composition: bounded array required")
		}
		return schemaValid(*s.Items, depth+1, n)
	case "string":
		if s.MaxLength < 1 || s.MaxLength > 1<<20 {
			return errors.New("composition: bounded string required")
		}
	case "boolean", "integer", "number":
	default:
		return errors.New("composition: unsupported schema type")
	}
	return nil
}
func pointer(p string) ([]string, error) {
	if p == "" {
		return nil, nil
	}
	if len(p) > 1024 || !strings.HasPrefix(p, "/") {
		return nil, errors.New("composition: invalid JSON pointer")
	}
	parts := strings.Split(p[1:], "/")
	if len(parts) > 16 {
		return nil, errors.New("composition: pointer depth limit")
	}
	for i, s := range parts {
		var out strings.Builder
		for j := 0; j < len(s); j++ {
			if s[j] == '~' {
				j++
				if j >= len(s) || (s[j] != '0' && s[j] != '1') {
					return nil, errors.New("composition: invalid pointer escape")
				}
				if s[j] == '0' {
					out.WriteByte('~')
				} else {
					out.WriteByte('/')
				}
			} else {
				out.WriteByte(s[j])
			}
		}
		parts[i] = out.String()
	}
	return parts, nil
}
func at(s Schema, p string) (Schema, error) {
	parts, e := pointer(p)
	if e != nil {
		return Schema{}, e
	}
	for _, part := range parts {
		// Array indexing could refer to a missing element: only foreach can unwrap it.
		if s.Type != "object" || !contains(s.Required, part) {
			return Schema{}, errors.New("composition: pointer source is not guaranteed present")
		}
		s = s.Properties[part]
	}
	return s, nil
}
func assignable(src, dst Schema) bool {
	if src.Type != dst.Type {
		return false
	}
	switch src.Type {
	case "string":
		return src.MaxLength <= dst.MaxLength
	case "array":
		return src.MaxItems <= dst.MaxItems && assignable(*src.Items, *dst.Items)
	case "object":
		for _, k := range dst.Required {
			if !contains(src.Required, k) {
				return false
			}
		}
		for k, s := range src.Properties {
			d, ok := dst.Properties[k]
			if !ok || !assignable(s, d) {
				return false
			}
		}
	}
	return true
}

// ValidateValue checks untrusted task/tool output values against the supported
// contract before the caller uses them as downstream input. It grants no rights.
func ValidateValue(s Schema, b []byte, maxBytes int) error {
	if maxBytes < 1 || maxBytes > 4<<20 {
		return errors.New("composition: invalid value budget")
	}
	n := 0
	if e := schemaValid(s, 0, &n); e != nil {
		return e
	}
	if e := checkJSON(b, maxBytes); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if e := d.Decode(&v); e != nil {
		return e
	}
	return value(s, v)
}
func value(s Schema, v any) error {
	fail := func() error { return fmt.Errorf("composition: value violates %s schema", s.Type) }
	switch s.Type {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return fail()
		}
		for _, k := range s.Required {
			if _, ok := m[k]; !ok {
				return fail()
			}
		}
		for k, v := range m {
			p, ok := s.Properties[k]
			if !ok {
				return fail()
			}
			if e := value(p, v); e != nil {
				return e
			}
		}
	case "array":
		a, ok := v.([]any)
		if !ok || len(a) > s.MaxItems {
			return fail()
		}
		for _, v := range a {
			if e := value(*s.Items, v); e != nil {
				return e
			}
		}
	case "string":
		x, ok := v.(string)
		if !ok || utf8.RuneCountInString(x) > s.MaxLength {
			return fail()
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fail()
		}
	case "number", "integer":
		x, ok := v.(json.Number)
		if !ok {
			return fail()
		}
		f, e := strconv.ParseFloat(string(x), 64)
		if e != nil || math.IsInf(f, 0) || math.IsNaN(f) || (s.Type == "integer" && math.Trunc(f) != f) {
			return fail()
		}
	default:
		return fail()
	}
	return nil
}
func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
func identifier(s string, slash bool) bool {
	if s == "" || len(s) > 256 {
		return false
	}
	for _, c := range s {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' || slash && c == '/' {
			continue
		}
		return false
	}
	return true
}
