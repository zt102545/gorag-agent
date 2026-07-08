package handler

import (
	"github.com/cloudwego/eino/adk"
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/session"
	"github.com/yourorg/agent_customer_service/pkg/response"
	"github.com/yourorg/agent_customer_service/pkg/sse"
)

type ApprovalHandler struct {
	runner       *adk.Runner
	sessionStore session.Store
	sseWriter    *sse.Writer
}

func NewApprovalHandler(runner *adk.Runner, sessionStore session.Store, sseWriter *sse.Writer) *ApprovalHandler {
	return &ApprovalHandler{
		runner:       runner,
		sessionStore: sessionStore,
		sseWriter:    sseWriter,
	}
}

type ApprovalRequest struct {
	InterruptID string `json:"interrupt_id" binding:"required"`
	Approved    bool   `json:"approved"`
}

func (h *ApprovalHandler) Approve(c *gin.Context) {
	sessionID := c.Param("id")
	var req ApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if _, err := h.sessionStore.Get(c.Request.Context(), sessionID); err != nil {
		response.NotFound(c, err.Error())
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	iter, err := h.runner.ResumeWithParams(c.Request.Context(), sessionID, &adk.ResumeParams{
		Targets: map[string]any{
			req.InterruptID: req.Approved,
		},
	})
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if err := h.sseWriter.Write(c.Writer, event); err != nil {
			break
		}
		c.Writer.Flush()
	}
}
