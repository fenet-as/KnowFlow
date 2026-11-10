package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserRepo struct {
	users         map[string]*domain.User
	usersByID     map[uuid.UUID]*domain.User
	tokens        map[string]*domain.RefreshToken
	createErr     error
	getByEmailErr error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:     make(map[string]*domain.User),
		usersByID: make(map[uuid.UUID]*domain.User),
		tokens:    make(map[string]*domain.RefreshToken),
	}
}

func (m *mockUserRepo) CreateUser(ctx context.Context, u *domain.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	if _, exists := m.users[u.Email]; exists {
		return domain.NewConflictError("email already in use")
	}
	m.users[u.Email] = u
	m.usersByID[u.ID] = u
	return nil
}

func (m *mockUserRepo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.getByEmailErr != nil {
		return nil, m.getByEmailErr
	}
	u, ok := m.users[email]
	if !ok {
		return nil, domain.NewNotFoundError("user not found")
	}
	return u, nil
}

func (m *mockUserRepo) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return nil, domain.NewNotFoundError("user not found")
	}
	return u, nil
}

func (m *mockUserRepo) UpdateUser(ctx context.Context, u *domain.User) error {
	m.users[u.Email] = u
	m.usersByID[u.ID] = u
	return nil
}

func (m *mockUserRepo) UpdatePassword(ctx context.Context, userID uuid.UUID, newPasswordHash string) error {
	if u, ok := m.usersByID[userID]; ok {
		u.PasswordHash = newPasswordHash
		return nil
	}
	return domain.NewNotFoundError("user not found")
}

func (m *mockUserRepo) SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	m.tokens[token.TokenHash] = token
	return nil
}

func (m *mockUserRepo) GetRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	t, ok := m.tokens[tokenHash]
	if !ok {
		return nil, domain.NewNotFoundError("token not found")
	}
	return t, nil
}

func (m *mockUserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	if t, ok := m.tokens[tokenHash]; ok {
		now := time.Now()
		t.RevokedAt = &now
	}
	return nil
}

func (m *mockUserRepo) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	for _, t := range m.tokens {
		if t.UserID == userID {
			t.RevokedAt = &now
		}
	}
	return nil
}

