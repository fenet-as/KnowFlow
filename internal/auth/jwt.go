package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// UserClaims defines JWT claims for KnowFlow access tokens.
type UserClaims struct {
	jwt.RegisteredClaims
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

// TokenVerifier defines the interface for validating Bearer tokens.
type TokenVerifier interface {
	VerifyToken(tokenStr string) (string, error)
}

// ChainedVerifier validates tokens across multiple verifiers in sequence.
type ChainedVerifier struct {
	verifiers []TokenVerifier
}

// NewChainedVerifier creates a combined verifier attempting each verifier in order.
func NewChainedVerifier(verifiers ...TokenVerifier) TokenVerifier {
	var nonNil []TokenVerifier
	for _, v := range verifiers {
		if v != nil {
			nonNil = append(nonNil, v)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	if len(nonNil) == 1 {
		return nonNil[0]
	}
	return &ChainedVerifier{verifiers: nonNil}
}

// VerifyToken iterates over registered verifiers until one succeeds.
func (c *ChainedVerifier) VerifyToken(tokenStr string) (string, error) {
	var lastErr error
	for _, v := range c.verifiers {
		ownerID, err := v.VerifyToken(tokenStr)
		if err == nil {
			return ownerID, nil
		}
		lastErr = err
	}
	return "", lastErr
}

// JWTManager handles minting and validating native KnowFlow tokens.
type JWTManager struct {
	secret          []byte
	accessDuration  time.Duration
	refreshDuration time.Duration
}

// NewJWTManager creates an initialized JWTManager.
func NewJWTManager(secret string, accessDuration, refreshDuration time.Duration) *JWTManager {
	if accessDuration <= 0 {
		accessDuration = 15 * time.Minute
	}
	if refreshDuration <= 0 {
		refreshDuration = 7 * 24 * time.Hour // 7 days
	}
	return &JWTManager{
		secret:          []byte(secret),
		accessDuration:  accessDuration,
		refreshDuration: refreshDuration,
	}
}

// GenerateAccessToken signs a new short-lived JWT for the user.
func (m *JWTManager) GenerateAccessToken(user *domain.User) (string, int64, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessDuration)

	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "knowflow-auth",
		},
		Email:    user.Email,
		FullName: user.FullName,
		Role:     user.Role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", 0, fmt.Errorf("failed to sign access token: %w", err)
	}

	return signed, int64(m.accessDuration.Seconds()), nil
}

// GenerateRefreshToken creates a cryptographic random string and its SHA-256 hash for storage.
func (m *JWTManager) GenerateRefreshToken() (string, string, time.Time, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to generate random token: %w", err)
	}

	rawToken := hex.EncodeToString(bytes)
	tokenHash := HashRefreshToken(rawToken)
	expiresAt := time.Now().Add(m.refreshDuration)

	return rawToken, tokenHash, expiresAt, nil
}

// HashRefreshToken calculates the SHA-256 hash of a raw refresh token string.
func HashRefreshToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

// VerifyToken validates the access token and returns the owner/user ID (implements TokenVerifier).
func (m *JWTManager) VerifyToken(tokenStr string) (string, error) {
	claims, err := m.ParseClaims(tokenStr)
	if err != nil {
		return "", err
	}
	return claims.Subject, nil
}

// ParseClaims decodes and verifies a JWT token string.
func (m *JWTManager) ParseClaims(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, errors.New("invalid user ID in token subject")
	}

	return claims, nil
}
