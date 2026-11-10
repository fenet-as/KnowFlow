package service

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/repo"
	"github.com/google/uuid"
)

// AuthService defines the business logic for native authentication.
type AuthService interface {
	Register(ctx context.Context, req domain.RegisterRequest, userAgent, ip string) (*domain.User, *domain.TokenPair, error)
	Login(ctx context.Context, req domain.LoginRequest, userAgent, ip string) (*domain.User, *domain.TokenPair, error)
	Refresh(ctx context.Context, rawRefreshToken, userAgent, ip string) (*domain.User, *domain.TokenPair, error)
	Logout(ctx context.Context, rawRefreshToken string) error
	GetMe(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, req domain.UpdateProfileRequest) (*domain.User, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, req domain.ChangePasswordRequest) error
}

// DefaultAuthService implements AuthService.
type DefaultAuthService struct {
	userRepo   repo.UserRepo
	jwtManager *auth.JWTManager
}

// NewAuthService constructs an initialized DefaultAuthService.
func NewAuthService(userRepo repo.UserRepo, jwtManager *auth.JWTManager) *DefaultAuthService {
	return &DefaultAuthService{
		userRepo:   userRepo,
		jwtManager: jwtManager,
	}
}

// Register validates user input, creates an account, and issues credentials.
func (s *DefaultAuthService) Register(
	ctx context.Context,
	req domain.RegisterRequest,
	userAgent, ip string,
) (*domain.User, *domain.TokenPair, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, nil, domain.NewValidationError("Invalid email address format.", nil)
	}

	fullName := strings.TrimSpace(req.FullName)
	if len(fullName) < 2 {
		return nil, nil, domain.NewValidationError("Full name must be at least 2 characters.", nil)
	}

	password := req.Password
	if len(password) < 8 {
		return nil, nil, domain.NewValidationError("Password must be at least 8 characters.", nil)
	}

	// Hash password
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, nil, domain.NewInternalError("Failed to hash password", err)
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		FullName:     fullName,
		Role:         "member",
	}

	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return nil, nil, err
	}

	// Mint tokens
	tokens, err := s.issueTokens(ctx, user, userAgent, ip)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

// Login verifies credentials and issues a new session.
func (s *DefaultAuthService) Login(
	ctx context.Context,
	req domain.LoginRequest,
	userAgent, ip string,
) (*domain.User, *domain.TokenPair, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		return nil, nil, domain.NewUnauthorizedError("Invalid email or password.")
	}

	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		// Constant time comparison against dummy hash to prevent timing attacks
		auth.CheckPassword(req.Password, "$2a$12$e8Y6e6qJb8H6q4rU7m3DfeE1N4m9m7J7z9r7D3k9s6X5s3v7s6e5G")
		return nil, nil, domain.NewUnauthorizedError("Invalid email or password.")
	}

	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		return nil, nil, domain.NewUnauthorizedError("Invalid email or password.")
	}

	tokens, err := s.issueTokens(ctx, user, userAgent, ip)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

// Refresh validates and rotates a refresh token.
func (s *DefaultAuthService) Refresh(
	ctx context.Context,
	rawRefreshToken, userAgent, ip string,
) (*domain.User, *domain.TokenPair, error) {
	raw := strings.TrimSpace(rawRefreshToken)
	if raw == "" {
		return nil, nil, domain.NewUnauthorizedError("Refresh token missing.")
	}

	tokenHash := auth.HashRefreshToken(raw)
	stored, err := s.userRepo.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, nil, domain.NewUnauthorizedError("Invalid or expired session.")
	}

	if stored.RevokedAt != nil || stored.ExpiresAt.Before(time.Now()) {
		return nil, nil, domain.NewUnauthorizedError("Session has expired or been revoked.")
	}

	// Revoke the used refresh token (rotation)
	_ = s.userRepo.RevokeRefreshToken(ctx, tokenHash)

	// Fetch user
	user, err := s.userRepo.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return nil, nil, domain.NewUnauthorizedError("User account no longer exists.")
	}

	// Mint new tokens
	tokens, err := s.issueTokens(ctx, user, userAgent, ip)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

// Logout revokes the active refresh token.
func (s *DefaultAuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	raw := strings.TrimSpace(rawRefreshToken)
	if raw == "" {
		return nil
	}
	tokenHash := auth.HashRefreshToken(raw)
	return s.userRepo.RevokeRefreshToken(ctx, tokenHash)
}

// GetMe retrieves the authenticated user's profile.
func (s *DefaultAuthService) GetMe(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetUserByID(ctx, userID)
}

// UpdateProfile updates the user's name and/or email address.
func (s *DefaultAuthService) UpdateProfile(ctx context.Context, userID uuid.UUID, req domain.UpdateProfileRequest) (*domain.User, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, domain.NewValidationError("Invalid email address format.", nil)
	}

	fullName := strings.TrimSpace(req.FullName)
	if len(fullName) < 2 {
		return nil, domain.NewValidationError("Full name must be at least 2 characters.", nil)
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// If email changed, check if new email is already registered by another user
	if !strings.EqualFold(user.Email, email) {
		existing, err := s.userRepo.GetUserByEmail(ctx, email)
		if err == nil && existing != nil && existing.ID != userID {
			return nil, domain.NewConflictError("an account with this email already exists")
		}
	}

	user.Email = email
	user.FullName = fullName
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// ChangePassword verifies the current password and hashes/stores the new password.
func (s *DefaultAuthService) ChangePassword(ctx context.Context, userID uuid.UUID, req domain.ChangePasswordRequest) error {
	if len(req.NewPassword) < 8 {
		return domain.NewValidationError("New password must be at least 8 characters.", nil)
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	if !auth.CheckPassword(req.CurrentPassword, user.PasswordHash) {
		return domain.NewUnauthorizedError("Current password is incorrect.")
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return domain.NewInternalError("Failed to hash new password", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, newHash); err != nil {
		return err
	}

	return nil
}

func (s *DefaultAuthService) issueTokens(
	ctx context.Context,
	user *domain.User,
	userAgent, ip string,
) (*domain.TokenPair, error) {
	accessToken, expiresIn, err := s.jwtManager.GenerateAccessToken(user)
	if err != nil {
		return nil, domain.NewInternalError("Failed to generate access token", err)
	}

	rawRefresh, refreshHash, refreshExpires, err := s.jwtManager.GenerateRefreshToken()
	if err != nil {
		return nil, domain.NewInternalError("Failed to generate refresh token", err)
	}

	record := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: refreshExpires,
		UserAgent: userAgent,
		IPAddress: ip,
		CreatedAt: time.Now(),
	}

	if err := s.userRepo.SaveRefreshToken(ctx, record); err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:      accessToken,
		ExpiresIn:        expiresIn,
		RawRefreshToken:  rawRefresh,
		RefreshTokenHash: refreshHash,
		RefreshExpiresAt: refreshExpires,
	}, nil
}
