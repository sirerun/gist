package gist

import (
	"encoding/json"
	"fmt"
	"strings"
)

// chunkJSON splits JSON content into chunks based on its structure.
// Objects are chunked by top-level keys; arrays are grouped by element ranges.
func chunkJSON(content string, maxBytes int) ([]ContentChunk, error) {
	var raw any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		// Invalid JSON — fall back to plain text chunking.
		return chunkPlainText(content, maxBytes), nil
	}

	var chunks []ContentChunk
	chunkStructured(raw, nil, maxBytes, &chunks)

	if len(chunks) == 0 {
		// null, empty object, empty array, or primitive — single chunk.
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

// chunkStructured recursively chunks a parsed JSON/YAML value.
func chunkStructured(v any, path []string, maxBytes int, chunks *[]ContentChunk) {
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			return
		}
		for key, child := range val {
			childPath := append(append([]string{}, path...), key)
			serialized, err := json.Marshal(map[string]any{key: child})
			if err != nil {
				continue
			}
			if len(serialized) <= maxBytes {
				*chunks = append(*chunks, ContentChunk{
					Content:     string(serialized),
					HeadingPath: strings.Join(childPath, " > "),
					ContentType: "prose",
				})
			} else {
				// Try to recurse into nested structure.
				if nested, ok := child.(map[string]any); ok && len(nested) > 0 {
					chunkStructured(nested, childPath, maxBytes, chunks)
				} else if arr, ok := child.([]any); ok && len(arr) > 0 {
					chunkStructured(arr, childPath, maxBytes, chunks)
				} else {
					// Primitive or non-splittable — emit as-is.
					*chunks = append(*chunks, ContentChunk{
						Content:     string(serialized),
						HeadingPath: strings.Join(childPath, " > "),
						ContentType: "prose",
					})
				}
			}
		}
	case []any:
		if len(val) == 0 {
			return
		}
		chunkArray(val, path, maxBytes, chunks)
	default:
		// Primitive (string, number, bool, nil) — handled by caller.
	}
}

// chunkArray groups array elements into chunks that fit within maxBytes.
func chunkArray(arr []any, path []string, maxBytes int, chunks *[]ContentChunk) {
	start := 0
	var buf []any

	for i, elem := range arr {
		candidate := append(buf, elem)
		serialized, err := json.Marshal(candidate)
		if err != nil {
			continue
		}
		if len(serialized) > maxBytes && len(buf) > 0 {
			// Flush current buffer.
			flushArrayChunk(buf, path, start, i-1, chunks)
			buf = []any{elem}
			start = i
		} else {
			buf = candidate
		}
		// Single element exceeds maxBytes — try to recurse or emit.
		if len(buf) == 1 {
			single, _ := json.Marshal([]any{elem})
			if len(single) > maxBytes {
				if m, ok := elem.(map[string]any); ok && len(m) > 0 {
					elemPath := append(append([]string{}, path...), fmt.Sprintf("[%d]", i))
					chunkStructured(m, elemPath, maxBytes, chunks)
					buf = nil
					start = i + 1
				} else if a, ok := elem.([]any); ok && len(a) > 0 {
					elemPath := append(append([]string{}, path...), fmt.Sprintf("[%d]", i))
					chunkStructured(a, elemPath, maxBytes, chunks)
					buf = nil
					start = i + 1
				}
				// Otherwise keep single oversized element in buf for flush.
			}
		}
	}

	if len(buf) > 0 {
		flushArrayChunk(buf, path, start, len(arr)-1, chunks)
	}
}

func flushArrayChunk(buf []any, path []string, start, end int, chunks *[]ContentChunk) {
	serialized, err := json.Marshal(buf)
	if err != nil {
		return
	}
	label := fmt.Sprintf("[%d-%d]", start, end)
	if start == end {
		label = fmt.Sprintf("[%d]", start)
	}
	headingPath := label
	if len(path) > 0 {
		headingPath = strings.Join(path, " > ") + " > " + label
	}
	*chunks = append(*chunks, ContentChunk{
		Content:     string(serialized),
		HeadingPath: headingPath,
		ContentType: "prose",
	})
}
