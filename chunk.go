package gist

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Format, FormatMarkdown, FormatPlainText are defined in store.go.

const defaultMaxChunkBytes = 4096

// ContentChunk represents a single chunk of content produced by ChunkContent.
type ContentChunk struct {
	// Content is the raw text content of this chunk.
	Content string
	// HeadingPath is the hierarchical heading context (e.g., "Config > Database > Pool").
	HeadingPath string
	// ContentType classifies the chunk as "code" or "prose".
	ContentType string
	// StartByte is the starting byte offset within the original content.
	StartByte int
	// EndByte is the ending byte offset within the original content.
	EndByte int
	// Index is the zero-based sequential position of this chunk.
	Index int
}

// ChunkOption configures the chunking behavior.
type ChunkOption func(*chunkConfig)

type chunkConfig struct {
	maxChunkBytes int
	format        Format
}

func defaultChunkConfig() chunkConfig {
	return chunkConfig{
		maxChunkBytes: defaultMaxChunkBytes,
		format:        FormatMarkdown,
	}
}

// WithMaxChunkBytes sets the maximum size in bytes for each chunk.
func WithMaxChunkBytes(n int) ChunkOption {
	return func(c *chunkConfig) {
		if n > 0 {
			c.maxChunkBytes = n
		}
	}
}

// WithChunkFormat sets the content format for chunking.
func WithChunkFormat(f Format) ChunkOption {
	return func(c *chunkConfig) {
		c.format = f
	}
}

// ChunkContent splits content into chunks based on the configured format.
// For markdown, it parses the AST and splits on headings while preserving
// code blocks. For plain text, it splits on double newlines then by size.
func ChunkContent(content string, opts ...ChunkOption) ([]ContentChunk, error) {
	if content == "" {
		return nil, nil
	}

	cfg := defaultChunkConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	switch cfg.format {
	case FormatPlainText:
		return chunkPlainText(content, cfg.maxChunkBytes), nil
	case FormatJSON:
		return chunkJSON(content, cfg.maxChunkBytes)
	case FormatYAML:
		return chunkYAML(content, cfg.maxChunkBytes)
	default:
		return chunkMarkdown(content, cfg.maxChunkBytes)
	}
}

// section represents a parsed markdown section with heading context.
type section struct {
	headingPath string
	content     string
	contentType string // "code" or "prose"
	startByte   int
	endByte     int
}

func chunkMarkdown(content string, maxBytes int) ([]ContentChunk, error) {
	source := []byte(content)
	md := goldmark.New()
	doc := md.Parser().Parse(text.NewReader(source))

	sections := extractSections(doc, source)

	if len(sections) == 0 {
		// No headings found — treat as plain text.
		return chunkPlainText(content, maxBytes), nil
	}

	var chunks []ContentChunk
	for _, sec := range sections {
		if len(sec.content) <= maxBytes {
			chunks = append(chunks, ContentChunk{
				Content:     sec.content,
				HeadingPath: sec.headingPath,
				ContentType: sec.contentType,
				StartByte:   sec.startByte,
				EndByte:     sec.endByte,
			})
		} else {
			// Oversized section — split on paragraph boundaries.
			sub := splitOnParagraphs(sec.content, maxBytes)
			for _, s := range sub {
				chunks = append(chunks, ContentChunk{
					Content:     s.text,
					HeadingPath: sec.headingPath,
					ContentType: sec.contentType,
					StartByte:   sec.startByte + s.offset,
					EndByte:     sec.startByte + s.offset + len(s.text),
				})
			}
		}
	}

	// Assign sequential indices.
	for i := range chunks {
		chunks[i].Index = i
	}

	return chunks, nil
}

