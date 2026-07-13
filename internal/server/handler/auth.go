package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/auth"
	"github.com/yourorg/agent_customer_service/internal/server/middleware"
	"github.com/yourorg/agent_customer_service/pkg/response"
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
	auth *auth.Service
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

func NewCallerTokenHandler(authSvc *auth.Service) *CallerTokenHandler {
	return &CallerTokenHandler{auth: authSvc}
}

func (h *CallerTokenHandler) Create(c *gin.Context) {
	var req auth.CreateCallerTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	plain, row, err := h.auth.CreateCallerToken(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{
		"id":         row.ID,
		"name":       row.Name,
		"prefix":     row.Prefix,
		"channel_id": row.ChannelID,
		"tenant_id":  row.TenantID,
		"enabled":    row.Enabled,
		"remark":     row.Remark,
		"expires_at": row.ExpiresAt,
		"created_at": row.CreatedAt,
		"app_id":     row.AppID,
		"app_secret": plain,
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
		response.InternalError(c, err.Error())
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
