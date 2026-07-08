package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/yourorg/agent_customer_service/internal/auth"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

const (
	RequestIDKey = "request_id"
	ClaimsKey    = "claims"
	APIKeyIDKey  = "api_key_id"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(RequestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("[HTTP] %s %s %d %s %v",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			c.GetString(RequestIDKey),
			time.Since(start),
		)
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID,X-API-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func Deprecate(message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[DEPRECATE] %s %s — %s", c.Request.Method, c.Request.URL.Path, message)
		c.Header("Deprecation", "true")
		c.Header("Warning", `299 - "`+message+`"`)
		c.Next()
	}
}

func JWTAuth(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, "missing bearer token")
			c.Abort()
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := authSvc.ParseToken(token)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid token")
			c.Abort()
			return
		}
		c.Set(ClaimsKey, claims)
		c.Next()
	}
}

func RequireRole(minRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get(ClaimsKey)
		if !ok {
			response.Error(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}
		claims := v.(*auth.Claims)
		if !auth.RoleAtLeast(claims.Role, minRole) {
			response.Error(c, http.StatusForbidden, "forbidden")
			c.Abort()
			return
		}
		c.Next()
	}
}

func APIKeyAuth(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "ApiKey ") {
				key = strings.TrimSpace(strings.TrimPrefix(authHeader, "ApiKey "))
			}
		}
		row, err := authSvc.ValidateAPIKey(c.Request.Context(), key)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid api key")
			c.Abort()
			return
		}
		c.Set(APIKeyIDKey, row.ID)
		c.Next()
	}
}

func GetClaims(c *gin.Context) *auth.Claims {
	v, ok := c.Get(ClaimsKey)
	if !ok {
		return nil
	}
	claims, _ := v.(*auth.Claims)
	return claims
}
