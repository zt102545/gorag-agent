package handler

import (
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"strings"

	"github.com/yourorg/agent_customer_service/internal/server/middleware"
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

	var assistantContent strings.Builder
	var assistantToolCalls []schema.ToolCall
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Output != nil && event.Output.MessageOutput != nil && event.Output.MessageOutput.Message != nil {
			msg := event.Output.MessageOutput.Message
			if msg.Role == schema.Assistant {
				assistantContent.WriteString(msg.Content)
				if len(msg.ToolCalls) > 0 {
					assistantToolCalls = append(assistantToolCalls, msg.ToolCalls...)
				}
			}
		}
		if err := h.sseWriter.Write(c.Writer, event); err != nil {
			break
		}
		c.Writer.Flush()
	}

	content := strings.TrimSpace(assistantContent.String())
	if content != "" || len(assistantToolCalls) > 0 {
		persistCtx, cancel := persistenceContext(c.Request.Context())
		defer cancel()
		_ = h.sessionStore.AppendMessages(persistCtx, sessionID, session.AppendParams{
			RequestID: c.GetString(middleware.RequestIDKey),
		}, schema.AssistantMessage(content, assistantToolCalls))
	}
}
