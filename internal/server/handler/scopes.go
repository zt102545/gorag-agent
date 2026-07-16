package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	scopesapp "github.com/gorag-ai/gorag/internal/application/scopes"
	settingsapp "github.com/gorag-ai/gorag/internal/application/settings"
)

type ScopeHandler struct {
	service  *scopesapp.Service
	settings *settingsapp.Service
}

func NewScopeHandler(service *scopesapp.Service, settings *settingsapp.Service) *ScopeHandler {
	return &ScopeHandler{service: service, settings: settings}
}

func (h *ScopeHandler) ListPromptGroups(c *gin.Context) {
	rows, err := h.service.ListPromptGroups(c.Request.Context())
	h.respond(c, rows, err)
}
func (h *ScopeHandler) CreatePromptGroup(c *gin.Context) {
	var req scopesapp.UpsertRequest
	if !h.bind(c, &req) {
		return
	}
	row, err := h.service.CreatePromptGroup(c.Request.Context(), req)
	h.respond(c, row, err)
}
func (h *ScopeHandler) UpdatePromptGroup(c *gin.Context) {
	var req scopesapp.UpsertRequest
	if !h.bind(c, &req) {
		return
	}
	row, err := h.service.UpdatePromptGroup(c.Request.Context(), c.Param("id"), req)
	h.respond(c, row, err)
}
func (h *ScopeHandler) CopyPromptGroup(c *gin.Context) {
	var req scopesapp.CopyRequest
	if !h.bind(c, &req) {
		return
	}
	row, err := h.service.CopyPromptGroup(c.Request.Context(), c.Param("id"), req)
	h.respond(c, row, err)
}
func (h *ScopeHandler) DeletePromptGroup(c *gin.Context) {
	h.respond(c, gin.H{"deleted": true}, h.service.DeletePromptGroup(c.Request.Context(), c.Param("id")))
}
func (h *ScopeHandler) ListGroupPrompts(c *gin.Context) {
	rows, err := h.service.ListPrompts(c.Request.Context(), c.Param("id"))
	h.respond(c, rows, err)
}
func (h *ScopeHandler) UpdateGroupPrompt(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if !h.bind(c, &req) {
		return
	}
	row, err := h.service.UpdatePrompt(c.Request.Context(), c.Param("id"), c.Param("key"), req.Content)
	if err == nil && h.settings != nil {
		h.settings.InvalidatePrompt(c.Param("key"))
	}
	h.respond(c, row, err)
}

func (h *ScopeHandler) ListKnowledgeBases(c *gin.Context) {
	rows, err := h.service.ListKnowledgeBases(c.Request.Context())
	h.respond(c, rows, err)
}
func (h *ScopeHandler) CreateKnowledgeBase(c *gin.Context) {
	var req scopesapp.UpsertRequest
	if !h.bind(c, &req) {
		return
	}
	row, err := h.service.CreateKnowledgeBase(c.Request.Context(), req)
	h.respond(c, row, err)
}
func (h *ScopeHandler) UpdateKnowledgeBase(c *gin.Context) {
	var req scopesapp.UpsertRequest
	if !h.bind(c, &req) {
		return
	}
	row, err := h.service.UpdateKnowledgeBase(c.Request.Context(), c.Param("id"), req)
	h.respond(c, row, err)
}
func (h *ScopeHandler) CopyKnowledgeBase(c *gin.Context) {
	var req scopesapp.CopyRequest
	if !h.bind(c, &req) {
		return
	}
	row, err := h.service.CopyKnowledgeBase(c.Request.Context(), c.Param("id"), req)
	h.respond(c, row, err)
}
func (h *ScopeHandler) DeleteKnowledgeBase(c *gin.Context) {
	h.respond(c, gin.H{"deleted": true}, h.service.DeleteKnowledgeBase(c.Request.Context(), c.Param("id")))
}

func (h *ScopeHandler) bind(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

func (h *ScopeHandler) respond(c *gin.Context, data any, err error) {
	if err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	gmOK(c, data)
}
