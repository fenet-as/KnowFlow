package chunk

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkdownChunker_TableDriven(t *testing.T) {
	opts := ChunkerOptions{
		TargetTokens: 100, // Small target for concise testing
		MaxTokens:    150,
		OverlapPct:   12,
	}
	chunker := NewMarkdownChunker(opts)

	tests := []struct {
		name          string
		input         string
		expectedCount int
		validateFn    func(t *testing.T, chunks []Chunk)
	}{
		{
			name:          "empty input",
			input:         "",
			expectedCount: 0,
			validateFn: func(t *testing.T, chunks []Chunk) {
				assert.Empty(t, chunks)
			},
		},
		{
			name:          "whitespace only",
			input:         "   \n\t  \n  ",
			expectedCount: 0,
			validateFn: func(t *testing.T, chunks []Chunk) {
				assert.Empty(t, chunks)
			},
		},
		{
			name: "plain text without headings",
			input: "This is a simple plain text document without any markdown headings.\n\n" +
				"It has two paragraphs separated by a blank line.",
			expectedCount: 1,
			validateFn: func(t *testing.T, chunks []Chunk) {
				require.Len(t, chunks, 1)
				assert.Nil(t, chunks[0].Heading)
				assert.Equal(t, 0, chunks[0].Index)
				assert.Contains(t, chunks[0].Content, "simple plain text")
				assert.Contains(t, chunks[0].Content, "two paragraphs")
			},
		},
		{
			name: "headings create distinct sections",
			input: "# Introduction\nWelcome to KnowFlow.\n\n" +
				"## Setup Guide\nFollow these instructions carefully to install the app.",
			expectedCount: 2,
			validateFn: func(t *testing.T, chunks []Chunk) {
				require.Len(t, chunks, 2)
				require.NotNil(t, chunks[0].Heading)
				assert.Equal(t, "Introduction", *chunks[0].Heading)
				assert.Equal(t, 0, chunks[0].Index)

				require.NotNil(t, chunks[1].Heading)
				assert.Equal(t, "Setup Guide", *chunks[1].Heading)
				assert.Equal(t, 1, chunks[1].Index)
			},
		},
		{
			name: "code fences kept together despite interior blank lines",
			input: "# Code Section\nHere is some code:\n\n```go\npackage main\n\nfunc main() {\n    println(\"hello\")\n}\n```\n\nAfter code block.",
			expectedCount: 1,
			validateFn: func(t *testing.T, chunks []Chunk) {
				require.Len(t, chunks, 1)
				assert.Contains(t, chunks[0].Content, "func main()")
				assert.Contains(t, chunks[0].Content, "```go")
			},
		},
		{
			name: "very long paragraph gets split into multiple chunks under MaxTokens",
			input: strings.Repeat("KnowFlow is an intelligent knowledge base system for developers. ", 50),
			validateFn: func(t *testing.T, chunks []Chunk) {
				assert.Greater(t, len(chunks), 1)
				for i, ch := range chunks {
					assert.Equal(t, i, ch.Index)
					assert.LessOrEqual(t, ch.TokenCount, opts.MaxTokens+5, "chunk %d exceeded max tokens: got %d", i, ch.TokenCount)
				}
			},
		},
		{
			name: "Unicode text safety (Japanese, Cyrillic, Emojis)",
			input: "# 多言語サポート 🚀\n" +
				"これは日本語のテキストです。文字化けせずに適切にチャンク化される必要があります。\n\n" +
				"Это русский текст для проверки кодировки UTF-8. Привет мир!",
			expectedCount: 1,
			validateFn: func(t *testing.T, chunks []Chunk) {
				require.Len(t, chunks, 1)
				require.NotNil(t, chunks[0].Heading)
				assert.Equal(t, "多言語サポート 🚀", *chunks[0].Heading)
				assert.Contains(t, chunks[0].Content, "これは日本語のテキストです")
				assert.Contains(t, chunks[0].Content, "Это русский текст")
				assert.Greater(t, chunks[0].TokenCount, 0)
			},
		},
		{
			name: "heading with no body followed by another heading",
			input: "# Empty Heading\n## Actual Heading\nContent for actual heading.",
			validateFn: func(t *testing.T, chunks []Chunk) {
				require.NotEmpty(t, chunks)
				lastChunk := chunks[len(chunks)-1]
				require.NotNil(t, lastChunk.Heading)
				assert.Equal(t, "Actual Heading", *lastChunk.Heading)
				assert.Contains(t, lastChunk.Content, "Content for actual heading")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			chunks, err := chunker.Split(tc.input)
			require.NoError(t, err)
			if tc.expectedCount > 0 {
				assert.Len(t, chunks, tc.expectedCount)
			}
			if tc.validateFn != nil {
				tc.validateFn(t, chunks)
			}
		})
	}
}

func TestMarkdownChunker_Determinism(t *testing.T) {
	opts := DefaultChunkerOptions()
	chunker := NewMarkdownChunker(opts)

	doc := `# Title
First paragraph with some text.

Second paragraph with more interesting details about KnowFlow architecture.

## Section Two
Another paragraph under section two.
`

	chunks1, err1 := chunker.Split(doc)
	require.NoError(t, err1)

	chunks2, err2 := chunker.Split(doc)
	require.NoError(t, err2)

	require.Equal(t, len(chunks1), len(chunks2))
	for i := range chunks1 {
		assert.Equal(t, chunks1[i].Index, chunks2[i].Index)
		assert.Equal(t, chunks1[i].Content, chunks2[i].Content)
		assert.Equal(t, chunks1[i].TokenCount, chunks2[i].TokenCount)
		if chunks1[i].Heading == nil {
			assert.Nil(t, chunks2[i].Heading)
		} else {
			require.NotNil(t, chunks2[i].Heading)
			assert.Equal(t, *chunks1[i].Heading, *chunks2[i].Heading)
		}
	}
}

func TestEstimateTokens(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
	assert.Equal(t, 1, EstimateTokens("hi"))
	assert.Equal(t, 2, EstimateTokens("12345678"))
	assert.Equal(t, 25, EstimateTokens(fmt.Sprintf("%100s", "a")))
}
