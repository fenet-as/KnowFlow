package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DocumentResponse matches the KnowFlow API contract for a single document.
type DocumentResponse struct {
	ID           uuid.UUID `json:"id"`
	Filename     string    `json:"filename"`
	SizeBytes    int64     `json:"size_bytes"`
	Status       string    `json:"status"`
	ErrorMessage *string   `json:"error_message"`
	ChunkCount   int       `json:"chunk_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// DocumentListResponse matches the KnowFlow API contract for cursor-paginated documents.
type DocumentListResponse struct {
	Items      []DocumentResponse `json:"items"`
	NextCursor *string            `json:"next_cursor"`
}

// DocumentHandler handles document upload, retrieval, listing, and deletion.
type DocumentHandler struct {
	docService     service.DocumentService
	maxUploadBytes int64
}

// NewDocumentHandler creates a new DocumentHandler.
func NewDocumentHandler(docService service.DocumentService, maxUploadBytes int64) *DocumentHandler {
	if maxUploadBytes <= 0 {
		maxUploadBytes = 1048576 // 1 MB default
	}
	return &DocumentHandler{
		docService:     docService,
		maxUploadBytes: maxUploadBytes,
	}
}

// toDocumentResponse maps domain.Document to DocumentResponse DTO.
func toDocumentResponse(doc *domain.Document) DocumentResponse {
	return DocumentResponse{
		ID:           doc.ID,
		Filename:     doc.Filename,
		SizeBytes:    doc.SizeBytes,
		Status:       doc.Status,
		ErrorMessage: doc.ErrorMessage,
		ChunkCount:   doc.ChunkCount,
		CreatedAt:    doc.CreatedAt.UTC(),
		UpdatedAt:    doc.UpdatedAt.UTC(),
	}
}

// Upload handles POST /api/v1/documents multipart uploads.
func (h *DocumentHandler) Upload(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			RespondWithError(c, domain.NewValidationError("Multipart form field 'file' is required.", err))
			return
		}
		RespondWithError(c, domain.NewValidationError(fmt.Sprintf("Failed to read multipart file: %v", err), err))
		return
	}

	// Quick check on declared header size
	if fileHeader.Size > h.maxUploadBytes {
		RespondWithError(c, domain.NewPayloadTooLargeError(fmt.Sprintf("Uploaded file size %d bytes exceeds maximum limit of %d bytes.", fileHeader.Size, h.maxUploadBytes)))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		RespondWithError(c, domain.NewInternalError("Failed to open uploaded file", err))
		return
	}
	defer file.Close()

	// Read content stream with limit to avoid memory exhaustion
	limitReader := io.LimitReader(file, h.maxUploadBytes+1)
	content, err := io.ReadAll(limitReader)
	if err != nil {
		RespondWithError(c, domain.NewInternalError("Failed to read uploaded file contents", err))
		return
	}

	if int64(len(content)) > h.maxUploadBytes {
		RespondWithError(c, domain.NewPayloadTooLargeError(fmt.Sprintf("Uploaded file size exceeds maximum limit of %d bytes.", h.maxUploadBytes)))
		return
	}

	doc, err := h.docService.Upload(c.Request.Context(), ownerID, fileHeader.Filename, content)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	// Returns 202 Accepted on success
	c.JSON(http.StatusAccepted, toDocumentResponse(doc))
}

// List handles GET /api/v1/documents with keyset pagination.
func (h *DocumentHandler) List(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	limit, cursor, ok := RequirePagination(c)
	if !ok {
		return
	}

	docs, nextCursor, err := h.docService.List(c.Request.Context(), ownerID, limit, cursor)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	items := make([]DocumentResponse, 0, len(docs))
	for _, doc := range docs {
		items = append(items, toDocumentResponse(doc))
	}

	c.JSON(http.StatusOK, DocumentListResponse{
		Items:      items,
		NextCursor: nextCursor,
	})
}

// GetByID handles GET /api/v1/documents/:id.
func (h *DocumentHandler) GetByID(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	docID, ok := RequireUUIDParam(c, "id")
	if !ok {
		return
	}

	doc, err := h.docService.GetByID(c.Request.Context(), ownerID, docID)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, toDocumentResponse(doc))
}

// Delete handles DELETE /api/v1/documents/:id.
func (h *DocumentHandler) Delete(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	docID, ok := RequireUUIDParam(c, "id")
	if !ok {
		return
	}

	if err := h.docService.Delete(c.Request.Context(), ownerID, docID); err != nil {
		RespondWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
