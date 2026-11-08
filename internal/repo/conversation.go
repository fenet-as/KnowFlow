package repo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresConversationRepo implements ConversationRepo using pgxpool.
type PostgresConversationRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresConversationRepo constructs a PostgresConversationRepo.
func NewPostgresConversationRepo(pool *pgxpool.Pool) *PostgresConversationRepo {
	return &PostgresConversationRepo{pool: pool}
}

// Create inserts a new conversation scoped to the authenticated owner.
func (r *PostgresConversationRepo) Create(ctx context.Context, conv *domain.Conversation) error {
	if conv == nil {
		return domain.NewValidationError("conversation must not be nil", nil)
	}
	if strings.TrimSpace(conv.OwnerID) == "" {
		return domain.NewValidationError("owner_id must not be empty", nil)
	}
	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = "New conversation"
	}

	query := `
		INSERT INTO conversations (owner_id, title)
		VALUES ($1, $2)
		RETURNING id, title, created_at, updated_at;
	`

	err := r.pool.QueryRow(ctx, query, conv.OwnerID, title).Scan(
		&conv.ID,
		&conv.Title,
		&conv.CreatedAt,
		&conv.UpdatedAt,
	)
	if err != nil {
		return domain.NewInternalError("failed to create conversation", err)
	}
	return nil
}

// GetByID fetches a single conversation owned by ownerID.
func (r *PostgresConversationRepo) GetByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Conversation, error) {
	query := `
		SELECT id, owner_id, title, created_at, updated_at
		FROM conversations
		WHERE id = $1 AND owner_id = $2;
	`

	conv := &domain.Conversation{}
	err := r.pool.QueryRow(ctx, query, id, ownerID).Scan(
		&conv.ID,
		&conv.OwnerID,
		&conv.Title,
		&conv.CreatedAt,
		&conv.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.NewNotFoundError("Conversation not found.")
		}
		return nil, domain.NewInternalError("failed to query conversation", err)
	}
	return conv, nil
}

// List returns a cursor-paginated list of conversations for ownerID ordered newest first.
func (r *PostgresConversationRepo) List(
	ctx context.Context,
	ownerID string,
	limit int,
	cursorUpdatedAt *time.Time,
	cursorID *uuid.UUID,
) ([]*domain.Conversation, error) {
	query := `
		SELECT id, owner_id, title, created_at, updated_at
		FROM conversations
		WHERE owner_id = $1
		  AND ($2::timestamptz IS NULL OR (updated_at, id) < ($2, $3))
		ORDER BY updated_at DESC, id DESC
		LIMIT $4;
	`

	fetchLimit := limit + 1
	rows, err := r.pool.Query(ctx, query, ownerID, cursorUpdatedAt, cursorID, fetchLimit)
	if err != nil {
		return nil, domain.NewInternalError("failed to list conversations", err)
	}
	defer rows.Close()

	var convs []*domain.Conversation
	for rows.Next() {
		conv := &domain.Conversation{}
		err := rows.Scan(
			&conv.ID,
			&conv.OwnerID,
			&conv.Title,
			&conv.CreatedAt,
			&conv.UpdatedAt,
		)
		if err != nil {
			return nil, domain.NewInternalError("failed to scan conversation row", err)
		}
		convs = append(convs, conv)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.NewInternalError("error iterating conversation rows", err)
	}
	return convs, nil
}

