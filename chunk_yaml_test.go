package gist

import (
	"strings"
	"testing"
)

func TestChunkYAML_FlatObject(t *testing.T) {
	input := "name: Alice\nage: 30\ncity: NYC\n"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatYAML))
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
			t.Error("expected non-empty heading path")
		}
	}
}

func TestChunkYAML_NestedObject(t *testing.T) {
	input := `database:
  host: localhost
  port: 5432
  pool:
    min: 5
    max: 20
cache:
  ttl: 300
`
	chunks, err := ChunkContent(input, WithChunkFormat(FormatYAML), WithMaxChunkBytes(60))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	foundNested := false
	for _, c := range chunks {
		if strings.Contains(c.HeadingPath, " > ") {
			foundNested = true
		}
	}
	if !foundNested {
		t.Error("expected nested heading paths")
	}
}

func TestChunkYAML_Array(t *testing.T) {
	input := "- 1\n- 2\n- 3\n- 4\n- 5\n- 6\n- 7\n- 8\n- 9\n- 10\n"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatYAML), WithMaxChunkBytes(20))
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

func TestChunkYAML_EmptyObject(t *testing.T) {
	input := "{}\n"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for empty object, got %d", len(chunks))
	}
}

func TestChunkYAML_EmptyArray(t *testing.T) {
	input := "[]\n"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for empty array, got %d", len(chunks))
	}
}

func TestChunkYAML_Null(t *testing.T) {
	input := "null\n"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for null, got %d", len(chunks))
	}
}

func TestChunkYAML_Primitive(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"string", "hello world\n"},
		{"number", "42\n"},
		{"boolean", "true\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := ChunkContent(tt.input, WithChunkFormat(FormatYAML))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(chunks) != 1 {
				t.Fatalf("expected 1 chunk, got %d", len(chunks))
			}
		})
	}
}

func TestChunkYAML_OversizedValue(t *testing.T) {
	input := "config:\n  a: " + strings.Repeat("x", 100) + "\n  b: " + strings.Repeat("y", 100) + "\n"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatYAML), WithMaxChunkBytes(50))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for oversized nested object, got %d", len(chunks))
	}
}

func TestChunkYAML_InvalidYAML(t *testing.T) {
	input := ":\n  - :\n  invalid: [unterminated"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected fallback chunks for invalid YAML")
	}
}

func TestChunkYAML_SequentialIndices(t *testing.T) {
	input := "a: 1\nb: 2\nc: 3\nd: 4\n"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatYAML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk %d has index %d", i, c.Index)
		}
	}
}
