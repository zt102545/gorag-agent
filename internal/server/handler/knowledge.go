package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	knowledgeapp "github.com/yourorg/agent_customer_service/internal/application/knowledge"
	sessionapp "github.com/yourorg/agent_customer_service/internal/application/session"
	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
	"github.com/yourorg/agent_customer_service/internal/session"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

type KnowledgeHandler struct {
	service *knowledgeapp.Service
	jobs    *knowledgeapp.ClassificationJobService
}

func NewKnowledgeHandler(service *knowledgeapp.Service, jobs *knowledgeapp.ClassificationJobService) *KnowledgeHandler {
	return &KnowledgeHandler{service: service, jobs: jobs}
}

func (h *KnowledgeHandler) Create(c *gin.Context) {
	var req qa.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, item)
}

func (h *KnowledgeHandler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, item)
}

func (h *KnowledgeHandler) Update(c *gin.Context) {
	var req qa.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, item)
}

func (h *KnowledgeHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *KnowledgeHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	result, err := h.service.List(c.Request.Context(), qa.ListQuery{
		Category:       c.Query("category"),
		ClassifyStatus: c.Query("classify_status"),
		Tag:            c.Query("tag"),
		Q:              c.Query("q"),
		Page:           page,
		Size:           size,
	})
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *KnowledgeHandler) Import(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	f, err := file.Open()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	defer f.Close()

	result, err := h.service.ImportCSV(c.Request.Context(), f)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *KnowledgeHandler) Reclassify(c *gin.Context) {
	item, err := h.jobs.Reclassify(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, item)
}

func (h *KnowledgeHandler) Categories(c *gin.Context) {
	cats, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, cats)
}

func (h *KnowledgeHandler) Tags(c *gin.Context) {
	tags, err := h.service.ListTags(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, tags)
}

type AdminSessionHandler struct {
	service *sessionapp.Service
}

func NewAdminSessionHandler(service *sessionapp.Service) *AdminSessionHandler {
	return &AdminSessionHandler{service: service}
}

func (h *AdminSessionHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	createdFrom := parseRFC3339Query(c.Query("created_from"))
	createdTo := parseRFC3339Query(c.Query("created_to"))
	total, items, err := h.service.AdminList(c.Request.Context(), session.ListQuery{
		SessionID:      c.Query("session_id"),
		Q:              c.Query("q"),
		Page:           page,
		Size:           size,
		Status:         c.Query("status"),
		ExternalUserID: c.Query("caller_user_id"),
		ChannelID:      c.Query("channel_id"),
		TenantID:       c.Query("tenant_id"),
		CallerTokenID:  c.Query("caller_token_id"),
		CreatedFrom:    createdFrom,
		CreatedTo:      createdTo,
	})
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"total": total, "items": items})
}

func (h *AdminSessionHandler) Get(c *gin.Context) {
	meta, messages, err := h.service.AdminGet(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, gin.H{
		"session":       meta,
		"messages":      messages,
		"message_count": len(messages),
	})
}

func parseRFC3339Query(value string) *time.Time {
	if value == "" {
		return nil
	}
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &ts
}
