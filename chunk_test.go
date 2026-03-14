package gist

import (
	"strings"
	"testing"
)

func TestChunkContent_EmptyInput(t *testing.T) {
	chunks, err := ChunkContent("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunks != nil {
		t.Fatalf("expected nil, got %d chunks", len(chunks))
	}
}

func TestChunkContent_SimpleHeadingSplitting(t *testing.T) {
	input := "# Introduction\n\nThis is the intro.\n\n# Details\n\nHere are details.\n"

	chunks, err := ChunkContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	tests := []struct {
		idx         int
		wantHeading string
		wantContain string
	}{
		{0, "Introduction", "This is the intro."},
		{1, "Details", "Here are details."},
	}
	for _, tt := range tests {
		c := chunks[tt.idx]
		if c.HeadingPath != tt.wantHeading {
			t.Errorf("chunk %d: heading = %q, want %q", tt.idx, c.HeadingPath, tt.wantHeading)
		}
		if !strings.Contains(c.Content, tt.wantContain) {
			t.Errorf("chunk %d: content missing %q", tt.idx, tt.wantContain)
		}
		if c.Index != tt.idx {
			t.Errorf("chunk %d: index = %d, want %d", tt.idx, c.Index, tt.idx)
		}
	}
}

func TestChunkContent_NestedHeadingHierarchy(t *testing.T) {
	input := `# Config

Top-level config.

## Database

Database settings.

### Pool

Pool configuration.

## Cache

Cache settings.
`

	chunks, err := ChunkContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPaths := []string{
		"Config",
		"Config > Database",
		"Config > Database > Pool",
		"Config > Cache",
	}

	if len(chunks) != len(wantPaths) {
		t.Fatalf("expected %d chunks, got %d", len(wantPaths), len(chunks))
	}

	for i, want := range wantPaths {
		if chunks[i].HeadingPath != want {
			t.Errorf("chunk %d: heading path = %q, want %q", i, chunks[i].HeadingPath, want)
		}
	}
}

func TestChunkContent_CodeBlockPreservation(t *testing.T) {
	input := "# Setup\n\nInstall dependencies:\n\n```bash\napt-get update\napt-get install -y golang\n```\n\nThen run the app.\n"

	chunks, err := ChunkContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	c := chunks[0]
	if !strings.Contains(c.Content, "```bash") {
		t.Error("code block fence not preserved")
	}
	if !strings.Contains(c.Content, "apt-get install -y golang") {
		t.Error("code block content not preserved")
	}
	if !strings.Contains(c.Content, "```\n") {
		t.Error("closing fence not preserved")
	}
}

func TestChunkContent_CodeBlockNeverSplit(t *testing.T) {
	// Create a code block that is large but should never be split mid-block.
	var b strings.Builder
	b.WriteString("# Code\n\n```go\n")
	for i := 0; i < 50; i++ {
		b.WriteString("fmt.Println(\"line\")\n")
	}
	b.WriteString("```\n")

	chunks, err := ChunkContent(b.String(), WithMaxChunkBytes(200))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no chunk splits inside the code fence.
	for i, c := range chunks {
		openCount := strings.Count(c.Content, "```")
		if openCount%2 != 0 {
			// Allow chunk that contains entire code block.
			if strings.Contains(c.Content, "```go") && strings.HasSuffix(strings.TrimSpace(c.Content), "```") {
				continue
			}
			t.Errorf("chunk %d has unbalanced fences (count=%d):\n%s", i, openCount, c.Content)
		}
	}
}

func TestChunkContent_OversizedSectionSplitOnParagraphs(t *testing.T) {
	para1 := "This is paragraph one with some content."
	para2 := "This is paragraph two with more content."
	para3 := "This is paragraph three with even more text."
	input := "# Big Section\n\n" + para1 + "\n\n" + para2 + "\n\n" + para3

	maxBytes := len("# Big Section\n\n") + len(para1) + 10
	chunks, err := ChunkContent(input, WithMaxChunkBytes(maxBytes))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for oversized section, got %d", len(chunks))
	}

	for i, c := range chunks {
		if c.HeadingPath != "Big Section" {
			t.Errorf("chunk %d: heading = %q, want %q", i, c.HeadingPath, "Big Section")
		}
	}
}

