package http

import (
	"net/http"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateConversationRequest payload for creating a new conversation.
type CreateConversationRequest struct {
	Title string `json:"title"`
}

// ConversationResponse DTO for basic conversation metadata.
type ConversationResponse struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MessageResponse DTO for a message turn.
type MessageResponse struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Citations      any       `json:"citations"`
	CreatedAt      time.Time `json:"created_at"`
}

// ConversationDetailResponse DTO for conversation details including initial messages.
type ConversationDetailResponse struct {
	ID        uuid.UUID         `json:"id"`
	Title     string            `json:"title"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Messages  []MessageResponse `json:"messages"`
}

// ConversationListResponse DTO for paginated conversation lists.
type ConversationListResponse struct {
	Items      []ConversationResponse `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}

// MessageListResponse DTO for paginated message lists.
type MessageListResponse struct {
	Items      []MessageResponse `json:"items"`
	NextCursor *string           `json:"next_cursor"`
}

// ConversationHandler exposes HTTP endpoints for chat threads and messages (KF-28).
type ConversationHandler struct {
	convService service.ConversationService
}

// NewConversationHandler constructs a new ConversationHandler.
func NewConversationHandler(convService service.ConversationService) *ConversationHandler {
	return &ConversationHandler{convService: convService}
}

func toConversationResponse(conv *domain.Conversation) ConversationResponse {
	return ConversationResponse{
		ID:        conv.ID,
		Title:     conv.Title,
		CreatedAt: conv.CreatedAt.UTC(),
		UpdatedAt: conv.UpdatedAt.UTC(),
	}
}

func toMessageResponse(msg *domain.Message) MessageResponse {
	citations := msg.Citations
	if citations == nil {
		citations = []any{}
	}
	return MessageResponse{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		Role:           msg.Role,
		Content:        msg.Content,
		Citations:      citations,
		CreatedAt:      msg.CreatedAt.UTC(),
	}
}

// Create handles POST /api/v1/conversations.
func (h *ConversationHandler) Create(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	var req CreateConversationRequest
	// Binding is optional since title can be omitted or empty
	_ = c.ShouldBindJSON(&req)

	conv, err := h.convService.Create(c.Request.Context(), ownerID, req.Title)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toConversationResponse(conv))
}

// List handles GET /api/v1/conversations with keyset pagination.
func (h *ConversationHandler) List(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	limit, cursor, ok := RequirePagination(c)
	if !ok {
		return
	}

	convs, nextCursor, err := h.convService.List(c.Request.Context(), ownerID, limit, cursor)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	items := make([]ConversationResponse, 0, len(convs))
	for _, conv := range convs {
		items = append(items, toConversationResponse(conv))
	}

	c.JSON(http.StatusOK, ConversationListResponse{
		Items:      items,
		NextCursor: nextCursor,
	})
}

// GetByID handles GET /api/v1/conversations/:id.
func (h *ConversationHandler) GetByID(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	convID, ok := RequireUUIDParam(c, "id")
	if !ok {
		return
	}

	limit, _, _ := RequirePagination(c)
	if limit <= 0 {
		limit = 50
	}

	conv, messages, err := h.convService.GetByID(c.Request.Context(), ownerID, convID, limit)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	msgResponses := make([]MessageResponse, 0, len(messages))
	for _, msg := range messages {
		msgResponses = append(msgResponses, toMessageResponse(msg))
	}

	c.JSON(http.StatusOK, ConversationDetailResponse{
		ID:        conv.ID,
		Title:     conv.Title,
		CreatedAt: conv.CreatedAt.UTC(),
		UpdatedAt: conv.UpdatedAt.UTC(),
		Messages:  msgResponses,
	})
}

// Delete handles DELETE /api/v1/conversations/:id.
func (h *ConversationHandler) Delete(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	convID, ok := RequireUUIDParam(c, "id")
	if !ok {
		return
	}

	if err := h.convService.Delete(c.Request.Context(), ownerID, convID); err != nil {
		RespondWithError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListMessages handles GET /api/v1/conversations/:id/messages with keyset pagination.
func (h *ConversationHandler) ListMessages(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	convID, ok := RequireUUIDParam(c, "id")
	if !ok {
		return
	}

	limit, cursor, ok := RequirePagination(c)
	if !ok {
		return
	}

	messages, nextCursor, err := h.convService.ListMessages(c.Request.Context(), ownerID, convID, limit, cursor)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	items := make([]MessageResponse, 0, len(messages))
	for _, msg := range messages {
		items = append(items, toMessageResponse(msg))
	}

	c.JSON(http.StatusOK, MessageListResponse{
		Items:      items,
		NextCursor: nextCursor,
	})
}
