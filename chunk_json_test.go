package gist

import (
	"strings"
	"testing"
)

func TestChunkJSON_FlatObject(t *testing.T) {
	input := `{"name":"Alice","age":30,"city":"NYC"}`
	chunks, err := ChunkContent(input, WithChunkFormat(FormatJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	for _, c := range chunks {
		if c.ContentType != "prose" {
			t.Errorf("expected prose, got %q", c.ContentType)
		}
		if c.HeadingPath == "" {
			t.Error("expected non-empty heading path for object key")
		}
	}
}

func TestChunkJSON_NestedObject(t *testing.T) {
	input := `{"database":{"host":"localhost","port":5432,"pool":{"min":5,"max":20}},"cache":{"ttl":300}}`
	chunks, err := ChunkContent(input, WithChunkFormat(FormatJSON), WithMaxChunkBytes(60))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Should have heading paths with " > " separators for nested keys.
	foundNested := false
	for _, c := range chunks {
		if strings.Contains(c.HeadingPath, " > ") {
			foundNested = true
		}
	}
	if !foundNested {
		t.Error("expected nested heading paths with ' > ' separator")
	}
}

func TestChunkJSON_Array(t *testing.T) {
	input := `[1,2,3,4,5,6,7,8,9,10]`
	chunks, err := ChunkContent(input, WithChunkFormat(FormatJSON), WithMaxChunkBytes(20))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for array, got %d", len(chunks))
	}
	for _, c := range chunks {
		if !strings.Contains(c.HeadingPath, "[") {
			t.Errorf("expected array range in heading path, got %q", c.HeadingPath)
		}
	}
}

func TestChunkJSON_EmptyObject(t *testing.T) {
	input := `{}`
	chunks, err := ChunkContent(input, WithChunkFormat(FormatJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for empty object, got %d", len(chunks))
	}
	if chunks[0].Content != input {
		t.Errorf("expected content %q, got %q", input, chunks[0].Content)
	}
}

func TestChunkJSON_EmptyArray(t *testing.T) {
	input := `[]`
	chunks, err := ChunkContent(input, WithChunkFormat(FormatJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for empty array, got %d", len(chunks))
	}
}

func TestChunkJSON_Null(t *testing.T) {
	input := `null`
	chunks, err := ChunkContent(input, WithChunkFormat(FormatJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for null, got %d", len(chunks))
	}
}

func TestChunkJSON_Primitive(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"string", `"hello world"`},
		{"number", `42`},
		{"boolean", `true`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := ChunkContent(tt.input, WithChunkFormat(FormatJSON))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(chunks) != 1 {
				t.Fatalf("expected 1 chunk, got %d", len(chunks))
			}
		})
	}
}

func TestChunkJSON_OversizedValue(t *testing.T) {
	// A key with a very large nested object that must be recursed into.
	input := `{"config":{"a":"` + strings.Repeat("x", 100) + `","b":"` + strings.Repeat("y", 100) + `"}}`
	chunks, err := ChunkContent(input, WithChunkFormat(FormatJSON), WithMaxChunkBytes(50))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for oversized nested object, got %d", len(chunks))
	}
}

func TestChunkJSON_InvalidJSON(t *testing.T) {
	input := `{not valid json`
	chunks, err := ChunkContent(input, WithChunkFormat(FormatJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected fallback chunks for invalid JSON")
	}
}

func TestChunkJSON_SequentialIndices(t *testing.T) {
	input := `{"a":1,"b":2,"c":3,"d":4}`
	chunks, err := ChunkContent(input, WithChunkFormat(FormatJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk %d has index %d", i, c.Index)
		}
	}
}
