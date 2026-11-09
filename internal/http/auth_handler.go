package http

import (
	"net/http"
	"strings"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/fenet-as/KnowFlow/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	refreshTokenCookieName = "knowflow_refresh_token"
	refreshTokenPath       = "/api/v1/auth"
)

// AuthHandler exposes HTTP handlers for user authentication.
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler constructs an initialized AuthHandler.
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Register creates a new account, sets an HttpOnly refresh token cookie, and returns the access token.
func (h *AuthHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondWithError(c, domain.NewValidationError("Invalid registration request payload.", err))
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	user, tokens, err := h.authService.Register(c.Request.Context(), req, userAgent, ip)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	h.setRefreshTokenCookie(c, tokens.RawRefreshToken, int(tokens.RefreshExpiresAt.Sub(user.CreatedAt).Seconds()))

	c.JSON(http.StatusCreated, domain.AuthResponse{
		AccessToken: tokens.AccessToken,
		ExpiresIn:   tokens.ExpiresIn,
		User:        user,
	})
}

// Login verifies credentials, sets the refresh token cookie, and returns the access token.
func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondWithError(c, domain.NewValidationError("Invalid login credentials format.", err))
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	user, tokens, err := h.authService.Login(c.Request.Context(), req, userAgent, ip)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	h.setRefreshTokenCookie(c, tokens.RawRefreshToken, int(tokens.RefreshExpiresAt.Sub(user.CreatedAt).Seconds()))

	c.JSON(http.StatusOK, domain.AuthResponse{
		AccessToken: tokens.AccessToken,
		ExpiresIn:   tokens.ExpiresIn,
		User:        user,
	})
}

// Refresh rotates the refresh token read from the HttpOnly cookie and returns a new access token.
func (h *AuthHandler) Refresh(c *gin.Context) {
	rawToken, err := c.Cookie(refreshTokenCookieName)
	if err != nil || strings.TrimSpace(rawToken) == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Session expired or missing refresh token."))
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	user, tokens, err := h.authService.Refresh(c.Request.Context(), rawToken, userAgent, ip)
	if err != nil {
		h.clearRefreshTokenCookie(c)
		RespondWithError(c, err)
		return
	}

	h.setRefreshTokenCookie(c, tokens.RawRefreshToken, int(tokens.RefreshExpiresAt.Sub(user.CreatedAt).Seconds()))

	c.JSON(http.StatusOK, domain.AuthResponse{
		AccessToken: tokens.AccessToken,
		ExpiresIn:   tokens.ExpiresIn,
		User:        user,
	})
}

// Logout revokes the session and clears the refresh token cookie.
func (h *AuthHandler) Logout(c *gin.Context) {
	rawToken, _ := c.Cookie(refreshTokenCookieName)
	if rawToken != "" {
		_ = h.authService.Logout(c.Request.Context(), rawToken)
	}

	h.clearRefreshTokenCookie(c)
	c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
}

// GetMe returns the authenticated user's profile.
func (h *AuthHandler) GetMe(c *gin.Context) {
	ownerID := auth.GetOwnerID(c.Request.Context())
	if ownerID == "" {
		RespondWithError(c, domain.NewUnauthorizedError("Authentication required."))
		return
	}

	uid, err := uuid.Parse(ownerID)
	if err != nil {
		RespondWithError(c, domain.NewUnauthorizedError("Invalid user identity."))
		return
	}

	user, err := h.authService.GetMe(c.Request.Context(), uid)
	if err != nil {
		RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) setRefreshTokenCookie(c *gin.Context, rawToken string, maxAge int) {
	isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		refreshTokenCookieName,
		rawToken,
		maxAge,
		refreshTokenPath,
		"",
		isSecure,
		true, // HttpOnly
	)
}

func (h *AuthHandler) clearRefreshTokenCookie(c *gin.Context) {
	isSecure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		refreshTokenCookieName,
		"",
		-1,
		refreshTokenPath,
		"",
		isSecure,
		true,
	)
}
