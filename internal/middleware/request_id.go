package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "X-Request-ID"

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get request ID from header or generate a new one
		requestID := c.GetHeader(requestIDKey)
		if requestID == "" {
			// Generate a random 16-byte ID
			buf := make([]byte, 8)
			rand.Read(buf)
			requestID = hex.EncodeToString(buf)
		}

		// Set the request ID in the context
		c.Set(requestIDKey, requestID)

		// Add the request ID to the response headers
		c.Writer.Header().Set(requestIDKey, requestID)

		// Add to logger context if using a logging middleware
		if logger, exists := c.Get("logger"); exists {
			if logFunc, ok := logger.(func(context.Context) context.Context); ok {
				ctx := logFunc(c.Request.Context())
				c.Request = c.Request.WithContext(ctx)
			}
		}

		c.Next()
	}
}

// GetRequestID returns the request ID from the context
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(requestIDKey); exists {
		if str, ok := id.(string); ok {
			return str
		}
	}
	return ""
}
