package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gorag-ai/gorag/internal/auth"
	"github.com/gorag-ai/gorag/internal/server/middleware"
	"github.com/gorag-ai/gorag/pkg/response"
)

type AuthHandler struct {
	auth *auth.Service
}

func NewAuthHandler(authSvc *auth.Service) *AuthHandler {
	return &AuthHandler{auth: authSvc}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	token, user, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}
	response.OK(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	claims := middleware.GetClaims(c)
	if claims == nil {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := h.auth.GetUser(c.Request.Context(), claims.UserID)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}
	response.OK(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}

type UserHandler struct {
	auth *auth.Service
}

func NewUserHandler(authSvc *auth.Service) *UserHandler {
	return &UserHandler{auth: authSvc}
}

func (h *UserHandler) List(c *gin.Context) {
	users, err := h.auth.ListUsers(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, users)
}

func (h *UserHandler) Create(c *gin.Context) {
	var req auth.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	user, err := h.auth.CreateUser(c.Request.Context(), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, user)
}

func (h *UserHandler) Update(c *gin.Context) {
	var req auth.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	user, err := h.auth.UpdateUser(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, user)
}

func (h *UserHandler) Delete(c *gin.Context) {
	if err := h.auth.DeleteUser(c.Request.Context(), c.Param("id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

type CallerTokenHandler struct {
	auth          *auth.Service
	publicLimiter *middleware.PublicIPLimiter
}

func (h *CallerTokenHandler) IssueUserToken(c *gin.Context) {
	var req auth.IssueUserTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	token, expiresAt, err := h.auth.IssueUserToken(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}
	response.OK(c, gin.H{"access_token": token, "token_type": "Bearer", "expires_in": int(time.Until(expiresAt).Seconds()), "expires_at": expiresAt})
}

func (h *CallerTokenHandler) IssuePublicUserToken(c *gin.Context) {
	var req auth.IssuePublicUserTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	preLimit, err := h.publicLimiter.Allow(c.Request.Context(), "__public-token__", c.ClientIP(), 120, 10000)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "public widget limiter unavailable")
		return
	}
	if !preLimit.Allowed {
		writePublicLimitError(c, preLimit)
		return
	}
	token, expiresAt, policy, err := h.auth.IssuePublicUserToken(c.Request.Context(), req, c.GetHeader("Origin"))
	if err != nil {
		response.Error(c, http.StatusForbidden, err.Error())
		return
	}
	limit, err := h.publicLimiter.Allow(c.Request.Context(), policy.ChannelID, c.ClientIP(), policy.IPRatePerMinute, policy.IPDailyQuota)
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "public widget limiter unavailable")
		return
	}
	if !limit.Allowed {
		writePublicLimitError(c, limit)
		return
	}
	c.Header("X-RateLimit-Minute-Limit", fmt.Sprintf("%d", policy.IPRatePerMinute))
	c.Header("X-RateLimit-Daily-Limit", fmt.Sprintf("%d", policy.IPDailyQuota))
	response.OK(c, gin.H{"access_token": token, "token_type": "Bearer", "expires_in": int(time.Until(expiresAt).Seconds()), "expires_at": expiresAt})
}

func writePublicLimitError(c *gin.Context, limit middleware.PublicLimitResult) {
	c.Header("Retry-After", fmt.Sprintf("%d", limit.RetryAfter))
	message := "public widget IP rate limit exceeded"
	if limit.DailyLimited {
		message = "public widget IP daily quota exceeded"
	}
	response.Error(c, http.StatusTooManyRequests, message)
}

func NewCallerTokenHandler(authSvc *auth.Service, publicLimiter *middleware.PublicIPLimiter) *CallerTokenHandler {
	return &CallerTokenHandler{auth: authSvc, publicLimiter: publicLimiter}
}

func (h *CallerTokenHandler) Create(c *gin.Context) {
	var req auth.CreateCallerTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	plain, row, err := h.auth.CreateCallerToken(c.Request.Context(), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{
		"id":                 row.ID,
		"name":               row.Name,
		"prefix":             row.Prefix,
		"channel_id":         row.ChannelID,
		"tenant_id":          row.TenantID,
		"enabled":            row.Enabled,
		"remark":             row.Remark,
		"expires_at":         row.ExpiresAt,
		"created_at":         row.CreatedAt,
		"app_id":             row.AppID,
		"app_secret":         plain,
		"public_access":      row.PublicAccess,
		"allowed_origins":    row.AllowedOrigins,
		"ip_rate_per_minute": row.IPRatePerMinute,
		"ip_daily_quota":     row.IPDailyQuota,
		"prompt_group_id":    row.PromptGroupID,
		"knowledge_base_id":  row.KnowledgeBaseID,
	})
}

func (h *CallerTokenHandler) List(c *gin.Context) {
	rows, err := h.auth.ListCallerTokens(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, rows)
}

func (h *CallerTokenHandler) Update(c *gin.Context) {
	var req auth.UpdateCallerTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.auth.UpdateCallerToken(c.Request.Context(), c.Param("id"), req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"updated": true})
}

func (h *CallerTokenHandler) Delete(c *gin.Context) {
	if err := h.auth.DeleteCallerToken(c.Request.Context(), c.Param("id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
