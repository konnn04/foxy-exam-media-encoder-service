package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BearerAuth returns a gin middleware that validates
// the Authorization header against the expected API key.
//
// Expected format: "Authorization: Bearer <api-key>"
// Also accepts:    "X-API-Key: <api-key>"   (for compatibility)
func BearerAuth(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""

		// Try Authorization: Bearer <key>
		auth := c.GetHeader("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}

		// Fallback: X-API-Key header
		if token == "" {
			token = c.GetHeader("X-API-Key")
		}

		if token == "" || token != expectedKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized — invalid or missing API key",
			})
			return
		}

		c.Next()
	}
}