// extractSections walks the markdown AST and collects sections split by headings.
func extractSections(doc ast.Node, source []byte) []section {
	type headingInfo struct {
		level int
		text  string
		pos   int // byte position in source
	}

	var headings []headingInfo

	// Collect all headings.
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if h, ok := n.(*ast.Heading); ok {
			hText := extractText(h, source)
			startByte := 0
			if h.Lines().Len() > 0 {
				startByte = h.Lines().At(0).Start
			}
			// Walk backwards to include the heading markers (e.g. "## ").
			for startByte > 0 && source[startByte-1] != '\n' {
				startByte--
			}
			headings = append(headings, headingInfo{
				level: h.Level,
				text:  hText,
				pos:   startByte,
			})
		}
		return ast.WalkContinue, nil
	})

	if len(headings) == 0 {
		return nil
	}

	// Build heading path stack and create sections.
	var sections []section
	pathStack := make([]string, 0, 6) // max 6 heading levels
	levelStack := make([]int, 0, 6)

	for i, h := range headings {
		// Pop stack until we find a parent level.
		for len(levelStack) > 0 && levelStack[len(levelStack)-1] >= h.level {
			pathStack = pathStack[:len(pathStack)-1]
			levelStack = levelStack[:len(levelStack)-1]
		}
		pathStack = append(pathStack, h.text)
		levelStack = append(levelStack, h.level)

		startByte := h.pos
		var endByte int
		if i+1 < len(headings) {
			endByte = headings[i+1].pos
		} else {
			endByte = len(source)
		}

		sectionContent := string(source[startByte:endByte])
		// Trim trailing whitespace.
		sectionContent = strings.TrimRight(sectionContent, "\n\r\t ")

		if sectionContent == "" {
			continue
		}

		sec := section{
			headingPath: strings.Join(pathStack, " > "),
			content:     sectionContent,
			contentType: detectContentType(sectionContent),
			startByte:   startByte,
			endByte:     startByte + len(sectionContent),
		}
		sections = append(sections, sec)
	}

	// If content exists before the first heading, include it.
	if len(headings) > 0 && headings[0].pos > 0 {
		preContent := strings.TrimRight(string(source[:headings[0].pos]), "\n\r\t ")
		if preContent != "" {
			pre := section{
				headingPath: "",
				content:     preContent,
				contentType: detectContentType(preContent),
				startByte:   0,
				endByte:     len(preContent),
			}
			sections = append([]section{pre}, sections...)
		}
	}

	return sections
}

// extractText returns the text content of a heading node.
func extractText(n ast.Node, source []byte) string {
	var b strings.Builder
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
		}
	}
	return b.String()
}

// detectContentType determines if a section is predominantly code or prose.
func detectContentType(content string) string {
	codeBytes := 0
	inFence := false
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			codeBytes += len(line) + 1
		}
	}
	totalBytes := len(content)
	if totalBytes > 0 && float64(codeBytes)/float64(totalBytes) > 0.5 {
		return "code"
	}
	return "prose"
}

type subChunk struct {
	text   string
	offset int
}

// splitOnParagraphs splits text on double-newline paragraph boundaries
// such that each piece fits within maxBytes. Never splits mid-code-block.
func splitOnParagraphs(content string, maxBytes int) []subChunk {
	paragraphs := splitPreservingCodeBlocks(content)

	var result []subChunk
	var current strings.Builder
	currentOffset := 0
	segmentStart := 0

	for _, p := range paragraphs {
		containsFence := strings.Contains(p, "```")

		if current.Len() == 0 {
			if len(p) > maxBytes && !containsFence {
				// Single prose paragraph exceeds max — force split by bytes.
				for off := 0; off < len(p); off += maxBytes {
					end := off + maxBytes
					if end > len(p) {
						end = len(p)
					}
					result = append(result, subChunk{
						text:   p[off:end],
						offset: currentOffset + off,
					})
				}
				currentOffset += len(p)
				segmentStart = currentOffset
				continue
			}
			// Keep code blocks whole even if they exceed maxBytes.
			current.WriteString(p)
			segmentStart = currentOffset
			currentOffset += len(p)
			continue
		}

		separator := "\n\n"
		needed := current.Len() + len(separator) + len(p)
		if needed > maxBytes {
			// Flush current.
			result = append(result, subChunk{
				text:   current.String(),
				offset: segmentStart,
			})
			current.Reset()

			if len(p) > maxBytes && !containsFence {
				for off := 0; off < len(p); off += maxBytes {
					end := off + maxBytes
					if end > len(p) {
						end = len(p)
					}
					result = append(result, subChunk{
						text:   p[off:end],
						offset: currentOffset + off,
					})
				}
				currentOffset += len(p)
				segmentStart = currentOffset
				continue
			}
			segmentStart = currentOffset
			current.WriteString(p)
			currentOffset += len(p)
			continue
		}

		current.WriteString(separator)
		current.WriteString(p)
		currentOffset += len(separator) + len(p)
	}

	if current.Len() > 0 {
		result = append(result, subChunk{
			text:   current.String(),
			offset: segmentStart,
		})
	}

	return result
}

