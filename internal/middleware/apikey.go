package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyAuthMiddleware enforces API key authentication for HTTP endpoints
func APIKeyAuthMiddleware(expectedKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.Query("api_key")
		}
		if key == "" && strings.HasPrefix(c.Request.URL.Path, "/metrics") {
			// Allow unauthenticated access to /metrics
			c.Next()
			return
		}
		if key != expectedKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing API key"})
			return
		}
		c.Next()
	}
}
