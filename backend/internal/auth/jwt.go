// Package auth provides authentication utilities: JWT signing/validation,
// password hashing, and Gin middleware for route protection.
package auth

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWT-related configuration. In production, JWT_SECRET should be set
// via environment variable; this package reads it at startup.
var (
	jwtSecret     []byte
	jwtExpiry     = 24 * time.Hour // token valid for 24h
	ErrNoSecret   = errors.New("JWT_SECRET not configured")
	ErrInvalidToken = errors.New("invalid or expired token")
)

// InitJWT sets the secret key for signing tokens. Must be called once
// during server startup before any auth middleware runs.
func InitJWT(secret string) {
	jwtSecret = []byte(secret)
}

// Claims embeds standard JWT claims plus our custom fields.
type Claims struct {
	UserID uint   `json:"uid"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// SignToken creates a signed JWT for the given user.
func SignToken(userID uint, email, role string) (string, error) {
	if len(jwtSecret) == 0 {
		return "", ErrNoSecret
	}

	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "predictdestiny",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateToken parses and validates a JWT string, returning the claims
// if valid, or an error otherwise.
func ValidateToken(tokenStr string) (*Claims, error) {
	if len(jwtSecret) == 0 {
		return nil, ErrNoSecret
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// Context keys for storing auth info in gin.Context.
type ctxKey string

const (
	ClaimsKey ctxKey = "claims"
	UserIDKey ctxKey = "userID"
)

// GetClaims extracts claims from gin.Context. Returns nil if not present.
func GetClaims(c *gin.Context) *Claims {
	if v, exists := c.Get(string(ClaimsKey)); exists {
		if claims, ok := v.(*Claims); ok {
			return claims
		}
	}
	return nil
}

// GetUserID extracts user ID from gin.Context. Returns 0 if not authenticated.
func GetUserID(c *gin.Context) uint {
	if claims := GetClaims(c); claims != nil {
		return claims.UserID
	}
	return 0
}