func TestChunkContent_PlainTextFallback(t *testing.T) {
	input := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."

	chunks, err := ChunkContent(input, WithChunkFormat(FormatPlainText))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	combined := ""
	for _, c := range chunks {
		combined += c.Content
	}
	if !strings.Contains(combined, "First paragraph") {
		t.Error("missing first paragraph")
	}
	if !strings.Contains(combined, "Third paragraph") {
		t.Error("missing third paragraph")
	}

	for _, c := range chunks {
		if c.ContentType != "prose" {
			t.Errorf("plain text chunk should be prose, got %q", c.ContentType)
		}
	}
}

func TestChunkContent_PlainTextNoMarkdown(t *testing.T) {
	input := "This is plain text without any headings.\n\nAnother paragraph here.\n\nAnd a third one."

	chunks, err := ChunkContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 1 {
		t.Fatal("expected at least 1 chunk")
	}

	for _, c := range chunks {
		if c.ContentType != "prose" {
			t.Errorf("expected prose, got %q", c.ContentType)
		}
	}
}

func TestChunkContent_MixedCodeAndProse(t *testing.T) {
	input := "# Prose Section\n\nJust some regular text here.\n\n# Code Section\n\n" +
		"```python\nprint('hello')\nfor i in range(100):\n    print(i)\n    result = process(i)\n    data.append(result)\n    log.info(f'done {i}')\n```\n\n" +
		"```go\nfmt.Println(\"world\")\nfmt.Println(\"more\")\nfmt.Println(\"code\")\n```\n"

	chunks, err := ChunkContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	if chunks[0].ContentType != "prose" {
		t.Errorf("first chunk: content type = %q, want prose", chunks[0].ContentType)
	}

	if chunks[1].ContentType != "code" {
		t.Errorf("second chunk: content type = %q, want code", chunks[1].ContentType)
	}
}

func TestChunkContent_ContentBeforeFirstHeading(t *testing.T) {
	input := "Some intro text before any heading.\n\n# First Heading\n\nContent here.\n"

	chunks, err := ChunkContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	if chunks[0].HeadingPath != "" {
		t.Errorf("pre-heading chunk should have empty heading path, got %q", chunks[0].HeadingPath)
	}
	if !strings.Contains(chunks[0].Content, "Some intro text") {
		t.Error("pre-heading content missing")
	}
}

func TestChunkContent_SequentialIndices(t *testing.T) {
	input := "# A\n\nContent A.\n\n# B\n\nContent B.\n\n# C\n\nContent C.\n"

	chunks, err := ChunkContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk %d has index %d", i, c.Index)
		}
	}
}

func TestWithMaxChunkBytes_ZeroOrNegative(t *testing.T) {
	cfg := defaultChunkConfig()
	WithMaxChunkBytes(0)(&cfg)
	if cfg.maxChunkBytes != defaultMaxChunkBytes {
		t.Errorf("zero should not change default, got %d", cfg.maxChunkBytes)
	}
	WithMaxChunkBytes(-1)(&cfg)
	if cfg.maxChunkBytes != defaultMaxChunkBytes {
		t.Errorf("negative should not change default, got %d", cfg.maxChunkBytes)
	}
}

func TestChunkContent_PlainTextSmallMaxBytes(t *testing.T) {
	input := "AAA\n\nBBB\n\nCCC"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatPlainText), WithMaxChunkBytes(5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks with small max, got %d", len(chunks))
	}

	for _, c := range chunks {
		if len(c.Content) > 5 {
			t.Errorf("chunk exceeds max bytes: %q (len=%d)", c.Content, len(c.Content))
		}
	}
}

func TestChunkContent_PlainTextOversizedParagraph(t *testing.T) {
	// A single paragraph that exceeds max bytes.
	input := strings.Repeat("x", 100)
	chunks, err := ChunkContent(input, WithChunkFormat(FormatPlainText), WithMaxChunkBytes(30))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// Reassemble.
	var combined string
	for _, c := range chunks {
		combined += c.Content
	}
	if combined != input {
		t.Error("reassembled content doesn't match input")
	}
}

