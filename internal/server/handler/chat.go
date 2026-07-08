package handler

import (
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/session"
	"github.com/yourorg/agent_customer_service/pkg/response"
	"github.com/yourorg/agent_customer_service/pkg/sse"
)

type ChatHandler struct {
	runner       *adk.Runner
	sessionStore session.Store
	sseWriter    *sse.Writer
}

func NewChatHandler(runner *adk.Runner, sessionStore session.Store, sseWriter *sse.Writer) *ChatHandler {
	return &ChatHandler{
		runner:       runner,
		sessionStore: sessionStore,
		sseWriter:    sseWriter,
	}
}

type ChatRequest struct {
	Message string `json:"message" binding:"required"`
}

func (h *ChatHandler) Chat(c *gin.Context) {
	sessionID := c.Param("id")
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if _, err := h.sessionStore.Get(c.Request.Context(), sessionID); err != nil {
		response.NotFound(c, err.Error())
		return
	}

	history, err := h.sessionStore.GetMessages(c.Request.Context(), sessionID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	userMsg := schema.UserMessage(req.Message)
	messages := append(history, userMsg)
	if err := h.sessionStore.AppendMessages(c.Request.Context(), sessionID, userMsg); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	iter := h.runner.Run(c.Request.Context(), messages, adk.WithCheckPointID(sessionID))

	var assistantContent strings.Builder
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Output != nil && event.Output.MessageOutput != nil && event.Output.MessageOutput.Message != nil {
			if event.Output.MessageOutput.Message.Role == schema.Assistant {
				assistantContent.WriteString(event.Output.MessageOutput.Message.Content)
			}
		}
		if err := h.sseWriter.Write(c.Writer, event); err != nil {
			break
		}
		c.Writer.Flush()
	}

	if content := strings.TrimSpace(assistantContent.String()); content != "" {
		_ = h.sessionStore.AppendMessages(c.Request.Context(), sessionID, schema.AssistantMessage(content, nil))
	}
}
