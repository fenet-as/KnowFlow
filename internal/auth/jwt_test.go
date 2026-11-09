package auth

import (
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordHashing(t *testing.T) {
	password := "SecretPassword123!"

	hash, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)

	assert.True(t, CheckPassword(password, hash))
	assert.False(t, CheckPassword("WrongPassword", hash))
}

func TestJWTManager_GenerateAndVerify(t *testing.T) {
	mgr := NewJWTManager("super-secret-test-key-minimum-32-chars-long", 15*time.Minute, 7*24*time.Hour)

	user := &domain.User{
		ID:       uuid.New(),
		Email:    "test@knowflow.dev",
		FullName: "Test User",
		Role:     "member",
	}

	token, exp, err := mgr.GenerateAccessToken(user)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, int64(900), exp)

	// Verify
	ownerID, err := mgr.VerifyToken(token)
	require.NoError(t, err)
	assert.Equal(t, user.ID.String(), ownerID)

	// Claims
	claims, err := mgr.ParseClaims(token)
	require.NoError(t, err)
	assert.Equal(t, user.Email, claims.Email)
	assert.Equal(t, user.FullName, claims.FullName)
}

func TestJWTManager_InvalidToken(t *testing.T) {
	mgr := NewJWTManager("super-secret-test-key-minimum-32-chars-long", 15*time.Minute, 7*24*time.Hour)

	_, err := mgr.VerifyToken("invalid.token.here")
	assert.Error(t, err)

	otherMgr := NewJWTManager("completely-different-signing-key-32-chars", 15*time.Minute, 7*24*time.Hour)
	user := &domain.User{ID: uuid.New(), Email: "a@b.com", FullName: "A", Role: "member"}
	otherToken, _, _ := otherMgr.GenerateAccessToken(user)

	_, err = mgr.VerifyToken(otherToken)
	assert.Error(t, err)
}

func TestJWTManager_RefreshToken(t *testing.T) {
	mgr := NewJWTManager("super-secret-test-key-minimum-32-chars-long", 15*time.Minute, 7*24*time.Hour)

	raw, hash, exp, err := mgr.GenerateRefreshToken()
	require.NoError(t, err)
	assert.Len(t, raw, 64) // 32 bytes hex encoded
	assert.NotEmpty(t, hash)
	assert.True(t, exp.After(time.Now().Add(6*24*time.Hour)))

	// Deterministic hash check
	assert.Equal(t, hash, HashRefreshToken(raw))
}
