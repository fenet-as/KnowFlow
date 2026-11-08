package chunk

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Chunk represents a segmented portion of text with positional metadata.
type Chunk struct {
	Index      int     `json:"index"`
	Heading    *string `json:"heading"`
	Content    string  `json:"content"`
	TokenCount int     `json:"token_count"`
}

// ChunkerOptions configures the token packing limits and overlap.
type ChunkerOptions struct {
	TargetTokens int
	MaxTokens    int
	OverlapPct   int
}

// DefaultChunkerOptions returns default options (650 target, 800 max, 12% overlap).
func DefaultChunkerOptions() ChunkerOptions {
	return ChunkerOptions{
		TargetTokens: 650,
		MaxTokens:    800,
		OverlapPct:   12,
	}
}

// Chunker defines the contract for deterministic text segmentation.
type Chunker interface {
	Split(content string) ([]Chunk, error)
}

// MarkdownChunker implements Chunker for Markdown and plain text documents.
type MarkdownChunker struct {
	opts ChunkerOptions
}

// NewMarkdownChunker creates a new MarkdownChunker.
func NewMarkdownChunker(opts ChunkerOptions) *MarkdownChunker {
	if opts.TargetTokens <= 0 {
		opts.TargetTokens = 650
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 800
	}
	if opts.TargetTokens > opts.MaxTokens {
		opts.TargetTokens = opts.MaxTokens
	}
	if opts.OverlapPct < 0 {
		opts.OverlapPct = 0
	} else if opts.OverlapPct > 50 {
		opts.OverlapPct = 50
	}
	return &MarkdownChunker{opts: opts}
}

// EstimateTokens calculates approximate token count as characters / 4.
func EstimateTokens(text string) int {
	charCount := utf8.RuneCountInString(text)
	if charCount == 0 {
		return 0
	}
	tokens := charCount / 4
	if tokens == 0 {
		return 1
	}
	return tokens
}

type section struct {
	heading *string
	text    string
}

var headingRegex = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// Split segments content into deterministic chunks obeying heading boundaries,
// paragraph boundaries, code blocks, target/max token sizes, and section-isolated overlap.
func (c *MarkdownChunker) Split(content string) ([]Chunk, error) {
	// 1. Normalize line endings
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	trimmed := strings.TrimSpace(normalized)
	if trimmed == "" {
		return []Chunk{}, nil
	}

	// 2. Split into sections based on Markdown headings
	sections := c.extractSections(normalized)

	// 3. Process each section into chunks
	var allChunks []Chunk
	chunkIndex := 0

	for _, sec := range sections {
		secChunks := c.chunkSection(sec.text, sec.heading)
		for _, sc := range secChunks {
			sc.Index = chunkIndex
			chunkIndex++
			allChunks = append(allChunks, sc)
		}
	}

	return allChunks, nil
}

func (c *MarkdownChunker) extractSections(text string) []section {
	lines := strings.Split(text, "\n")
	var sections []section

	var currentHeading *string
	var currentLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		match := headingRegex.FindStringSubmatch(trimmedLine)
		if len(match) == 3 {
			// Found a heading
			if len(currentLines) > 0 || currentHeading != nil {
				secText := strings.TrimSpace(strings.Join(currentLines, "\n"))
				if secText != "" {
					sections = append(sections, section{
						heading: currentHeading,
						text:    secText,
					})
				}
				currentLines = nil
			}
			hTitle := strings.TrimSpace(match[2])
			currentHeading = &hTitle
			currentLines = append(currentLines, line)
		} else {
			currentLines = append(currentLines, line)
		}
	}

	if len(currentLines) > 0 {
		secText := strings.TrimSpace(strings.Join(currentLines, "\n"))
		if secText != "" {
			sections = append(sections, section{
				heading: currentHeading,
				text:    secText,
			})
		}
	}

	if len(sections) == 0 && strings.TrimSpace(text) != "" {
		sections = append(sections, section{
			heading: nil,
			text:    strings.TrimSpace(text),
		})
	}

	return sections
}

// chunkSection segments a single section's text into chunks with section-scoped overlap.
func (c *MarkdownChunker) chunkSection(text string, heading *string) []Chunk {
	paragraphs := extractParagraphsAndFences(text)
	if len(paragraphs) == 0 {
		return nil
	}

	// Sub-split any paragraph that exceeds maxTokens
	var units []string
	for _, p := range paragraphs {
		if EstimateTokens(p) <= c.opts.MaxTokens {
			units = append(units, p)
		} else {
			units = append(units, c.splitLargeParagraph(p)...)
		}
	}

	var chunks []Chunk
	var currentUnits []string
	currentTokens := 0

	for _, unit := range units {
		unitTokens := EstimateTokens(unit)

		if currentTokens > 0 && (currentTokens+unitTokens > c.opts.TargetTokens) {
			// Finalize current chunk
			chunkContent := strings.TrimSpace(strings.Join(currentUnits, "\n\n"))
			if chunkContent != "" {
				chunks = append(chunks, Chunk{
					Heading:    heading,
					Content:    chunkContent,
					TokenCount: EstimateTokens(chunkContent),
				})
			}

			// Compute overlap from end of chunkContent
			overlapText := c.computeOverlap(chunkContent)

			currentUnits = nil
			currentTokens = 0

			if overlapText != "" {
				currentUnits = append(currentUnits, overlapText)
				currentTokens = EstimateTokens(overlapText)
			}
		}

		currentUnits = append(currentUnits, unit)
		currentTokens += unitTokens
	}

	if len(currentUnits) > 0 {
		chunkContent := strings.TrimSpace(strings.Join(currentUnits, "\n\n"))
		if chunkContent != "" {
			chunks = append(chunks, Chunk{
				Heading:    heading,
				Content:    chunkContent,
				TokenCount: EstimateTokens(chunkContent),
			})
		}
	}

	return chunks
}

