package http

import (
	"net/http"
	"strings"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AskRequest defines the payload structure for POST /api/v1/ask.
type AskRequest struct {
	Question    string      `json:"question"`
	DocumentIDs []uuid.UUID `json:"document_ids,omitempty"`
}

// RAGHandler handles stateless question answering with citations.
type RAGHandler struct {
	ragService service.RAGService
}

// NewRAGHandler creates an initialized RAGHandler.
func NewRAGHandler(ragService service.RAGService) *RAGHandler {
	return &RAGHandler{
		ragService: ragService,
	}
}

// Ask handles POST /api/v1/ask.
func (h *RAGHandler) Ask(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	var req AskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondWithError(c, domain.NewValidationError("Invalid JSON request body.", err))
		return
	}

	if strings.TrimSpace(req.Question) == "" {
		RespondWithError(c, domain.NewValidationError("Field 'question' is required and cannot be empty.", nil))
		return
	}

	answer, err := h.ragService.Ask(c.Request.Context(), ownerID, req.Question, req.DocumentIDs)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	// Always ensure citations is serialized as an array, not null
	if answer.Citations == nil {
		answer.Citations = []domain.Citation{}
	}

	c.JSON(http.StatusOK, answer)
}
