package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/fenet-as/KnowFlow/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ClerkClaims models standard Clerk JWT session claims.
type ClerkClaims struct {
	jwt.RegisteredClaims
	AuthorizedParty string `json:"azp,omitempty"`
}

// ClerkVerifier validates Clerk session tokens against JWKS.
type ClerkVerifier struct {
	issuer            string
	authorizedParties []string
	keyFunc           jwt.Keyfunc
	leeway            time.Duration
}

// ClerkVerifierOptions provides configuration for the Clerk JWT verifier.
type ClerkVerifierOptions struct {
	Issuer            string
	AuthorizedParties []string
	JWKSURL           string
	KeyFunc           jwt.Keyfunc
	Leeway            time.Duration
}

// NewClerkVerifier creates an initialized ClerkVerifier.
func NewClerkVerifier(opts ClerkVerifierOptions) (*ClerkVerifier, error) {
	if opts.Issuer == "" {
		return nil, errors.New("clerk issuer is required")
	}

	keyFunc := opts.KeyFunc
	if keyFunc == nil {
		jwksURL := opts.JWKSURL
		if jwksURL == "" {
			jwksURL = strings.TrimSuffix(opts.Issuer, "/") + "/.well-known/jwks.json"
		}

		kf, err := keyfunc.NewDefault([]string{jwksURL})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize JWKS keyfunc for %s: %w", jwksURL, err)
		}
		keyFunc = kf.Keyfunc
	}

	leeway := opts.Leeway
	if leeway <= 0 {
		leeway = 5 * time.Second
	}

	return &ClerkVerifier{
		issuer:            strings.TrimSuffix(opts.Issuer, "/"),
		authorizedParties: opts.AuthorizedParties,
		keyFunc:           keyFunc,
		leeway:            leeway,
	}, nil
}

// VerifyToken parses and validates a Clerk JWT and returns the owner ID (subject claim).
func (v *ClerkVerifier) VerifyToken(tokenString string) (string, error) {
	parserOpts := []jwt.ParserOption{
		jwt.WithIssuer(v.issuer),
		jwt.WithLeeway(v.leeway),
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}),
	}

	claims := &ClerkClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, v.keyFunc, parserOpts...)
	if err != nil {
		return "", fmt.Errorf("token validation failed: %w", err)
	}

	if !token.Valid {
		return "", errors.New("invalid token")
	}

	if claims.Subject == "" {
		return "", errors.New("token missing subject claim")
	}

	// Validate authorized party (azp) if configured
	if len(v.authorizedParties) > 0 && claims.AuthorizedParty != "" {
		match := false
		for _, ap := range v.authorizedParties {
			if strings.EqualFold(claims.AuthorizedParty, strings.TrimSpace(ap)) {
				match = true
				break
			}
		}
		if !match {
			return "", fmt.Errorf("unauthorized party claim: %s", claims.AuthorizedParty)
		}
	}

	return claims.Subject, nil
}

// Middleware returns a Gin middleware that enforces authentication on protected routes.
// Unauthenticated requests return 401 with standard error envelope without leaking internal details.
func Middleware(verifier TokenVerifier, errorHandler func(c *gin.Context, err error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			errorHandler(c, domain.NewUnauthorizedError("missing authorization header"))
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			errorHandler(c, domain.NewUnauthorizedError("invalid authorization header format"))
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		if tokenStr == "" {
			errorHandler(c, domain.NewUnauthorizedError("empty bearer token"))
			return
		}

		ownerID, err := verifier.VerifyToken(tokenStr)
		if err != nil {
			errorHandler(c, domain.NewUnauthorizedError("invalid or expired token"))
			return
		}

		// Store verified owner ID in standard request context
		ctx := WithOwnerID(c.Request.Context(), ownerID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