// computeOverlap extracts roughly overlapPct from the tail of text, snapped to a sentence or line boundary.
func (c *MarkdownChunker) computeOverlap(text string) string {
	if c.opts.OverlapPct <= 0 || strings.TrimSpace(text) == "" {
		return ""
	}

	totalTokens := EstimateTokens(text)
	targetOverlapTokens := (c.opts.TargetTokens * c.opts.OverlapPct) / 100
	if targetOverlapTokens <= 0 || totalTokens <= targetOverlapTokens {
		return ""
	}

	// Approximate characters needed
	targetChars := targetOverlapTokens * 4
	runes := []rune(text)
	if len(runes) <= targetChars {
		return ""
	}

	startIdx := len(runes) - targetChars

	// Snap forward to nearest newline or sentence boundary within 60 runes
	sub := string(runes[startIdx:])
	for i, r := range sub {
		if i > 60 {
			break
		}
		if r == '\n' || r == '.' || r == '!' || r == '?' {
			startIdx = startIdx + i + 1
			break
		}
	}

	overlap := strings.TrimSpace(string(runes[startIdx:]))
	return overlap
}

// extractParagraphsAndFences splits text on blank lines while preserving code fences.
func extractParagraphsAndFences(text string) []string {
	lines := strings.Split(text, "\n")
	var paragraphs []string
	var current []string
	inFence := false
	fenceMarker := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for code fence start/end
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !inFence {
				inFence = true
				fenceMarker = marker
			} else if marker == fenceMarker {
				inFence = false
				fenceMarker = ""
			}
			current = append(current, line)
			continue
		}

		if inFence {
			current = append(current, line)
			continue
		}

		if trimmed == "" {
			if len(current) > 0 {
				p := strings.TrimSpace(strings.Join(current, "\n"))
				if p != "" {
					paragraphs = append(paragraphs, p)
				}
				current = nil
			}
		} else {
			current = append(current, line)
		}
	}

	if len(current) > 0 {
		p := strings.TrimSpace(strings.Join(current, "\n"))
		if p != "" {
			paragraphs = append(paragraphs, p)
		}
	}

	return paragraphs
}

// splitLargeParagraph breaks an oversized paragraph by sentences, and then by words/runes if needed.
func (c *MarkdownChunker) splitLargeParagraph(text string) []string {
	sentences := splitSentences(text)
	var units []string
	var current []string
	currentTokens := 0

	for _, s := range sentences {
		sTokens := EstimateTokens(s)
		if sTokens > c.opts.MaxTokens {
			// Flush current
			if len(current) > 0 {
				units = append(units, strings.Join(current, " "))
				current = nil
				currentTokens = 0
			}
			// Hard-split long sentence by runes
			units = append(units, c.splitLongSentence(s)...)
			continue
		}

		if currentTokens+sTokens > c.opts.MaxTokens {
			units = append(units, strings.Join(current, " "))
			current = nil
			currentTokens = 0
		}

		current = append(current, s)
		currentTokens += sTokens
	}

	if len(current) > 0 {
		units = append(units, strings.Join(current, " "))
	}

	return units
}

var sentenceEndRegex = regexp.MustCompile(`([.!?])\s+`)

func splitSentences(text string) []string {
	indices := sentenceEndRegex.FindAllStringIndex(text, -1)
	if len(indices) == 0 {
		return []string{text}
	}

	var sentences []string
	start := 0
	for _, idx := range indices {
		end := idx[1]
		sentences = append(sentences, strings.TrimSpace(text[start:end]))
		start = end
	}
	if start < len(text) {
		remainder := strings.TrimSpace(text[start:])
		if remainder != "" {
			sentences = append(sentences, remainder)
		}
	}
	return sentences
}

func (c *MarkdownChunker) splitLongSentence(text string) []string {
	maxChars := c.opts.MaxTokens * 4
	runes := []rune(text)
	var parts []string

	for len(runes) > 0 {
		if len(runes) <= maxChars {
			parts = append(parts, string(runes))
			break
		}
		// Try to break at whitespace
		breakIdx := maxChars
		for i := maxChars; i > maxChars-50 && i > 0; i-- {
			if runes[i] == ' ' || runes[i] == '\t' || runes[i] == '\n' {
				breakIdx = i
				break
			}
		}
		parts = append(parts, strings.TrimSpace(string(runes[:breakIdx])))
		runes = runes[breakIdx:]
	}

	return parts
}
