package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"

	sessionapp "github.com/yourorg/agent_customer_service/internal/application/session"
	"github.com/yourorg/agent_customer_service/internal/server/middleware"
	"github.com/yourorg/agent_customer_service/internal/session"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

type SessionHandler struct {
	service *sessionapp.Service
}

func NewSessionHandler(service *sessionapp.Service) *SessionHandler {
	return &SessionHandler{service: service}
}

type CreateSessionRequest struct {
	CallerUserID string         `json:"caller_user_id"`
	ChannelID    string         `json:"channel_id"`
	TenantID     string         `json:"tenant_id"`
	Meta         map[string]any `json:"meta"`
}

func (h *SessionHandler) Create(c *gin.Context) {
	var req CreateSessionRequest
	_ = c.ShouldBindJSON(&req)
	bizCtx := middleware.GetBizContext(c)
	params := session.CreateParams{
		ExternalUserID: req.CallerUserID,
		ChannelID:      req.ChannelID,
		TenantID:       req.TenantID,
		Meta:           req.Meta,
	}
	if bizCtx != nil {
		if bizCtx.AuthType == "user_token" {
			params.ExternalUserID = bizCtx.CallerUserID
			params.ChannelID = bizCtx.ChannelID
			params.TenantID = bizCtx.TenantID
		} else if params.ExternalUserID == "" {
			params.ExternalUserID = bizCtx.CallerUserID
		}
		if params.ChannelID == "" {
			params.ChannelID = bizCtx.ChannelID
		}
		if params.TenantID == "" {
			params.TenantID = bizCtx.TenantID
		}
		params.CallerTokenID = bizCtx.CallerTokenID
	}
	meta, err := h.service.Create(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, meta)
}

func (h *SessionHandler) Get(c *gin.Context) {
	meta, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, meta)
}

func (h *SessionHandler) History(c *gin.Context) {
	messages, err := h.service.History(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"messages": messages})
}

func (h *SessionHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *SessionHandler) ClearHistory(c *gin.Context) {
	if err := h.service.ClearHistory(c.Request.Context(), c.Param("id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"cleared": true})
}

func (h *SessionHandler) ListByUser(c *gin.Context) {
	page := 1
	size := 20
	if v := c.Query("page"); v != "" {
		fmt.Sscanf(v, "%d", &page)
	}
	if v := c.Query("size"); v != "" {
		fmt.Sscanf(v, "%d", &size)
	}
	callerUserID := c.Param("callerUserID")
	if callerUserID == "" {
		callerUserID = c.Query("caller_user_id")
	}
	if callerUserID == "" {
		response.BadRequest(c, "caller_user_id is required")
		return
	}
	bizCtx := middleware.GetBizContext(c)
	if bizCtx != nil && bizCtx.AuthType == "user_token" {
		if callerUserID != bizCtx.CallerUserID {
			response.NotFound(c, "user sessions not found")
			return
		}
		callerUserID = bizCtx.CallerUserID
	}
	total, items, err := h.service.ListByUser(c.Request.Context(), session.ListQuery{
		ExternalUserID: callerUserID,
		ChannelID:      scopedValue(c.Query("channel_id"), bizCtxValue(bizCtx, "channel"), bizCtx),
		TenantID:       scopedValue(c.Query("tenant_id"), bizCtxValue(bizCtx, "tenant"), bizCtx),
		CallerTokenID:  scopedTokenID(bizCtx),
		Page:           page,
		Size:           size,
	})
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{
		"total":          total,
		"caller_user_id": callerUserID,
		"items":          items,
	})
}

func scopedValue(requested, bound string, biz *middleware.BizContext) string {
	if biz != nil && biz.AuthType == "user_token" {
		return bound
	}
	return firstNonEmpty(requested, bound)
}

func scopedTokenID(biz *middleware.BizContext) string {
	if biz != nil && biz.AuthType == "user_token" {
		return biz.CallerTokenID
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func bizCtxValue(biz *middleware.BizContext, field string) string {
	if biz == nil {
		return ""
	}
	switch field {
	case "channel":
		return biz.ChannelID
	case "tenant":
		return biz.TenantID
	default:
		return ""
	}
}
