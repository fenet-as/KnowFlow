package embed

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"sync"
)

// FakeEmbedder is a test double that produces deterministic vector embeddings without network I/O.
type FakeEmbedder struct {
	mu        sync.Mutex
	dimension int
	failWith  error
	calls     int
	lastTexts []string
	lastKind  EmbedKind
}

// NewFakeEmbedder creates a FakeEmbedder with the given vector dimension.
func NewFakeEmbedder(dimension int) *FakeEmbedder {
	if dimension <= 0 {
		dimension = 768
	}
	return &FakeEmbedder{
		dimension: dimension,
	}
}

// SetError instructs the fake embedder to return an error on subsequent calls.
func (f *FakeEmbedder) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failWith = err
}

// Dimension returns the configured vector dimension.
func (f *FakeEmbedder) Dimension() int {
	return f.dimension
}

// Calls returns the number of times Embed was called.
func (f *FakeEmbedder) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// LastTexts returns the texts from the most recent Embed invocation.
func (f *FakeEmbedder) LastTexts() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastTexts
}

// LastKind returns the EmbedKind from the most recent Embed invocation.
func (f *FakeEmbedder) LastKind() EmbedKind {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastKind
}

// Embed generates deterministic unit-normalized vectors from text hashes.
func (f *FakeEmbedder) Embed(ctx context.Context, texts []string, kind EmbedKind) ([][]float32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls++
	f.lastTexts = texts
	f.lastKind = kind

	if f.failWith != nil {
		return nil, f.failWith
	}

	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embeddings[i] = generateDeterministicVector(text, f.dimension)
	}

	return embeddings, nil
}

// generateDeterministicVector derives a unit-length vector of length dim from the SHA-256 hash of text.
func generateDeterministicVector(text string, dim int) []float32 {
	vec := make([]float32, dim)
	seed := sha256.Sum256([]byte(text))

	var sumSq float64
	for i := 0; i < dim; i++ {
		byteOffset := (i * 4) % (len(seed) - 4)
		valUint := binary.BigEndian.Uint32(seed[byteOffset : byteOffset+4])
		// Map uint32 to float in [-1.0, 1.0]
		val := (float64(valUint)/float64(math.MaxUint32))*2.0 - 1.0
		vec[i] = float32(val)
		sumSq += val * val
	}

	// Normalize to unit length (L2 norm = 1.0)
	norm := float32(math.Sqrt(sumSq))
	if norm > 0 {
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec
}
