package chunk

// Chunk represents a segmented portion of text.
type Chunk struct {
	Index      int
	Heading    string
	Content    string
	TokenCount int
}

// Chunker defines the text splitting interface.
type Chunker interface {
	Split(content string) ([]Chunk, error)
}