func TestChunkContent_PlainTextMultipleParagraphFlush(t *testing.T) {
	// Multiple paragraphs where accumulating causes flush.
	input := "AAAA\n\nBBBB\n\nCCCC\n\nDDDD"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatPlainText), WithMaxChunkBytes(10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if c.ContentType != "prose" {
			t.Errorf("expected prose, got %q", c.ContentType)
		}
	}
}

func TestChunkContent_MarkdownOversizedWithCodeBlock(t *testing.T) {
	// A markdown section with a large code block that exceeds maxBytes.
	// The code block should be kept whole.
	var b strings.Builder
	b.WriteString("# Section\n\nSome intro.\n\n```go\n")
	for i := 0; i < 20; i++ {
		b.WriteString("fmt.Println(\"line\")\n")
	}
	b.WriteString("```\n\nAfter code.\n")

	chunks, err := ChunkContent(b.String(), WithMaxChunkBytes(100))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, c := range chunks {
		fences := strings.Count(c.Content, "```")
		if fences%2 != 0 {
			if strings.Contains(c.Content, "```go") && strings.HasSuffix(strings.TrimSpace(c.Content), "```") {
				continue
			}
			t.Errorf("chunk %d has unbalanced fences", i)
		}
	}
}

func TestChunkContent_MarkdownOversizedProseParagraph(t *testing.T) {
	// A markdown section with a very long prose paragraph that must be byte-split.
	longPara := strings.Repeat("word ", 100)
	input := "# Long\n\nShort intro.\n\n" + longPara

	chunks, err := ChunkContent(input, WithMaxChunkBytes(50))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if c.HeadingPath != "Long" {
			t.Errorf("unexpected heading path %q", c.HeadingPath)
		}
	}
}

func TestChunkContent_PlainTextOversizedAfterAccumulation(t *testing.T) {
	// First paragraph is small, second is oversized and triggers flush + byte split.
	small := "AA"
	large := strings.Repeat("B", 50)
	input := small + "\n\n" + large
	chunks, err := ChunkContent(input, WithChunkFormat(FormatPlainText), WithMaxChunkBytes(10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	// First chunk should contain the small paragraph.
	if chunks[0].Content != small {
		t.Errorf("first chunk = %q, want %q", chunks[0].Content, small)
	}
}

func TestSplitPreservingCodeBlocks(t *testing.T) {
	input := "para1\n\npara2\n\n```\ncode line 1\n\n\ncode line 2\n```\n\npara3"
	parts := splitPreservingCodeBlocks(input)

	// Code block with internal blank lines should be one part.
	foundCodeBlock := false
	for _, p := range parts {
		if strings.Contains(p, "```") {
			foundCodeBlock = true
			if !strings.Contains(p, "code line 1") || !strings.Contains(p, "code line 2") {
				t.Error("code block was split")
			}
		}
	}
	if !foundCodeBlock {
		t.Error("no code block found in parts")
	}
}

func TestSplitOnParagraphs_OversizedProse(t *testing.T) {
	// A single very long prose paragraph that exceeds maxBytes with current buffer empty.
	longPara := strings.Repeat("x", 100)
	chunks := splitOnParagraphs(longPara, 30)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple sub-chunks, got %d", len(chunks))
	}
	var combined string
	for _, c := range chunks {
		combined += c.text
	}
	if combined != longPara {
		t.Error("reassembled text doesn't match input")
	}
}

func TestSplitOnParagraphs_FlushThenOversized(t *testing.T) {
	// Current buffer has content, next paragraph causes flush, and next para is oversized prose.
	// splitOnParagraphs uses splitPreservingCodeBlocks which needs double blank lines.
	input := "short\n\n\n" + strings.Repeat("y", 100)
	chunks := splitOnParagraphs(input, 30)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 sub-chunks, got %d", len(chunks))
	}
	// First chunk should contain "short".
	if !strings.Contains(chunks[0].text, "short") {
		t.Errorf("first chunk = %q, should contain %q", chunks[0].text, "short")
	}
}

func TestSplitOnParagraphs_CodeBlockKeptWhole(t *testing.T) {
	// A code block that exceeds maxBytes should be kept whole.
	code := "```\n" + strings.Repeat("line\n", 20) + "```"
	chunks := splitOnParagraphs(code, 30)
	// The entire code block should be in one chunk.
	found := false
	for _, c := range chunks {
		if strings.Contains(c.text, "```") && strings.Count(c.text, "```") == 2 {
			found = true
		}
	}
	if !found {
		t.Error("code block was split across chunks")
	}
}

