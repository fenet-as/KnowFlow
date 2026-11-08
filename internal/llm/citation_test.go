package llm

import (
	"strings"
	"testing"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractCitations(t *testing.T) {
	docID1 := uuid.New()
	chunkID1 := uuid.New()
	heading1 := "Vacation Policy"

	docID2 := uuid.New()
	chunkID2 := uuid.New()

	passages := []domain.ChunkMatch{
		{
			ID:           chunkID1,
			DocumentID:   docID1,
			DocumentName: "handbook.md",
			Heading:      &heading1,
			Content:      "Full-time employees receive 15 days of PTO annually. Accrual begins on day one.",
		},
		{
			ID:           chunkID2,
			DocumentID:   docID2,
			DocumentName: "remote.md",
			Heading:      nil,
			Content:      "Remote work is approved on a case-by-case basis.",
		},
	}

	t.Run("valid markers present produces citations and grounded=true", func(t *testing.T) {
		raw := "Employees accrue 15 days of vacation [1], and remote work requires approval [2]."
		cleaned, citations, grounded := ExtractCitations(raw, passages)

		assert.True(t, grounded)
		assert.Equal(t, raw, cleaned)
		require.Len(t, citations, 2)

		assert.Equal(t, 1, citations[0].Marker)
		assert.Equal(t, chunkID1, citations[0].ChunkID)
		assert.Equal(t, "handbook.md", citations[0].DocumentName)
		assert.Equal(t, "Vacation Policy", *citations[0].Heading)
		assert.Contains(t, citations[0].Snippet, "Full-time employees")

		assert.Equal(t, 2, citations[1].Marker)
		assert.Equal(t, chunkID2, citations[1].ChunkID)
		assert.Equal(t, "remote.md", citations[1].DocumentName)
		assert.Nil(t, citations[1].Heading)
	})

	t.Run("out of range markers are stripped", func(t *testing.T) {
		raw := "Employees get 15 days [1], and there is an alien on Mars [99]!"
		cleaned, citations, grounded := ExtractCitations(raw, passages)

		assert.True(t, grounded)
		assert.NotContains(t, cleaned, "[99]")
		assert.Contains(t, cleaned, "[1]")
		require.Len(t, citations, 1)
		assert.Equal(t, 1, citations[0].Marker)
	})

	t.Run("no valid markers returns fixed not found answer and grounded=false", func(t *testing.T) {
		raw := "I could not find the answer in the provided documents."
		cleaned, citations, grounded := ExtractCitations(raw, passages)

		assert.False(t, grounded)
		assert.Equal(t, domain.NoRelevantContextMessage, cleaned)
		assert.Empty(t, citations)
	})

	t.Run("only invalid markers returns fixed not found answer", func(t *testing.T) {
		raw := "This claim cites non-existent passage [42]."
		cleaned, citations, grounded := ExtractCitations(raw, passages)

		assert.False(t, grounded)
		assert.Equal(t, domain.NoRelevantContextMessage, cleaned)
		assert.Empty(t, citations)
	})
}

func TestExtractSnippet(t *testing.T) {
	short := "Short text."
	assert.Equal(t, short, ExtractSnippet(short, 200))

	long := strings.Repeat("word ", 60) // 300 chars
	snippet := ExtractSnippet(long, 200)
	assert.LessOrEqual(t, len([]rune(snippet)), 203) // 200 + "..."
	assert.True(t, strings.HasSuffix(snippet, "..."))
}
