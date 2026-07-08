package middleware

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"github.com/yourorg/agent_customer_service/internal/auth"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

const (
	RequestIDKey  = "request_id"
	ClaimsKey     = "claims"
	BizContextKey = "biz_context"
)

type BizContext struct {
	RequestID      string `json:"request_id,omitempty"`
	CallerTokenID  string `json:"caller_token_id,omitempty"`
	CallerName     string `json:"caller_name,omitempty"`
	CallerUserID   string `json:"caller_user_id,omitempty"`
	ChannelID      string `json:"channel_id,omitempty"`
	TenantID       string `json:"tenant_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	AuthType       string `json:"auth_type,omitempty"`
}

var (
	limiterMu    sync.Mutex
	limiters     = map[string]*rate.Limiter{}
	sessionLocks sync.Map
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
		line := "[HTTP] " + c.Request.Method + " " + c.Request.URL.Path
		line += " status=" + strconv.Itoa(c.Writer.Status())
		line += " request_id=" + c.GetString(RequestIDKey)
		line += " latency=" + time.Since(start).String()
		if biz := GetBizContext(c); biz != nil {
			line += " auth_type=" + biz.AuthType
			line += " caller_token_id=" + biz.CallerTokenID
			line += " caller_user_id=" + biz.CallerUserID
			line += " channel_id=" + biz.ChannelID
			line += " tenant_id=" + biz.TenantID
		}
		log.Print(line)
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID,X-Caller-Token,X-Caller-User-Id,X-Channel-Id,X-Tenant-Id,Idempotency-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
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

func BizAuth(authSvc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		bizCtx := BizContext{
			RequestID:      c.GetString(RequestIDKey),
			CallerUserID:   strings.TrimSpace(c.GetHeader("X-Caller-User-Id")),
			ChannelID:      strings.TrimSpace(c.GetHeader("X-Channel-Id")),
			TenantID:       strings.TrimSpace(c.GetHeader("X-Tenant-Id")),
			IdempotencyKey: strings.TrimSpace(c.GetHeader("Idempotency-Key")),
		}

		if raw := strings.TrimSpace(c.GetHeader("X-Caller-Token")); raw != "" {
			identity, err := authSvc.ValidateCallerToken(c.Request.Context(), raw)
			if err != nil {
				response.Error(c, http.StatusUnauthorized, "invalid caller token")
				c.Abort()
				return
			}
			bizCtx.AuthType = "caller_token"
			bizCtx.CallerTokenID = identity.TokenID
			bizCtx.CallerName = identity.Name
			if bizCtx.ChannelID == "" {
				bizCtx.ChannelID = identity.ChannelID
			}
			if bizCtx.TenantID == "" {
				bizCtx.TenantID = identity.TenantID
			}
			c.Set(BizContextKey, &bizCtx)
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			raw := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			identity, err := authSvc.ValidateCallerToken(c.Request.Context(), raw)
			if err != nil {
				response.Error(c, http.StatusUnauthorized, "invalid caller token")
				c.Abort()
				return
			}
			bizCtx.AuthType = "caller_token"
			bizCtx.CallerTokenID = identity.TokenID
			bizCtx.CallerName = identity.Name
			if bizCtx.ChannelID == "" {
				bizCtx.ChannelID = identity.ChannelID
			}
			if bizCtx.TenantID == "" {
				bizCtx.TenantID = identity.TenantID
			}
			c.Set(BizContextKey, &bizCtx)
			c.Next()
			return
		}
		response.Error(c, http.StatusUnauthorized, "missing caller token")
		c.Abort()
	}
}

func BizRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		biz := GetBizContext(c)
		if biz == nil {
			c.Next()
			return
		}
		allowOrAbort(c, "auth:"+biz.CallerTokenID, 20, 40, "too many requests for caller")
		if c.IsAborted() {
			return
		}
		if biz.CallerUserID != "" {
			allowOrAbort(c, "user:"+biz.CallerUserID, 10, 20, "too many requests for caller user")
			if c.IsAborted() {
				return
			}
		}
		c.Next()
	}
}

func SessionSerial() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")
		if sessionID == "" {
			c.Next()
			return
		}
		lockAny, _ := sessionLocks.LoadOrStore(sessionID, &sync.Mutex{})
		mu := lockAny.(*sync.Mutex)
		mu.Lock()
		defer mu.Unlock()
		c.Next()
	}
}

func allowOrAbort(c *gin.Context, key string, r rate.Limit, burst int, message string) {
	if key == "" {
		return
	}
	limiterMu.Lock()
	limiter, ok := limiters[key]
	if !ok {
		limiter = rate.NewLimiter(r, burst)
		limiters[key] = limiter
	}
	limiterMu.Unlock()
	if !limiter.Allow() {
		response.Error(c, http.StatusTooManyRequests, message)
		c.Abort()
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func GetClaims(c *gin.Context) *auth.Claims {
	v, ok := c.Get(ClaimsKey)
	if !ok {
		return nil
	}
	claims, _ := v.(*auth.Claims)
	return claims
}

func GetBizContext(c *gin.Context) *BizContext {
	v, ok := c.Get(BizContextKey)
	if !ok {
		return nil
	}
	biz, _ := v.(*BizContext)
	return biz
}
