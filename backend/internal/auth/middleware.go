package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthRequired is a Gin middleware that requires a valid JWT.
// It extracts the token from the Authorization header (Bearer scheme)
// and validates it. On success, it stores the claims in the context.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing or invalid authorization header",
			})
			return
		}

		claims, err := ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		// Store claims for downstream handlers
		c.Set(string(ClaimsKey), claims)
		c.Set(string(UserIDKey), claims.UserID)
		c.Next()
	}
}

// OptionalAuth is like AuthRequired but doesn't abort on missing token.
// Use this for endpoints that work for both logged-in and anonymous users,
// where logged-in users get extra features (e.g., saving history).
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" {
			// No token, proceed as anonymous
			c.Next()
			return
		}

		claims, err := ValidateToken(token)
		if err != nil {
			// Invalid token, but we don't abort - just proceed as anonymous
			c.Next()
			return
		}

		c.Set(string(ClaimsKey), claims)
		c.Set(string(UserIDKey), claims.UserID)
		c.Next()
	}
}

// AdminRequired is a middleware that requires both authentication
// and admin role. Use this for admin-only endpoints.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing or invalid authorization header",
			})
			return
		}

		claims, err := ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		if claims.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "admin access required",
			})
			return
		}

		c.Set(string(ClaimsKey), claims)
		c.Set(string(UserIDKey), claims.UserID)
		c.Next()
	}
}

// extractBearerToken pulls the JWT from the Authorization header.
// Returns empty string if not found or malformed.
func extractBearerToken(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		return ""
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return parts[1]
}