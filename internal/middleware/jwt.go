package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig holds the config for JWT validation
// (add fields as needed, e.g. public key, secret, audience, issuer, roles, scopes)
type JWTConfig struct {
	Secret       string
	PublicKey    string // PEM encoded, for RS256
	Algorithm    string // e.g. "HS256", "RS256"
	Audience     string
	Issuer       string
	RequireRole  string // Optional: required role claim
	RequireScope string // Optional: required scope claim
}

// JWTMiddleware returns a Gin middleware that validates JWT tokens and claims
func JWTMiddleware(cfg JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			return
		}
		tokenString := strings.TrimPrefix(header, "Bearer ")

		var keyFunc jwt.Keyfunc
		if cfg.Algorithm == "HS256" {
			keyFunc = func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.Secret), nil
			}
		} else if cfg.Algorithm == "RS256" {
			keyFunc = func(token *jwt.Token) (interface{}, error) {
				return jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.PublicKey))
			}
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unsupported JWT algorithm"})
			return
		}

		token, err := jwt.Parse(tokenString, keyFunc)
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// Optionally check claims (aud, iss, role, scope, etc.)
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("jwt_claims", claims)
			if cfg.Audience != "" && claims["aud"] != cfg.Audience {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid audience"})
				return
			}
			if cfg.Issuer != "" && claims["iss"] != cfg.Issuer {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid issuer"})
				return
			}
			if cfg.RequireRole != "" {
				roles, ok := claims["role"]
				if !ok || roles != cfg.RequireRole {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Missing or invalid role claim"})
					return
				}
			}
			if cfg.RequireScope != "" {
				scopes, ok := claims["scope"]
				if !ok || !strings.Contains(scopes.(string), cfg.RequireScope) {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Missing or invalid scope claim"})
					return
				}
			}
			// You can add more claims validation here
		}

		c.Next()
	}
}
