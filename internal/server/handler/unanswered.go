package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	knowledgeapp "github.com/yourorg/agent_customer_service/internal/application/knowledge"
	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
	"github.com/yourorg/agent_customer_service/internal/knowledge/unanswered"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

type UnansweredHandler struct {
	service   *unanswered.Service
	knowledge *knowledgeapp.Service
}

func NewUnansweredHandler(service *unanswered.Service, knowledge *knowledgeapp.Service) *UnansweredHandler {
	return &UnansweredHandler{service: service, knowledge: knowledge}
}

func (h *UnansweredHandler) GMList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	total, rows, err := h.service.List(c.Request.Context(), unanswered.ListQuery{Q: c.Query("q"), Status: c.Query("status"), Page: page, Size: size})
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, gin.H{"list": rows, "total": total})
}

func (h *UnansweredHandler) GMUpdate(c *gin.Context) {
	var req unanswered.UpdateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	row, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	gmOK(c, row)
}

func (h *UnansweredHandler) GMConvert(c *gin.Context) {
	row, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		gmError(c, http.StatusNotFound, err.Error())
		return
	}
	if row.Status == unanswered.StatusConverted {
		gmError(c, http.StatusBadRequest, "question is already converted")
		return
	}
	if strings.TrimSpace(row.Answer) == "" {
		gmError(c, http.StatusBadRequest, "answer is required before converting to QA")
		return
	}
	item, err := h.knowledge.Create(c.Request.Context(), qa.CreateRequest{Question: row.Question, Answer: row.Answer, Category: row.Category, Tags: []string(row.Tags)})
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.service.MarkConverted(c.Request.Context(), row.ID, item.ID); err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, gin.H{"question": row, "qa": item})
}

func (h *UnansweredHandler) GMDelete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, gin.H{"deleted": true})
}

func (h *UnansweredHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	total, rows, err := h.service.List(c.Request.Context(), unanswered.ListQuery{Q: c.Query("q"), Status: c.Query("status"), Page: page, Size: size})
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows, "total": total})
}
func (h *UnansweredHandler) Update(c *gin.Context) {
	var req unanswered.UpdateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	row, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, row)
}
func (h *UnansweredHandler) Convert(c *gin.Context) {
	row, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	if row.Status == unanswered.StatusConverted {
		response.BadRequest(c, "question is already converted")
		return
	}
	if strings.TrimSpace(row.Answer) == "" {
		response.BadRequest(c, "answer is required before converting to QA")
		return
	}
	item, err := h.knowledge.Create(c.Request.Context(), qa.CreateRequest{Question: row.Question, Answer: row.Answer, Category: row.Category, Tags: []string(row.Tags)})
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if err := h.service.MarkConverted(c.Request.Context(), row.ID, item.ID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"question": row, "qa": item})
}
func (h *UnansweredHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
