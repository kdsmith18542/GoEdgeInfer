package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(2, 2) // 2 tokens/sec, capacity 2
	if !rl.Allow() {
		t.Error("expected first request to be allowed")
	}
	if !rl.Allow() {
		t.Error("expected second request to be allowed")
	}
	if rl.Allow() {
		t.Error("expected third request to be rate limited")
	}
	time.Sleep(600 * time.Millisecond)
	if !rl.Allow() {
		t.Error("expected request to be allowed after token refill")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimitMiddleware(1, 1))
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != 429 {
		t.Errorf("expected 429 Too Many Requests, got %d", w2.Code)
	}

	// Health and metrics endpoints should not be rate limited
	r.GET("/health", func(c *gin.Context) { c.String(200, "healthy") })
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w3, req3)
	if w3.Code != 200 {
		t.Errorf("expected 200 for /health, got %d", w3.Code)
	}

	r.GET("/metrics", func(c *gin.Context) { c.String(200, "metrics") })
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("GET", "/metrics", nil)
	r.ServeHTTP(w4, req4)
	if w4.Code != 200 {
		t.Errorf("expected 200 for /metrics, got %d", w4.Code)
	}
}
