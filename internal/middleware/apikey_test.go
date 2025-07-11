package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAPIKeyAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(APIKeyAuthMiddleware("testkey"))
	r.GET("/protected", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Key", "testkey")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal("should allow valid key")
	}
}
