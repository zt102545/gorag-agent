package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
	"github.com/yourorg/agent_customer_service/internal/session"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

type KnowledgeHandler struct {
	qaService *qa.Service
}

func NewKnowledgeHandler(qaService *qa.Service) *KnowledgeHandler {
	return &KnowledgeHandler{qaService: qaService}
}

func (h *KnowledgeHandler) Create(c *gin.Context) {
	var req qa.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.qaService.Create(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, item)
}

func (h *KnowledgeHandler) Get(c *gin.Context) {
	item, err := h.qaService.Get(c.Request.Context(), c.Param("id"))
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
	item, err := h.qaService.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, item)
}

func (h *KnowledgeHandler) Delete(c *gin.Context) {
	if err := h.qaService.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func (h *KnowledgeHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	result, err := h.qaService.List(c.Request.Context(), qa.ListQuery{
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

	result, err := h.qaService.ImportCSV(c.Request.Context(), f)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *KnowledgeHandler) Reclassify(c *gin.Context) {
	item, err := h.qaService.Reclassify(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, item)
}

func (h *KnowledgeHandler) Categories(c *gin.Context) {
	cats, err := h.qaService.ListCategories(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, cats)
}

func (h *KnowledgeHandler) Tags(c *gin.Context) {
	tags, err := h.qaService.ListTags(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, tags)
}

type AdminSessionHandler struct {
	store *session.SQLStore
}

func NewAdminSessionHandler(store *session.SQLStore) *AdminSessionHandler {
	return &AdminSessionHandler{store: store}
}

func (h *AdminSessionHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	total, items, err := h.store.List(c.Request.Context(), page, size, c.Query("status"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"total": total, "items": items})
}

func (h *AdminSessionHandler) Get(c *gin.Context) {
	meta, err := h.store.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	messages, err := h.store.GetMessages(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"session": meta, "messages": messages})
}
