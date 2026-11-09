package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	internalhttp "github.com/fenet-as/KnowFlow/internal/http"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAuthService struct {
	registerFn func(ctx context.Context, req domain.RegisterRequest, userAgent, ip string) (*domain.User, *domain.TokenPair, error)
	loginFn    func(ctx context.Context, req domain.LoginRequest, userAgent, ip string) (*domain.User, *domain.TokenPair, error)
	refreshFn  func(ctx context.Context, rawRefreshToken, userAgent, ip string) (*domain.User, *domain.TokenPair, error)
	logoutFn   func(ctx context.Context, rawRefreshToken string) error
	getMeFn    func(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

func (m *mockAuthService) Register(ctx context.Context, req domain.RegisterRequest, userAgent, ip string) (*domain.User, *domain.TokenPair, error) {
	return m.registerFn(ctx, req, userAgent, ip)
}
func (m *mockAuthService) Login(ctx context.Context, req domain.LoginRequest, userAgent, ip string) (*domain.User, *domain.TokenPair, error) {
	return m.loginFn(ctx, req, userAgent, ip)
}
func (m *mockAuthService) Refresh(ctx context.Context, rawRefreshToken, userAgent, ip string) (*domain.User, *domain.TokenPair, error) {
	return m.refreshFn(ctx, rawRefreshToken, userAgent, ip)
}
func (m *mockAuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	return m.logoutFn(ctx, rawRefreshToken)
}
func (m *mockAuthService) GetMe(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return m.getMeFn(ctx, userID)
}

func setupAuthRouter(svc *mockAuthService, verifier auth.TokenVerifier) *internalhttp.RouterOptions {
	return &internalhttp.RouterOptions{
		Logger:        slog.New(slog.NewJSONHandler(io.Discard, nil)),
		DBChecker:     &mockDBChecker{},
		TokenVerifier: verifier,
		AuthHandler:   internalhttp.NewAuthHandler(svc),
	}
}

func TestAuthHandler_Register_Success(t *testing.T) {
	mockSvc := &mockAuthService{
		registerFn: func(ctx context.Context, req domain.RegisterRequest, userAgent, ip string) (*domain.User, *domain.TokenPair, error) {
			u := &domain.User{
				ID:        uuid.New(),
				Email:     req.Email,
				FullName:  req.FullName,
				Role:      "member",
				CreatedAt: time.Now(),
			}
			tp := &domain.TokenPair{
				AccessToken:     "test.access.token",
				ExpiresIn:       900,
				RawRefreshToken: "test_refresh_token_string",
			}
			return u, tp, nil
		},
	}

	opts := setupAuthRouter(mockSvc, nil)
	r := internalhttp.NewRouter(*opts)

	body, _ := json.Marshal(domain.RegisterRequest{
		Email:    "new@knowflow.dev",
		Password: "password123",
		FullName: "New User",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp domain.AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "new@knowflow.dev", resp.User.Email)
	assert.Equal(t, "test.access.token", resp.AccessToken)
	assert.Contains(t, w.Header().Get("Set-Cookie"), "knowflow_refresh_token=test_refresh_token_string")
	assert.Contains(t, w.Header().Get("Set-Cookie"), "HttpOnly")
}

func TestAuthHandler_Login_Success(t *testing.T) {
	testUserID := uuid.New()
	mockSvc := &mockAuthService{
		loginFn: func(ctx context.Context, req domain.LoginRequest, userAgent, ip string) (*domain.User, *domain.TokenPair, error) {
			return &domain.User{
					ID:       testUserID,
					Email:    req.Email,
					FullName: "Demo User",
					Role:     "member",
				}, &domain.TokenPair{
					AccessToken:     "logged.in.token",
					ExpiresIn:       900,
					RawRefreshToken: "logged_in_refresh",
				}, nil
		},
	}

	opts := setupAuthRouter(mockSvc, nil)
	r := internalhttp.NewRouter(*opts)

	body, _ := json.Marshal(domain.LoginRequest{
		Email:    "demo@knowflow.dev",
		Password: "DemoPassword123!",
	})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp domain.AuthResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "demo@knowflow.dev", resp.User.Email)
	assert.Equal(t, "logged.in.token", resp.AccessToken)
}

func TestAuthHandler_Logout(t *testing.T) {
	loggedOut := false
	mockSvc := &mockAuthService{
		logoutFn: func(ctx context.Context, rawRefreshToken string) error {
			assert.Equal(t, "active_cookie_token", rawRefreshToken)
			loggedOut = true
			return nil
		},
	}

	opts := setupAuthRouter(mockSvc, nil)
	r := internalhttp.NewRouter(*opts)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  "knowflow_refresh_token",
		Value: "active_cookie_token",
	})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, loggedOut)
	// Cookie cleared
	assert.Contains(t, w.Header().Get("Set-Cookie"), "Max-Age=0")
}