func TestAuthService_Register_Success(t *testing.T) {
	repo := newMockUserRepo()
	jwtMgr := auth.NewJWTManager("test-secret-key-that-is-at-least-32-bytes-long", 15*time.Minute, 7*24*time.Hour)
	authSvc := service.NewAuthService(repo, jwtMgr)

	req := domain.RegisterRequest{
		Email:    "test@example.com",
		Password: "password123",
		FullName: "Test User",
	}

	user, tokens, err := authSvc.Register(context.Background(), req, "UserAgent", "127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "Test User", user.FullName)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RawRefreshToken)
}

func TestAuthService_Register_ValidationErrors(t *testing.T) {
	repo := newMockUserRepo()
	jwtMgr := auth.NewJWTManager("test-secret-key-that-is-at-least-32-bytes-long", 15*time.Minute, 7*24*time.Hour)
	authSvc := service.NewAuthService(repo, jwtMgr)

	// Short password (< 8)
	_, _, err := authSvc.Register(context.Background(), domain.RegisterRequest{
		Email:    "valid@example.com",
		Password: "short",
		FullName: "Name",
	}, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Password must be at least 8 characters")

	// Invalid email
	_, _, err = authSvc.Register(context.Background(), domain.RegisterRequest{
		Email:    "notanemail",
		Password: "password123",
		FullName: "Name",
	}, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid email address format")

	// Short name
	_, _, err = authSvc.Register(context.Background(), domain.RegisterRequest{
		Email:    "valid@example.com",
		Password: "password123",
		FullName: "A",
	}, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Full name must be at least 2 characters")
}

func TestAuthService_Login_And_Refresh_Lifecycle(t *testing.T) {
	repo := newMockUserRepo()
	jwtMgr := auth.NewJWTManager("test-secret-key-that-is-at-least-32-bytes-long", 15*time.Minute, 7*24*time.Hour)
	authSvc := service.NewAuthService(repo, jwtMgr)

	// Register
	_, _, err := authSvc.Register(context.Background(), domain.RegisterRequest{
		Email:    "user@example.com",
		Password: "securePassword123",
		FullName: "Lifecycle User",
	}, "agent", "ip")
	require.NoError(t, err)

	// Login with correct credentials
	user, tokens, err := authSvc.Login(context.Background(), domain.LoginRequest{
		Email:    "user@example.com",
		Password: "securePassword123",
	}, "agent", "ip")
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", user.Email)
	assert.NotEmpty(t, tokens.AccessToken)

	// Login with wrong password
	_, _, err = authSvc.Login(context.Background(), domain.LoginRequest{
		Email:    "user@example.com",
		Password: "wrongPassword",
	}, "agent", "ip")
	assert.Error(t, err)

	// Refresh token rotation
	newUser, newTokens, err := authSvc.Refresh(context.Background(), tokens.RawRefreshToken, "agent", "ip")
	require.NoError(t, err)
	assert.Equal(t, user.ID, newUser.ID)
	assert.NotEmpty(t, newTokens.AccessToken)
	assert.NotEqual(t, tokens.RawRefreshToken, newTokens.RawRefreshToken)

	// Reusing the old refresh token should fail (already rotated)
	_, _, err = authSvc.Refresh(context.Background(), tokens.RawRefreshToken, "agent", "ip")
	assert.Error(t, err)

	// Logout
	err = authSvc.Logout(context.Background(), newTokens.RawRefreshToken)
	require.NoError(t, err)

	// Refreshing after logout should fail
	_, _, err = authSvc.Refresh(context.Background(), newTokens.RawRefreshToken, "agent", "ip")
	assert.Error(t, err)
}

func TestAuthService_UpdateProfile(t *testing.T) {
	repo := newMockUserRepo()
	jwtMgr := auth.NewJWTManager("test-secret-key-that-is-at-least-32-bytes-long", 15*time.Minute, 7*24*time.Hour)
	authSvc := service.NewAuthService(repo, jwtMgr)

	user, _, err := authSvc.Register(context.Background(), domain.RegisterRequest{
		Email:    "original@example.com",
		Password: "password123",
		FullName: "Original Name",
	}, "agent", "ip")
	require.NoError(t, err)

	// Update name and email
	updated, err := authSvc.UpdateProfile(context.Background(), user.ID, domain.UpdateProfileRequest{
		FullName: "Updated Name",
		Email:    "updated@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.FullName)
	assert.Equal(t, "updated@example.com", updated.Email)

	// Validation: Invalid email
	_, err = authSvc.UpdateProfile(context.Background(), user.ID, domain.UpdateProfileRequest{
		FullName: "Name",
		Email:    "not-an-email",
	})
	assert.Error(t, err)

	// Validation: Name too short
	_, err = authSvc.UpdateProfile(context.Background(), user.ID, domain.UpdateProfileRequest{
		FullName: "A",
		Email:    "updated@example.com",
	})
	assert.Error(t, err)
}

func TestAuthService_ChangePassword(t *testing.T) {
	repo := newMockUserRepo()
	jwtMgr := auth.NewJWTManager("test-secret-key-that-is-at-least-32-bytes-long", 15*time.Minute, 7*24*time.Hour)
	authSvc := service.NewAuthService(repo, jwtMgr)

	user, _, err := authSvc.Register(context.Background(), domain.RegisterRequest{
		Email:    "pwdtest@example.com",
		Password: "oldPassword123!",
		FullName: "Password Test",
	}, "agent", "ip")
	require.NoError(t, err)

	// Fail with wrong current password
	err = authSvc.ChangePassword(context.Background(), user.ID, domain.ChangePasswordRequest{
		CurrentPassword: "wrongPassword",
		NewPassword:     "newPassword123!",
	})
	assert.Error(t, err)

	// Fail with short new password
	err = authSvc.ChangePassword(context.Background(), user.ID, domain.ChangePasswordRequest{
		CurrentPassword: "oldPassword123!",
		NewPassword:     "short",
	})
	assert.Error(t, err)

	// Success
	err = authSvc.ChangePassword(context.Background(), user.ID, domain.ChangePasswordRequest{
		CurrentPassword: "oldPassword123!",
		NewPassword:     "newPassword123!",
	})
	require.NoError(t, err)

	// Can login with new password
	_, _, err = authSvc.Login(context.Background(), domain.LoginRequest{
		Email:    "pwdtest@example.com",
		Password: "newPassword123!",
	}, "agent", "ip")
	require.NoError(t, err)
}
