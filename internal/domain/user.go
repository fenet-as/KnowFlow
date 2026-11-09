package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents an authenticated identity in KnowFlow.
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RefreshToken stores metadata for long-lived session rotation.
type RefreshToken struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	UserAgent string     `json:"user_agent,omitempty"`
	IPAddress string     `json:"ip_address,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// RegisterRequest holds registration payload.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required,min=2"`
}

// LoginRequest holds credentials for sign-in.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse holds access token and user profile returned to the frontend.
type AuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"` // Seconds until expiration
	User        *User  `json:"user"`
}

// TokenPair contains generated access token and raw refresh token string.
type TokenPair struct {
	AccessToken      string
	ExpiresIn        int64
	RawRefreshToken  string
	RefreshTokenHash string
	RefreshExpiresAt time.Time
}