func TestSplitOnParagraphs_FlushThenCodeBlock(t *testing.T) {
	// Buffer has content, then a code block that causes flush but code block itself
	// exceeds maxBytes and contains fences — kept whole.
	code := "```\n" + strings.Repeat("c\n", 30) + "```"
	input := "short\n\n\n" + code
	chunks := splitOnParagraphs(input, 20)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 sub-chunks, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0].text, "short") {
		t.Errorf("first chunk = %q, should contain %q", chunks[0].text, "short")
	}
}

func TestSplitPreservingCodeBlocks_EmptyInput(t *testing.T) {
	parts := splitPreservingCodeBlocks("")
	if len(parts) != 0 {
		t.Errorf("expected 0 parts for empty input, got %d", len(parts))
	}
}

func TestSplitPreservingCodeBlocks_NoParagraphBreaks(t *testing.T) {
	input := "single paragraph with no breaks"
	parts := splitPreservingCodeBlocks(input)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0] != input {
		t.Errorf("part = %q, want %q", parts[0], input)
	}
}

func TestSplitPreservingCodeBlocks_LeadingBlankLines(t *testing.T) {
	input := "\n\nparagraph after blanks"
	parts := splitPreservingCodeBlocks(input)
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if !strings.Contains(parts[0], "paragraph after blanks") {
		t.Error("expected paragraph content")
	}
}

func TestChunkPlainText_EmptyParagraphs(t *testing.T) {
	// Input with multiple empty paragraphs in a row.
	input := "AAA\n\n\n\nBBB\n\n\n\nCCC"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatPlainText))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
}

func TestChunkPlainText_OversizedMiddleParagraph(t *testing.T) {
	// First paragraph fits, second is oversized and triggers flush + split,
	// third is normal.
	small1 := "AA"
	large := strings.Repeat("B", 50)
	small2 := "CC"
	input := small1 + "\n\n" + large + "\n\n" + small2
	chunks, err := ChunkContent(input, WithChunkFormat(FormatPlainText), WithMaxChunkBytes(10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}
	// Last chunk should contain small2.
	last := chunks[len(chunks)-1]
	if last.Content != small2 {
		t.Errorf("last chunk = %q, want %q", last.Content, small2)
	}
}

func TestChunkPlainText_OversizedFirstParaThenAnother(t *testing.T) {
	// First paragraph is oversized (not the last), followed by more.
	large := strings.Repeat("A", 50)
	input := large + "\n\n" + "small"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatPlainText), WithMaxChunkBytes(20))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
}

func TestChunkPlainText_AccumulatedThenOversized(t *testing.T) {
	// First paragraph is small, second is oversized prose.
	input := "hi\n\n" + strings.Repeat("B", 50) + "\n\nend"
	chunks, err := ChunkContent(input, WithChunkFormat(FormatPlainText), WithMaxChunkBytes(10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}
}

func TestChunkContent_EmptySectionAfterHeading(t *testing.T) {
	// A heading followed immediately by another heading (empty section).
	input := "# One\n\n# Two\n\nContent here."
	chunks, err := ChunkContent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// At least one chunk for the non-empty section.
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
}

func TestSplitOnParagraphs_EmptyInput(t *testing.T) {
	chunks := splitOnParagraphs("", 100)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty, got %d", len(chunks))
	}
}

func TestSplitOnParagraphs_AccumulateAndFlush(t *testing.T) {
	// Multiple small paragraphs that accumulate, then a paragraph triggers flush.
	input := "aa\n\n\nbb\n\n\ncc\n\n\ndd"
	chunks := splitOnParagraphs(input, 8)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
}

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "pure prose",
			content: "Just some text.\nAnother line.",
			want:    "prose",
		},
		{
			name:    "mostly code",
			content: "```\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\n```",
			want:    "code",
		},
		{
			name:    "mixed leaning prose",
			content: "Lots of prose here.\nAnd more prose.\n```\nx\n```",
			want:    "prose",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectContentType(tt.content)
			if got != tt.want {
				t.Errorf("detectContentType() = %q, want %q", got, tt.want)
			}
		})
	}
}
