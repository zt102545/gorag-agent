package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/gorag-ai/gorag/internal/admin"
	"github.com/gorag-ai/gorag/pkg/response"
)

type AIErrorHandler struct {
	service *admin.AIErrorService
}

func NewAIErrorHandler(service *admin.AIErrorService) *AIErrorHandler {
	return &AIErrorHandler{service: service}
}

func (h *AIErrorHandler) List(c *gin.Context) {
	rows, err := h.service.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, rows)
}

func (h *AIErrorHandler) UnresolvedCount(c *gin.Context) {
	count, err := h.service.UnresolvedCount(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"count": count})
}

func (h *AIErrorHandler) Resolve(c *gin.Context) {
	if err := h.service.Resolve(c.Request.Context(), c.Param("id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"updated": true})
}
