package embed

import "context"

// EmbedKind distinguishes document chunk embeddings from query embeddings.
type EmbedKind string

const (
	EmbedKindDocument EmbedKind = "document"
	EmbedKindQuery    EmbedKind = "query"
)

// Embedder defines the contract for generating vector embeddings.
type Embedder interface {
	Embed(ctx context.Context, texts []string, kind EmbedKind) ([][]float32, error)
	Dimension() int
}

// EmbedQuery is a convenience helper for embedding a single search query string.
func EmbedQuery(ctx context.Context, embedder Embedder, query string) ([]float32, error) {
	vecs, err := embedder.Embed(ctx, []string{query}, EmbedKindQuery)
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	return vecs[0], nil
}

// EmbedDocuments is a convenience helper for embedding a batch of document texts.
func EmbedDocuments(ctx context.Context, embedder Embedder, texts []string) ([][]float32, error) {
	return embedder.Embed(ctx, texts, EmbedKindDocument)
}
