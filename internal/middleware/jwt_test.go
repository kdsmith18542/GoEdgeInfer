package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func generateHS256Token(secret, aud, iss, role, scope string) string {
	claims := jwt.MapClaims{
		"aud":   aud,
		"iss":   iss,
		"role":  role,
		"scope": scope,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, _ := token.SignedString([]byte(secret))
	return t
}

func TestJWTMiddleware_Basic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Add JWT middleware test logic here
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cfg := JWTConfig{Secret: "mysecret", Algorithm: "HS256", Audience: "aud1", Issuer: "iss1", RequireRole: "admin", RequireScope: "read"}
	r.Use(JWTMiddleware(cfg))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	token := generateHS256Token("mysecret", "aud1", "iss1", "admin", "read write")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestJWTMiddleware_MissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(JWTMiddleware(JWTConfig{Secret: "mysecret", Algorithm: "HS256"}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(JWTMiddleware(JWTConfig{Secret: "mysecret", Algorithm: "HS256"}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTMiddleware_WrongAudience(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cfg := JWTConfig{Secret: "mysecret", Algorithm: "HS256", Audience: "aud2"}
	r.Use(JWTMiddleware(cfg))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	token := generateHS256Token("mysecret", "aud1", "iss1", "admin", "read")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401 for wrong audience, got %d", w.Code)
	}
}

func TestJWTMiddleware_WrongIssuer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cfg := JWTConfig{Secret: "mysecret", Algorithm: "HS256", Issuer: "iss2"}
	r.Use(JWTMiddleware(cfg))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	token := generateHS256Token("mysecret", "aud1", "iss1", "admin", "read")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401 for wrong issuer, got %d", w.Code)
	}
}

func TestJWTMiddleware_WrongRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cfg := JWTConfig{Secret: "mysecret", Algorithm: "HS256", RequireRole: "user"}
	r.Use(JWTMiddleware(cfg))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	token := generateHS256Token("mysecret", "aud1", "iss1", "admin", "read")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("expected 403 for wrong role, got %d", w.Code)
	}
}

func TestJWTMiddleware_WrongScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cfg := JWTConfig{Secret: "mysecret", Algorithm: "HS256", RequireScope: "admin"}
	r.Use(JWTMiddleware(cfg))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	token := generateHS256Token("mysecret", "aud1", "iss1", "admin", "read")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("expected 403 for wrong scope, got %d", w.Code)
	}
}

func TestJWTMiddleware_UnsupportedAlgorithm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	cfg := JWTConfig{Secret: "mysecret", Algorithm: "none"}
	r.Use(JWTMiddleware(cfg))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer sometoken")
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401 for unsupported algorithm, got %d", w.Code)
	}
}
