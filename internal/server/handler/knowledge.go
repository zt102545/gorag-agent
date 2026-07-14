package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	knowledgeapp "github.com/yourorg/agent_customer_service/internal/application/knowledge"
	sessionapp "github.com/yourorg/agent_customer_service/internal/application/session"
	settingsapp "github.com/yourorg/agent_customer_service/internal/application/settings"
	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
	"github.com/yourorg/agent_customer_service/internal/session"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

type KnowledgeHandler struct {
	service  *knowledgeapp.Service
	jobs     *knowledgeapp.ClassificationJobService
	settings *settingsapp.Service
}

// GMList returns the QA list in the response envelope expected by Web-GM-Client.
// The native admin endpoint remains unchanged.
func (h *KnowledgeHandler) GMList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("pageNum", c.DefaultQuery("page", "1")))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", c.DefaultQuery("size", "20")))
	result, err := h.service.List(c.Request.Context(), qa.ListQuery{
		KnowledgeBaseID: c.Query("knowledge_base_id"),
		Category:        c.Query("category"),
		ClassifyStatus:  c.Query("classify_status"),
		Tag:             c.Query("tag"),
		Q:               c.Query("q"),
		Page:            page,
		Size:            size,
	})
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, gin.H{"list": result.Items, "total": result.Total})
}

// GMCategories returns all non-empty QA categories for the GM filter.
func (h *KnowledgeHandler) GMCategories(c *gin.Context) {
	categories, err := h.service.ListCategories(c.Request.Context(), c.Query("knowledge_base_id"))
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, categories)
}

// GMTags returns all non-empty QA tags for the GM filter.
func (h *KnowledgeHandler) GMTags(c *gin.Context) {
	tags, err := h.service.ListTags(c.Request.Context(), c.Query("knowledge_base_id"))
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, tags)
}

// GMImport imports a CSV QA file using the GM response envelope.
func (h *KnowledgeHandler) GMImport(c *gin.Context) {
	if !h.canUseEmbedding(c) {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		gmError(c, http.StatusBadRequest, "file is required")
		return
	}
	f, err := file.Open()
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	result, err := h.service.ImportCSVTo(c.Request.Context(), f, c.PostForm("knowledge_base_id"))
	if err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	gmOK(c, result)
}

// GMCreate adds a single QA item using the response envelope expected by
// Web-GM-Client.
func (h *KnowledgeHandler) GMCreate(c *gin.Context) {
	if !h.canUseEmbedding(c) {
		return
	}
	var req qa.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, item)
}

// GMUpdate updates a single QA item and rebuilds its vector index entry.
func (h *KnowledgeHandler) GMUpdate(c *gin.Context) {
	if !h.canUseEmbedding(c) {
		return
	}
	var req qa.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		gmError(c, http.StatusNotFound, err.Error())
		return
	}
	gmOK(c, item)
}

// GMDelete removes both the QA metadata and its vector index entry.
func (h *KnowledgeHandler) GMDelete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		gmError(c, http.StatusNotFound, err.Error())
		return
	}
	gmOK(c, gin.H{"deleted": true})
}

// GMBatchDelete deletes up to 200 QA items in one request. Individual failures
// are returned so the GM can refresh the list and report a partial result.
func (h *KnowledgeHandler) GMBatchDelete(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required,min=1,max=200,dive,required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}

	deleted := 0
	failed := make([]gin.H, 0)
	for _, id := range req.IDs {
		if err := h.service.Delete(c.Request.Context(), id); err != nil {
			failed = append(failed, gin.H{"id": id, "error": err.Error()})
			continue
		}
		deleted++
	}
	gmOK(c, gin.H{"deleted": deleted, "failed": failed})
}

func NewKnowledgeHandler(service *knowledgeapp.Service, jobs *knowledgeapp.ClassificationJobService, settings *settingsapp.Service) *KnowledgeHandler {
	return &KnowledgeHandler{service: service, jobs: jobs, settings: settings}
}

func (h *KnowledgeHandler) Create(c *gin.Context) {
	if !h.canUseEmbedding(c) {
		return
	}
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
	if !h.canUseEmbedding(c) {
		return
	}
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
		KnowledgeBaseID: c.Query("knowledge_base_id"),
		Category:        c.Query("category"),
		ClassifyStatus:  c.Query("classify_status"),
		Tag:             c.Query("tag"),
		Q:               c.Query("q"),
		Page:            page,
		Size:            size,
	})
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *KnowledgeHandler) Import(c *gin.Context) {
	if !h.canUseEmbedding(c) {
		return
	}
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
	if !h.canUseEmbedding(c) {
		return
	}
	item, err := h.jobs.Reclassify(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, item)
}

func (h *KnowledgeHandler) Categories(c *gin.Context) {
	cats, err := h.service.ListCategories(c.Request.Context(), c.Query("knowledge_base_id"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, cats)
}

func (h *KnowledgeHandler) Tags(c *gin.Context) {
	tags, err := h.service.ListTags(c.Request.Context(), c.Query("knowledge_base_id"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, tags)
}

func (h *KnowledgeHandler) canUseEmbedding(c *gin.Context) bool {
	if h.settings == nil {
		return true
	}
	if !h.settings.EmbeddingConfigured(c.Request.Context()) {
		response.Error(c, http.StatusServiceUnavailable, "embedding model is not configured in GM")
		return false
	}
	return true
}

type AdminSessionHandler struct {
	service *sessionapp.Service
}

// GMList returns retained conversations in Web-GM-Client's table format.
func (h *AdminSessionHandler) GMList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("pageNum", c.DefaultQuery("page", "1")))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", c.DefaultQuery("size", "20")))
	total, items, err := h.service.AdminList(c.Request.Context(), session.ListQuery{
		SessionID:      c.Query("session_id"),
		Q:              c.Query("q"),
		Page:           page,
		Size:           size,
		Status:         c.Query("status"),
		ExternalUserID: c.Query("caller_user_id"),
		ChannelID:      c.Query("channel_id"),
		TenantID:       c.Query("tenant_id"),
		CreatedFrom:    parseRFC3339Query(c.Query("created_from")),
		CreatedTo:      parseRFC3339Query(c.Query("created_to")),
	})
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, gin.H{"list": items, "total": total})
}

func (h *AdminSessionHandler) GMGet(c *gin.Context) {
	meta, messages, err := h.service.AdminGet(c.Request.Context(), c.Param("id"))
	if err != nil {
		gmError(c, http.StatusNotFound, err.Error())
		return
	}
	gmOK(c, gin.H{"session": meta, "messages": messages, "message_count": len(messages)})
}

func gmOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": data})
}

func gmError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": status, "err": message})
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
