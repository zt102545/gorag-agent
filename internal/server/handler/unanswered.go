package handler

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	knowledgeapp "github.com/yourorg/agent_customer_service/internal/application/knowledge"
	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
	"github.com/yourorg/agent_customer_service/internal/knowledge/unanswered"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

type UnansweredHandler struct {
	service   *unanswered.Service
	knowledge *knowledgeapp.Service
	convertMu sync.Mutex
}

func NewUnansweredHandler(service *unanswered.Service, knowledge *knowledgeapp.Service) *UnansweredHandler {
	return &UnansweredHandler{service: service, knowledge: knowledge}
}

func (h *UnansweredHandler) GMList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	total, rows, err := h.service.List(c.Request.Context(), listUnansweredQuery(c, page, size))
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, gin.H{"list": rows, "total": total})
}

func (h *UnansweredHandler) GMStats(c *gin.Context) {
	stats, err := h.service.Stats(c.Request.Context())
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, stats)
}

func (h *UnansweredHandler) GMEvents(c *gin.Context) {
	rows, err := h.service.ListEvents(c.Request.Context(), c.Param("id"), 100)
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, rows)
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
	h.convertMu.Lock()
	defer h.convertMu.Unlock()
	row, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		gmError(c, http.StatusNotFound, err.Error())
		return
	}
	if row.Status == unanswered.StatusConverted {
		item, getErr := h.knowledge.Get(c.Request.Context(), row.ConvertedQAID)
		if getErr != nil {
			gmError(c, http.StatusConflict, "question is converted but linked QA is missing")
			return
		}
		gmOK(c, gin.H{"question": row, "qa": item, "idempotent": true})
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
		_ = h.knowledge.Delete(c.Request.Context(), item.ID)
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
	total, rows, err := h.service.List(c.Request.Context(), listUnansweredQuery(c, page, size))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows, "total": total})
}
func (h *UnansweredHandler) Stats(c *gin.Context) {
	stats, err := h.service.Stats(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, stats)
}
func (h *UnansweredHandler) Events(c *gin.Context) {
	rows, err := h.service.ListEvents(c.Request.Context(), c.Param("id"), 100)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, rows)
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
	h.convertMu.Lock()
	defer h.convertMu.Unlock()
	row, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	if row.Status == unanswered.StatusConverted {
		item, getErr := h.knowledge.Get(c.Request.Context(), row.ConvertedQAID)
		if getErr != nil {
			response.Error(c, http.StatusConflict, "question is converted but linked QA is missing")
			return
		}
		response.OK(c, gin.H{"question": row, "qa": item, "idempotent": true})
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
		_ = h.knowledge.Delete(c.Request.Context(), item.ID)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"question": row, "qa": item})
}

func listUnansweredQuery(c *gin.Context, page, size int) unanswered.ListQuery {
	return unanswered.ListQuery{Q: c.Query("q"), Status: c.Query("status"), Reason: c.Query("reason"), TenantID: c.Query("tenant_id"), ChannelID: c.Query("channel_id"), Sort: c.Query("sort"), Page: page, Size: size}
}
func (h *UnansweredHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
