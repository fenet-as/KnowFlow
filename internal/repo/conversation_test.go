package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/platform"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresConversationRepo_Lifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dbURL := getTestDatabaseURL()
	poolCfg := platform.DefaultDBPoolConfig()
	pool, err := platform.NewDBPool(ctx, dbURL, poolCfg)
	if err != nil {
		t.Skipf("PostgreSQL not reachable at %s: %v", dbURL, err)
		return
	}
	defer pool.Close()

	convRepo := repo.NewPostgresConversationRepo(pool)

	ownerA := "owner_conv_A_" + uuid.New().String()[:8]
	ownerB := "owner_conv_B_" + uuid.New().String()[:8]

	// 1. Create conversations for Owner A
	conv1 := &domain.Conversation{
		OwnerID: ownerA,
		Title:   "Benefits discussion",
	}
	err = convRepo.Create(ctx, conv1)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, conv1.ID)
	assert.Equal(t, "Benefits discussion", conv1.Title)

	conv2 := &domain.Conversation{
		OwnerID: ownerA,
		Title:   "", // Should default to "New conversation"
	}
	err = convRepo.Create(ctx, conv2)
	require.NoError(t, err)
	assert.Equal(t, "New conversation", conv2.Title)

	// 2. GetByID
	fetched, err := convRepo.GetByID(ctx, ownerA, conv1.ID)
	require.NoError(t, err)
	assert.Equal(t, conv1.ID, fetched.ID)
	assert.Equal(t, "Benefits discussion", fetched.Title)

	// Cross-tenant access returns ErrNotFound
	_, err = convRepo.GetByID(ctx, ownerB, conv1.ID)
	assert.Error(t, err)
	assert.True(t, errorsIsNotFound(err))

	// 3. Create Messages in conv1
	msg1 := &domain.Message{
		ConversationID: conv1.ID,
		Role:           "user",
		Content:        "What are my medical benefits?",
	}
	err = convRepo.CreateMessage(ctx, ownerA, msg1)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, msg1.ID)

	msg2 := &domain.Message{
		ConversationID: conv1.ID,
		Role:           "assistant",
		Content:        "Medical benefits include dental and vision [1].",
		Citations: []map[string]any{
			{"marker": 1, "heading": "Dental"},
		},
	}
	err = convRepo.CreateMessage(ctx, ownerA, msg2)
	require.NoError(t, err)

	// Cross-tenant message insertion fails
	msgAlien := &domain.Message{
		ConversationID: conv1.ID,
		Role:           "user",
		Content:        "Alien attempt",
	}
	err = convRepo.CreateMessage(ctx, ownerB, msgAlien)
	assert.Error(t, err)
	assert.True(t, errorsIsNotFound(err))

	// 4. List Messages (ordered chronologically created_at ASC)
	messages, err := convRepo.ListMessages(ctx, ownerA, conv1.ID, 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "What are my medical benefits?", messages[0].Content)
	assert.Equal(t, "assistant", messages[1].Role)

	// Cross-tenant message listing fails
	_, err = convRepo.ListMessages(ctx, ownerB, conv1.ID, 10, nil, nil)
	assert.Error(t, err)
	assert.True(t, errorsIsNotFound(err))

	// 5. GetWithMessages
	cWithM, mList, err := convRepo.GetWithMessages(ctx, ownerA, conv1.ID, 10)
	require.NoError(t, err)
	assert.Equal(t, conv1.ID, cWithM.ID)
	assert.Len(t, mList, 2)

	// 6. Delete conversation (cascades to messages)
	err = convRepo.Delete(ctx, ownerA, conv1.ID)
	require.NoError(t, err)

	// Verify deleted
	_, err = convRepo.GetByID(ctx, ownerA, conv1.ID)
	assert.Error(t, err)
	assert.True(t, errorsIsNotFound(err))

	// Verify messages deleted by cascade
	var remainingMessages int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM messages WHERE conversation_id = $1", conv1.ID).Scan(&remainingMessages)
	require.NoError(t, err)
	assert.Equal(t, 0, remainingMessages)

	// Clean up conv2
	_ = convRepo.Delete(ctx, ownerA, conv2.ID)
}

func errorsIsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return appErr.Code == domain.CodeNotFound
	}
	return errors.Is(err, domain.ErrNotFound)
}
