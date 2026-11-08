package llm

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/fenet-as/KnowFlow/internal/domain"
)

var markerRegex = regexp.MustCompile(`\[(\d+)\]`)

// ExtractSnippet returns the first maxRunes of text, snapped to a word boundary.
func ExtractSnippet(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}

	cutoff := maxRunes
	for i := maxRunes; i > maxRunes-40 && i > 0; i-- {
		if runes[i] == ' ' || runes[i] == '\t' || runes[i] == '\n' {
			cutoff = i
			break
		}
	}

	snippet := strings.TrimSpace(string(runes[:cutoff]))
	return snippet + "..."
}

// ExtractCitations parses [n] citation markers from the raw LLM answer.
// Out-of-range markers are stripped from the answer text.
// If no valid citation markers remain, returns the fixed not-found answer and grounded=false.
func ExtractCitations(rawAnswer string, passages []domain.ChunkMatch) (string, []domain.Citation, bool) {
	if len(passages) == 0 {
		return domain.NoRelevantContextMessage, []domain.Citation{}, false
	}

	validMarkers := make(map[int]bool)
	var orderedMarkers []int

	// Remove invalid markers and collect valid markers in order of appearance
	cleanedAnswer := markerRegex.ReplaceAllStringFunc(rawAnswer, func(m string) string {
		numStr := m[1 : len(m)-1]
		num, err := strconv.Atoi(numStr)
		if err != nil || num < 1 || num > len(passages) {
			// Out of range marker: strip it
			return ""
		}
		if !validMarkers[num] {
			validMarkers[num] = true
			orderedMarkers = append(orderedMarkers, num)
		}
		return m
	})

	cleanedAnswer = strings.TrimSpace(cleanedAnswer)

	// If no valid citation marker is present in the answer
	if len(orderedMarkers) == 0 {
		return domain.NoRelevantContextMessage, []domain.Citation{}, false
	}

	citations := make([]domain.Citation, 0, len(orderedMarkers))
	for _, m := range orderedMarkers {
		targetPassage := passages[m-1]
		citations = append(citations, domain.Citation{
			Marker:       m,
			ChunkID:      targetPassage.ID,
			DocumentID:   targetPassage.DocumentID,
			DocumentName: targetPassage.DocumentName,
			Heading:      targetPassage.Heading,
			Snippet:      ExtractSnippet(targetPassage.Content, 200),
		})
	}

	return cleanedAnswer, citations, true
}
