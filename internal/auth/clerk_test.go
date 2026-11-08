package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fenet-as/KnowFlow/internal/auth"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func generateTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA test key: %v", err)
	}
	return key
}

func createTestToken(t *testing.T, key *rsa.PrivateKey, kid, iss, sub, azp string, exp time.Time) string {
	t.Helper()
	claims := auth.ClerkClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    iss,
			Subject:   sub,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		AuthorizedParty: azp,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return signed
}

func TestClerkVerifier_VerifyToken(t *testing.T) {
	keyA := generateTestRSAKey(t)
	keyB := generateTestRSAKey(t) // used for wrong signature

	issuer := "https://clerk.example.com"
	kidA := "test-key-id-1"

	keyFunc := func(token *jwt.Token) (any, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok || kid != kidA {
			return nil, errors.New("unknown key ID")
		}
		return &keyA.PublicKey, nil
	}

	verifier, err := auth.NewClerkVerifier(auth.ClerkVerifierOptions{
		Issuer:            issuer,
		AuthorizedParties: []string{"http://localhost:5173", "http://localhost:3000"},
		KeyFunc:           keyFunc,
		Leeway:            2 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	t.Run("valid token", func(t *testing.T) {
		tokenStr := createTestToken(t, keyA, kidA, issuer, "user_123", "http://localhost:5173", time.Now().Add(1*time.Hour))
		sub, err := verifier.VerifyToken(tokenStr)
		if err != nil {
			t.Fatalf("expected valid token, got: %v", err)
		}
		if sub != "user_123" {
			t.Errorf("expected sub 'user_123', got %s", sub)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		tokenStr := createTestToken(t, keyA, kidA, issuer, "user_123", "http://localhost:5173", time.Now().Add(-10*time.Minute))
		_, err := verifier.VerifyToken(tokenStr)
		if err == nil {
			t.Fatal("expected expired token error, got nil")
		}
	})

	t.Run("wrong issuer", func(t *testing.T) {
		tokenStr := createTestToken(t, keyA, kidA, "https://wrong-issuer.com", "user_123", "http://localhost:5173", time.Now().Add(1*time.Hour))
		_, err := verifier.VerifyToken(tokenStr)
		if err == nil {
			t.Fatal("expected wrong issuer error, got nil")
		}
	})

	t.Run("wrong signature", func(t *testing.T) {
		// Signed with keyB, but keyFunc returns keyA
		tokenStr := createTestToken(t, keyB, kidA, issuer, "user_123", "http://localhost:5173", time.Now().Add(1*time.Hour))
		_, err := verifier.VerifyToken(tokenStr)
		if err == nil {
			t.Fatal("expected wrong signature error, got nil")
		}
	})

	t.Run("unknown key ID", func(t *testing.T) {
		tokenStr := createTestToken(t, keyA, "unknown-kid-999", issuer, "user_123", "http://localhost:5173", time.Now().Add(1*time.Hour))
		_, err := verifier.VerifyToken(tokenStr)
		if err == nil {
			t.Fatal("expected unknown kid error, got nil")
		}
	})

	t.Run("unauthorized party claim", func(t *testing.T) {
		tokenStr := createTestToken(t, keyA, kidA, issuer, "user_123", "http://malicious-site.com", time.Now().Add(1*time.Hour))
		_, err := verifier.VerifyToken(tokenStr)
		if err == nil {
			t.Fatal("expected unauthorized azp error, got nil")
		}
	})
}

func TestClerkMiddleware(t *testing.T) {
	key := generateTestRSAKey(t)
	issuer := "https://clerk.example.com"
	kid := "test-key"

	keyFunc := func(token *jwt.Token) (any, error) {
		return &key.PublicKey, nil
	}

	verifier, _ := auth.NewClerkVerifier(auth.ClerkVerifierOptions{
		Issuer:  issuer,
		KeyFunc: keyFunc,
	})

	var lastErr error
	mockErrorHandler := func(c *gin.Context, err error) {
		lastErr = err
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}

	r := gin.New()
	r.Use(auth.Middleware(verifier, mockErrorHandler))
	r.GET("/protected", func(c *gin.Context) {
		ownerID := auth.GetOwnerID(c.Request.Context())
		c.String(http.StatusOK, ownerID)
	})

	t.Run("missing header returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
		if !errors.Is(lastErr, domain.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", lastErr)
		}
	})

	t.Run("invalid header format returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Basic username:password")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("empty bearer token returns 401", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer ")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("expired bearer token returns 401", func(t *testing.T) {
		expiredToken := createTestToken(t, key, kid, issuer, "user_expired", "", time.Now().Add(-1*time.Hour))
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("token with invalid signature returns 401", func(t *testing.T) {
		wrongKey := generateTestRSAKey(t)
		invalidToken := createTestToken(t, wrongKey, kid, issuer, "user_impostor", "", time.Now().Add(1*time.Hour))
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+invalidToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("valid bearer token sets owner ID", func(t *testing.T) {
		tokenStr := createTestToken(t, key, kid, issuer, "user_owner_42", "", time.Now().Add(1*time.Hour))
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if w.Body.String() != "user_owner_42" {
			t.Errorf("expected owner ID 'user_owner_42', got %s", w.Body.String())
		}
	})
}

