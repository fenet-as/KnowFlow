package http

import (
	"net/http"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/gin-gonic/gin"
)

// ChunkHandler handles chunk retrieval requests.
type ChunkHandler struct {
	chunkRepo repo.ChunkRepo
}

// NewChunkHandler constructs a new ChunkHandler.
func NewChunkHandler(chunkRepo repo.ChunkRepo) *ChunkHandler {
	return &ChunkHandler{chunkRepo: chunkRepo}
}

// GetByID handles GET /api/v1/chunks/:id.
// It returns the full passage, source document name, and heading, scoped to the authenticated owner.
func (h *ChunkHandler) GetByID(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	chunkID, ok := RequireUUIDParam(c, "id")
	if !ok {
		return
	}

	detail, err := h.chunkRepo.GetByID(c.Request.Context(), ownerID, chunkID)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, detail)
}
