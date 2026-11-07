package domain

import "time"

// Document represents an uploaded source document.
type Document struct {
	ID           string     `json:"id"`
	OwnerID      string     `json:"owner_id"`
	Filename     string     `json:"filename"`
	SizeBytes    int64      `json:"size_bytes"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message"`
	ChunkCount   int        `json:"chunk_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

