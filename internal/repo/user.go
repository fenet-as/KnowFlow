package repo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepo defines the storage interface for users and refresh sessions.
type UserRepo interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
}

// DefaultUserRepo implements UserRepo using pgxpool.
type DefaultUserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo constructs an initialized DefaultUserRepo.
func NewUserRepo(pool *pgxpool.Pool) *DefaultUserRepo {
	return &DefaultUserRepo{pool: pool}
}

// CreateUser inserts a new user record.
func (r *DefaultUserRepo) CreateUser(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, full_name, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))

	_, err := r.pool.Exec(
		ctx,
		query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.FullName,
		user.Role,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_key") || strings.Contains(err.Error(), "unique constraint") {
			return domain.NewConflictError("an account with this email already exists")
		}
		return domain.NewInternalError("failed to create user", err)
	}
	return nil
}

// GetUserByEmail queries a user by lowercased email.
func (r *DefaultUserRepo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, full_name, role, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var u domain.User
	err := r.pool.QueryRow(ctx, query, strings.TrimSpace(email)).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.NewNotFoundError("user not found")
		}
		return nil, domain.NewInternalError("failed to get user by email", err)
	}
	return &u, nil
}

// GetUserByID queries a user by UUID.
func (r *DefaultUserRepo) GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, full_name, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u domain.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.Role,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.NewNotFoundError("user not found")
		}
		return nil, domain.NewInternalError("failed to get user by ID", err)
	}
	return &u, nil
}

// SaveRefreshToken stores a new hashed refresh token.
func (r *DefaultUserRepo) SaveRefreshToken(ctx context.Context, token *domain.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked_at, user_agent, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now()
	}

	_, err := r.pool.Exec(
		ctx,
		query,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.RevokedAt,
		token.UserAgent,
		token.IPAddress,
		token.CreatedAt,
	)
	if err != nil {
		return domain.NewInternalError("failed to save refresh token", err)
	}
	return nil
}

// GetRefreshToken retrieves a token by its SHA-256 hash.
func (r *DefaultUserRepo) GetRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, revoked_at, user_agent, ip_address, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var t domain.RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&t.ID,
		&t.UserID,
		&t.TokenHash,
		&t.ExpiresAt,
		&t.RevokedAt,
		&t.UserAgent,
		&t.IPAddress,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.NewNotFoundError("refresh token not found")
		}
		return nil, domain.NewInternalError("failed to get refresh token", err)
	}
	return &t, nil
}

// RevokeRefreshToken marks a single refresh token as revoked.
func (r *DefaultUserRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`
	_, err := r.pool.Exec(ctx, query, tokenHash)
	if err != nil {
		return domain.NewInternalError("failed to revoke refresh token", err)
	}
	return nil
}

// RevokeAllUserTokens marks all active refresh tokens for a user as revoked (e.g. password change / logout all).
func (r *DefaultUserRepo) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL
	`
	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return domain.NewInternalError("failed to revoke user tokens", err)
	}
	return nil
}
