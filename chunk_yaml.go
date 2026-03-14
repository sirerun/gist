package gist

import (
	"gopkg.in/yaml.v3"
)

// chunkYAML splits YAML content into chunks by parsing it and delegating
// to the same structured chunking logic used for JSON.
func chunkYAML(content string, maxBytes int) ([]ContentChunk, error) {
	var raw any
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		// Invalid YAML — fall back to plain text chunking.
		return chunkPlainText(content, maxBytes), nil
	}

	// yaml.v3 unmarshals maps as map[string]any when keys are strings,
	// but may produce map[any]any for non-string keys. Normalize.
	raw = normalizeYAML(raw)

	var chunks []ContentChunk
	chunkStructured(raw, nil, maxBytes, &chunks)

	if len(chunks) == 0 {
		chunks = append(chunks, ContentChunk{
			Content:     content,
			ContentType: "prose",
		})
	}

	for i := range chunks {
		chunks[i].Index = i
	}
	return chunks, nil
}

// normalizeYAML converts map[any]any (produced by yaml.v3 for some inputs)
// to map[string]any recursively, so chunkStructured can handle it uniformly.
func normalizeYAML(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			val[k] = normalizeYAML(child)
		}
		return val
	case map[any]any:
		out := make(map[string]any, len(val))
		for k, child := range val {
			key, ok := k.(string)
			if !ok {
				key = stringify(k)
			}
			out[key] = normalizeYAML(child)
		}
		return out
	case []any:
		for i, child := range val {
			val[i] = normalizeYAML(child)
		}
		return val
	default:
		return v
	}
}

func stringify(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := yaml.Marshal(v)
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return string(b)
}
