package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/admin"
	settingsapp "github.com/yourorg/agent_customer_service/internal/application/settings"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

type SettingsHandler struct {
	settings *settingsapp.Service
}

// GMListPrompts exposes every prompt configuration in the external GM format.
func (h *SettingsHandler) GMListPrompts(c *gin.Context) {
	rows, err := h.settings.ListPrompts(c.Request.Context())
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, rows)
}

// GMUpdatePrompt updates one prompt without altering the native admin API.
func (h *SettingsHandler) GMUpdatePrompt(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := h.settings.UpdatePrompt(c.Request.Context(), c.Param("key"), req.Content)
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, row)
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

func (h *SettingsHandler) ListProviderProfiles(c *gin.Context) {
	rows, err := h.settings.ListProviderProfiles(c.Request.Context(), c.Param("kind"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, rows)
}

func (h *SettingsHandler) CreateProviderProfile(c *gin.Context) {
	var req admin.UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	row, err := h.settings.CreateProviderProfile(c.Request.Context(), c.Param("kind"), req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *SettingsHandler) UpdateProviderProfile(c *gin.Context) {
	var req admin.UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	row, err := h.settings.UpdateProviderProfile(c.Request.Context(), c.Param("kind"), c.Param("id"), req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *SettingsHandler) ActivateProviderProfile(c *gin.Context) {
	if err := h.settings.ActivateProviderProfile(c.Request.Context(), c.Param("kind"), c.Param("id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"updated": true})
}

func (h *SettingsHandler) DeleteProviderProfile(c *gin.Context) {
	if err := h.settings.DeleteProviderProfile(c.Request.Context(), c.Param("kind"), c.Param("id")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
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

func (h *SettingsHandler) GetAgentConfig(c *gin.Context) {
	row, err := h.settings.GetAgentConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *SettingsHandler) UpdateAgentConfig(c *gin.Context) {
	var req admin.UpdateAgentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	row, err := h.settings.UpdateAgentConfig(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *SettingsHandler) ListChatLightRules(c *gin.Context) {
	rows, err := h.settings.ListChatLightRules(c.Request.Context(), c.Query("enabled") == "true")
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, rows)
}

func (h *SettingsHandler) CreateChatLightRule(c *gin.Context) {
	var req admin.UpsertChatLightRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	row, err := h.settings.CreateChatLightRule(c.Request.Context(), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *SettingsHandler) UpdateChatLightRule(c *gin.Context) {
	var req admin.UpsertChatLightRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	row, err := h.settings.UpdateChatLightRule(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *SettingsHandler) DeleteChatLightRule(c *gin.Context) {
	if err := h.settings.DeleteChatLightRule(c.Request.Context(), c.Param("id")); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
