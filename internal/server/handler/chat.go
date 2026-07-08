package handler

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/graph/rag"
	"github.com/yourorg/agent_customer_service/internal/session"
	"github.com/yourorg/agent_customer_service/pkg/response"
	"github.com/yourorg/agent_customer_service/pkg/sse"
)

type ChatHandler struct {
	runner       *adk.Runner
	sessionStore session.Store
	sseWriter    *sse.Writer
	intent       *rag.IntentAnalyzer
}

func NewChatHandler(runner *adk.Runner, sessionStore session.Store, sseWriter *sse.Writer, intent *rag.IntentAnalyzer) *ChatHandler {
	return &ChatHandler{
		runner:       runner,
		sessionStore: sessionStore,
		sseWriter:    sseWriter,
		intent:       intent,
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
	messages := history
	if h.intent != nil {
		if res, err := h.intent.Analyze(c.Request.Context(), req.Message); err == nil && res != nil {
			hint := fmt.Sprintf("意图分析结果：intent=%s need_search=%t confidence=%.2f reason=%s。请根据 need_search 决定是否调用 knowledge_search；need_search=false 时不要调用。",
				res.Intent, res.NeedSearch, res.Confidence, res.Reason)
			messages = append(messages, schema.SystemMessage(hint))
		}
	}
	messages = append(messages, userMsg)
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
		_ = h.sessionStore.AppendMessages(c.Request.Context(), sessionID, schema.AssistantMessage(content, assistantToolCalls))
	}
}
