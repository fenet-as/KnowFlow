package llm

import "context"

// LLM defines the generative model contract for answer synthesis.
type LLM interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

