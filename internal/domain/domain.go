package domain

import (
	"time"

	"github.com/google/uuid"
)

// Document represents an uploaded source document.
type Document struct {
	ID           uuid.UUID  `json:"id"`
	OwnerID      string     `json:"owner_id"`
	Filename     string     `json:"filename"`
	SizeBytes    int64      `json:"size_bytes"`
	ContentHash  string     `json:"content_hash"`
	RawContent   string     `json:"raw_content"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message"`
	ChunkCount   int        `json:"chunk_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Chunk represents a segmented text passage and its vector embedding.
type Chunk struct {
	ID          uuid.UUID  `json:"id"`
	DocumentID  uuid.UUID  `json:"document_id"`
	OwnerID     string     `json:"owner_id"`
	ChunkIndex  int        `json:"chunk_index"`
	Content     string     `json:"content"`
	Heading     *string    `json:"heading"`
	TokenCount  int        `json:"token_count"`
	Embedding   []float32  `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ChunkMatch represents a retrieved chunk with its source document name and similarity score.
type ChunkMatch struct {
	ID           uuid.UUID `json:"id"`
	DocumentID   uuid.UUID `json:"document_id"`
	DocumentName string    `json:"document_name"`
	Heading      *string   `json:"heading"`
	Content      string    `json:"content"`
	Score        float64   `json:"score"`
}

// ChunkDetail represents the full details of a chunk for source inspection.
type ChunkDetail struct {
	ID           uuid.UUID `json:"id"`
	DocumentID   uuid.UUID `json:"document_id"`
	DocumentName string    `json:"document_name"`
	ChunkIndex   int       `json:"chunk_index"`
	Heading      *string   `json:"heading"`
	Content      string    `json:"content"`
}

// Job represents an asynchronous background task.
type Job struct {
	ID          uuid.UUID  `json:"id"`
	DocumentID  uuid.UUID  `json:"document_id"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	RunAfter    time.Time  `json:"run_after"`
	LockedAt    *time.Time `json:"locked_at"`
	LockedBy    *string    `json:"locked_by"`
	LastError   *string    `json:"last_error"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Conversation represents a chat thread owned by a user.
type Conversation struct {
	ID        uuid.UUID `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a user or assistant turn within a conversation.
type Message struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Citations      any       `json:"citations"`
	CreatedAt      time.Time `json:"created_at"`
}
