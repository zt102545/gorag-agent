package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/admin"
	settingsapp "github.com/yourorg/agent_customer_service/internal/application/settings"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

type SettingsHandler struct {
	settings *settingsapp.Service
}

func NewSettingsHandler(settings *settingsapp.Service) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

func (h *SettingsHandler) GetChatProvider(c *gin.Context) {
	view, err := h.settings.GetProvider(c.Request.Context(), admin.ProviderKindChat)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, view)
}

func (h *SettingsHandler) UpdateChatProvider(c *gin.Context) {
	var req admin.UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	view, err := h.settings.UpdateProvider(c.Request.Context(), admin.ProviderKindChat, req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, view)
}

func (h *SettingsHandler) GetEmbeddingProvider(c *gin.Context) {
	view, err := h.settings.GetProvider(c.Request.Context(), admin.ProviderKindEmbedding)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, view)
}

func (h *SettingsHandler) UpdateEmbeddingProvider(c *gin.Context) {
	var req admin.UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	view, err := h.settings.UpdateProvider(c.Request.Context(), admin.ProviderKindEmbedding, req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, view)
}

func (h *SettingsHandler) ListPrompts(c *gin.Context) {
	rows, err := h.settings.ListPrompts(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, rows)
}

func (h *SettingsHandler) GetPrompt(c *gin.Context) {
	row, err := h.settings.GetPrompt(c.Request.Context(), c.Param("key"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *SettingsHandler) UpdatePrompt(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	row, err := h.settings.UpdatePrompt(c.Request.Context(), c.Param("key"), req.Content)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, row)
}
