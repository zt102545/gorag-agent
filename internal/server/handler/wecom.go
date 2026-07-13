package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/wecom"
	"github.com/yourorg/agent_customer_service/pkg/response"
)

type WeComHandler struct{ service *wecom.Service }

func NewWeComHandler(service *wecom.Service) *WeComHandler { return &WeComHandler{service: service} }

func (h *WeComHandler) GMGetConfig(c *gin.Context) {
	view, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, view)
}

func (h *WeComHandler) GMSaveConfig(c *gin.Context) {
	var req wecom.ConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	view, err := h.service.SaveConfig(c.Request.Context(), req)
	if err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	gmOK(c, view)
}

func (h *WeComHandler) GMTest(c *gin.Context) {
	if err := h.service.TestConnection(c.Request.Context()); err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	gmOK(c, gin.H{"connected": true})
}

func (h *WeComHandler) GMSync(c *gin.Context) {
	result, err := h.service.SyncNow(c.Request.Context())
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, result)
}

func (h *WeComHandler) GMAnalyze(c *gin.Context) {
	var req struct {
		Date string `json:"date"`
	}
	_ = c.ShouldBindJSON(&req)
	result, err := h.service.AnalyzeDate(c.Request.Context(), req.Date)
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, result)
}

func (h *WeComHandler) GMMessages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	total, rows, err := h.service.ListMessages(c.Request.Context(), c.Query("room_id"), c.Query("date"), page, size)
	if err != nil {
		gmError(c, http.StatusBadRequest, err.Error())
		return
	}
	gmOK(c, gin.H{"list": rows, "total": total})
}

func (h *WeComHandler) GMRuns(c *gin.Context) {
	rows, err := h.service.ListRuns(c.Request.Context(), 50)
	if err != nil {
		gmError(c, http.StatusInternalServerError, err.Error())
		return
	}
	gmOK(c, rows)
}

func (h *WeComHandler) GetConfig(c *gin.Context) {
	view, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, view)
}

func (h *WeComHandler) SaveConfig(c *gin.Context) {
	var req wecom.ConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	view, err := h.service.SaveConfig(c.Request.Context(), req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, view)
}

func (h *WeComHandler) Test(c *gin.Context) {
	if err := h.service.TestConnection(c.Request.Context()); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"connected": true})
}

func (h *WeComHandler) Sync(c *gin.Context) {
	result, err := h.service.SyncNow(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *WeComHandler) Analyze(c *gin.Context) {
	var req struct {
		Date string `json:"date"`
	}
	_ = c.ShouldBindJSON(&req)
	result, err := h.service.AnalyzeDate(c.Request.Context(), req.Date)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *WeComHandler) Messages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	total, rows, err := h.service.ListMessages(c.Request.Context(), c.Query("room_id"), c.Query("date"), page, size)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"items": rows, "total": total})
}

func (h *WeComHandler) Runs(c *gin.Context) {
	rows, err := h.service.ListRuns(c.Request.Context(), 50)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, rows)
}
