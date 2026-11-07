package embed

import "context"

// Embedder defines the contract for generating vector embeddings.
type Embedder interface {
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

