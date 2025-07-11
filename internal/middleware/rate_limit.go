package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu          sync.Mutex
	lastCheck   time.Time
	rate        float64   // tokens per second
	capacity    float64   // maximum tokens in bucket
	tokens      float64   // current number of tokens
}

// NewRateLimiter creates a new rate limiter with the specified rate and capacity
func NewRateLimiter(ratePerSecond float64, capacity int) *RateLimiter {
	return &RateLimiter{
		lastCheck: time.Now(),
		rate:     ratePerSecond,
		capacity: float64(capacity),
		tokens:   float64(capacity),
	}
}

// Allow checks if a request is allowed or rate limited
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastCheck).Seconds()
	rl.lastCheck = now

	// Add tokens based on elapsed time
	rl.tokens += elapsed * rl.rate

	// Cap tokens at capacity
	if rl.tokens > rl.capacity {
		rl.tokens = rl.capacity
	}

	// Check if we have enough tokens
	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}

	return false
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(ratePerSecond float64, capacity int) gin.HandlerFunc {
	limiter := NewRateLimiter(ratePerSecond, capacity)

	return func(c *gin.Context) {
		// Skip rate limiting for health check and metrics endpoints
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests, please try again later",
			})
			return
		}

		c.Next()
	}
}