// splitPreservingCodeBlocks splits content on double newlines but never
// breaks inside a fenced code block (```).
func splitPreservingCodeBlocks(content string) []string {
	var parts []string
	var current strings.Builder
	inFence := false
	lines := strings.Split(content, "\n")
	prevEmpty := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
		}

		isEmpty := trimmed == ""

		if !inFence && isEmpty && prevEmpty {
			// Double newline boundary — flush.
			text := current.String()
			text = strings.TrimRight(text, "\n")
			if text != "" {
				parts = append(parts, text)
			}
			current.Reset()
			prevEmpty = false
			continue
		}

		if current.Len() > 0 || !isEmpty {
			if current.Len() > 0 {
				current.WriteByte('\n')
			}
			current.WriteString(line)
		}
		prevEmpty = isEmpty
	}

	if current.Len() > 0 {
		text := strings.TrimRight(current.String(), "\n")
		if text != "" {
			parts = append(parts, text)
		}
	}

	return parts
}

// chunkPlainText splits plain text on double newlines, then by size.
func chunkPlainText(content string, maxBytes int) []ContentChunk {
	paragraphs := strings.Split(content, "\n\n")

	var chunks []ContentChunk
	var current strings.Builder
	offset := 0
	segmentStart := 0

	for i, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			if i < len(paragraphs)-1 {
				offset += 2
			}
			continue
		}

		if current.Len() == 0 {
			segmentStart = offset
			if len(p) > maxBytes {
				for off := 0; off < len(p); off += maxBytes {
					end := off + maxBytes
					if end > len(p) {
						end = len(p)
					}
					chunks = append(chunks, ContentChunk{
						Content:     p[off:end],
						ContentType: "prose",
						StartByte:   offset + off,
						EndByte:     offset + end,
					})
				}
				offset += len(p)
				if i < len(paragraphs)-1 {
					offset += 2
				}
				continue
			}
			current.WriteString(p)
			offset += len(p)
			if i < len(paragraphs)-1 {
				offset += 2
			}
			continue
		}

		separator := "\n\n"
		if current.Len()+len(separator)+len(p) > maxBytes {
			chunks = append(chunks, ContentChunk{
				Content:     current.String(),
				ContentType: "prose",
				StartByte:   segmentStart,
				EndByte:     segmentStart + current.Len(),
			})
			current.Reset()
			segmentStart = offset

			if len(p) > maxBytes {
				for off := 0; off < len(p); off += maxBytes {
					end := off + maxBytes
					if end > len(p) {
						end = len(p)
					}
					chunks = append(chunks, ContentChunk{
						Content:     p[off:end],
						ContentType: "prose",
						StartByte:   offset + off,
						EndByte:     offset + end,
					})
				}
				offset += len(p)
				if i < len(paragraphs)-1 {
					offset += 2
				}
				continue
			}
			current.WriteString(p)
			offset += len(p)
			if i < len(paragraphs)-1 {
				offset += 2
			}
			continue
		}

		current.WriteString(separator)
		current.WriteString(p)
		offset += len(separator) + len(p)
		if i < len(paragraphs)-1 {
			offset += 2
		}
	}

	if current.Len() > 0 {
		chunks = append(chunks, ContentChunk{
			Content:     current.String(),
			ContentType: "prose",
			StartByte:   segmentStart,
			EndByte:     segmentStart + current.Len(),
		})
	}

	for i := range chunks {
		chunks[i].Index = i
	}

	return chunks
}