// Delete removes a conversation owned by ownerID. Cascading foreign keys delete its messages.
func (r *PostgresConversationRepo) Delete(ctx context.Context, ownerID string, id uuid.UUID) error {
	query := `DELETE FROM conversations WHERE id = $1 AND owner_id = $2;`
	tag, err := r.pool.Exec(ctx, query, id, ownerID)
	if err != nil {
		return domain.NewInternalError("failed to delete conversation", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NewNotFoundError("Conversation not found.")
	}
	return nil
}

// CreateMessage adds a message to a conversation after verifying owner ownership.
func (r *PostgresConversationRepo) CreateMessage(ctx context.Context, ownerID string, msg *domain.Message) error {
	if msg == nil {
		return domain.NewValidationError("message must not be nil", nil)
	}

	// Verify conversation exists and is owned by ownerID
	var convExists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM conversations WHERE id = $1 AND owner_id = $2);`
	err := r.pool.QueryRow(ctx, checkQuery, msg.ConversationID, ownerID).Scan(&convExists)
	if err != nil {
		return domain.NewInternalError("failed to verify conversation ownership", err)
	}
	if !convExists {
		return domain.NewNotFoundError("Conversation not found.")
	}

	citationsJSON, err := json.Marshal(msg.Citations)
	if err != nil || len(citationsJSON) == 0 {
		citationsJSON = []byte("[]")
	}

	insertSQL := `
		INSERT INTO messages (conversation_id, role, content, citations)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at;
	`
	err = r.pool.QueryRow(ctx, insertSQL, msg.ConversationID, msg.Role, msg.Content, citationsJSON).Scan(
		&msg.ID,
		&msg.CreatedAt,
	)
	if err != nil {
		return domain.NewInternalError("failed to insert message", err)
	}

	// Touch conversation updated_at
	_, _ = r.pool.Exec(ctx, "UPDATE conversations SET updated_at = now() WHERE id = $1;", msg.ConversationID)

	return nil
}

// ListMessages returns messages for a conversation, ordered chronologically (created_at ASC, id ASC).
// Scoped to the authenticated owner.
func (r *PostgresConversationRepo) ListMessages(
	ctx context.Context,
	ownerID string,
	convID uuid.UUID,
	limit int,
	cursorCreatedAt *time.Time,
	cursorID *uuid.UUID,
) ([]*domain.Message, error) {
	// First verify conversation ownership
	var convExists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM conversations WHERE id = $1 AND owner_id = $2);`
	err := r.pool.QueryRow(ctx, checkQuery, convID, ownerID).Scan(&convExists)
	if err != nil {
		return nil, domain.NewInternalError("failed to verify conversation ownership", err)
	}
	if !convExists {
		return nil, domain.NewNotFoundError("Conversation not found.")
	}

	query := `
		SELECT m.id, m.conversation_id, m.role, m.content, m.citations, m.created_at
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE m.conversation_id = $1 AND c.owner_id = $2
		  AND ($3::timestamptz IS NULL OR (m.created_at, m.id) > ($3, $4))
		ORDER BY m.created_at ASC, m.id ASC
		LIMIT $5;
	`

	fetchLimit := limit + 1
	rows, err := r.pool.Query(ctx, query, convID, ownerID, cursorCreatedAt, cursorID, fetchLimit)
	if err != nil {
		return nil, domain.NewInternalError("failed to list messages", err)
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		msg := &domain.Message{}
		var citationsBytes []byte
		err := rows.Scan(
			&msg.ID,
			&msg.ConversationID,
			&msg.Role,
			&msg.Content,
			&citationsBytes,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, domain.NewInternalError("failed to scan message row", err)
		}
		if len(citationsBytes) > 0 {
			var cits any
			if err := json.Unmarshal(citationsBytes, &cits); err == nil {
				msg.Citations = cits
			}
		}
		if msg.Citations == nil {
			msg.Citations = []any{}
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.NewInternalError("error iterating message rows", err)
	}

	return messages, nil
}

// GetWithMessages retrieves a conversation and its messages ordered chronologically.
func (r *PostgresConversationRepo) GetWithMessages(
	ctx context.Context,
	ownerID string,
	id uuid.UUID,
	messageLimit int,
) (*domain.Conversation, []*domain.Message, error) {
	conv, err := r.GetByID(ctx, ownerID, id)
	if err != nil {
		return nil, nil, err
	}

	if messageLimit <= 0 {
		messageLimit = 50
	}

	messages, err := r.ListMessages(ctx, ownerID, id, messageLimit, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	// Trim pagination item if present
	if len(messages) > messageLimit {
		messages = messages[:messageLimit]
	}

	return conv, messages, nil
}

// GetRecentMessages retrieves the most recent `limit` messages in a conversation, returned oldest first.
func (r *PostgresConversationRepo) GetRecentMessages(
	ctx context.Context,
	ownerID string,
	convID uuid.UUID,
	limit int,
) ([]*domain.Message, error) {
	if limit <= 0 {
		limit = 6
	}

	var convExists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM conversations WHERE id = $1 AND owner_id = $2);`
	err := r.pool.QueryRow(ctx, checkQuery, convID, ownerID).Scan(&convExists)
	if err != nil {
		return nil, domain.NewInternalError("failed to verify conversation ownership", err)
	}
	if !convExists {
		return nil, domain.NewNotFoundError("Conversation not found.")
	}

	query := `
		SELECT m.id, m.conversation_id, m.role, m.content, m.citations, m.created_at
		FROM messages m
		JOIN conversations c ON c.id = m.conversation_id
		WHERE m.conversation_id = $1 AND c.owner_id = $2
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT $3;
	`

	rows, err := r.pool.Query(ctx, query, convID, ownerID, limit)
	if err != nil {
		return nil, domain.NewInternalError("failed to query recent messages", err)
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		msg := &domain.Message{}
		var citationsBytes []byte
		err := rows.Scan(
			&msg.ID,
			&msg.ConversationID,
			&msg.Role,
			&msg.Content,
			&citationsBytes,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, domain.NewInternalError("failed to scan message row", err)
		}
		if len(citationsBytes) > 0 {
			var cits any
			if err := json.Unmarshal(citationsBytes, &cits); err == nil {
				msg.Citations = cits
			}
		}
		if msg.Citations == nil {
			msg.Citations = []any{}
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.NewInternalError("error iterating recent message rows", err)
	}

	// Reverse slice so order is chronological (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// SaveTurn transactionally stores both the user question and the assistant response,
// touches or updates the conversation's title/updated_at, and guarantees consistency.
func (r *PostgresConversationRepo) SaveTurn(
	ctx context.Context,
	ownerID string,
	convID uuid.UUID,
	userContent string,
	assistantContent string,
	citations any,
	updateTitleTo string,
) (*domain.Message, *domain.Message, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, domain.NewInternalError("failed to start transaction", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Lock and verify conversation exists and is owned by ownerID
	var existingTitle string
	err = tx.QueryRow(ctx, "SELECT title FROM conversations WHERE id = $1 AND owner_id = $2 FOR UPDATE;", convID, ownerID).Scan(&existingTitle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, domain.NewNotFoundError("Conversation not found.")
		}
		return nil, nil, domain.NewInternalError("failed to lock conversation", err)
	}

	// Update conversation title and updated_at if title needs update
	if updateTitleTo != "" {
		_, err = tx.Exec(ctx, "UPDATE conversations SET title = $1, updated_at = now() WHERE id = $2;", updateTitleTo, convID)
		if err != nil {
			return nil, nil, domain.NewInternalError("failed to update conversation title", err)
		}
	} else {
		_, err = tx.Exec(ctx, "UPDATE conversations SET updated_at = now() WHERE id = $1;", convID)
		if err != nil {
			return nil, nil, domain.NewInternalError("failed to touch conversation updated_at", err)
		}
	}

	// Insert user message
	userMsg := &domain.Message{
		ConversationID: convID,
		Role:           "user",
		Content:        userContent,
		Citations:      []any{},
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, role, content, citations)
		VALUES ($1, $2, $3, '[]'::jsonb)
		RETURNING id, created_at;
	`, convID, userMsg.Role, userMsg.Content).Scan(&userMsg.ID, &userMsg.CreatedAt)
	if err != nil {
		return nil, nil, domain.NewInternalError("failed to insert user message", err)
	}

	// Insert assistant message
	if citations == nil {
		citations = []any{}
	}
	citBytes, err := json.Marshal(citations)
	if err != nil {
		return nil, nil, domain.NewInternalError("failed to serialize citations", err)
	}

	asstMsg := &domain.Message{
		ConversationID: convID,
		Role:           "assistant",
		Content:        assistantContent,
		Citations:      citations,
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO messages (conversation_id, role, content, citations)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING id, created_at;
	`, convID, asstMsg.Role, asstMsg.Content, string(citBytes)).Scan(&asstMsg.ID, &asstMsg.CreatedAt)
	if err != nil {
		return nil, nil, domain.NewInternalError("failed to insert assistant message", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, domain.NewInternalError("failed to commit conversation turn", err)
	}

	return userMsg, asstMsg, nil
}
