package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/graph/rag"
	"github.com/yourorg/agent_customer_service/internal/server/middleware"
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
	Message         string `json:"message" binding:"required"`
	ClientMessageID string `json:"client_message_id"`
}

type AskRequest struct {
	Question        string         `json:"question" binding:"required"`
	CallerUserID    string         `json:"caller_user_id"`
	ChannelID       string         `json:"channel_id"`
	TenantID        string         `json:"tenant_id"`
	History         []HistoryInput `json:"history"`
	Meta            map[string]any `json:"meta"`
	ClientMessageID string         `json:"client_message_id"`
}

type HistoryInput struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (h *ChatHandler) Chat(c *gin.Context) {
	start := time.Now()
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

	bizCtx := middleware.GetBizContext(c)
	requestID := c.GetString(middleware.RequestIDKey)
	userMsg, messages := h.buildMessages(c.Request.Context(), history, req.Message)
	if err := h.sessionStore.AppendMessages(c.Request.Context(), sessionID, session.AppendParams{
		RequestID:       requestID,
		ClientMessageID: req.ClientMessageID,
	}, userMsg); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("X-Request-ID", requestID)

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
		_ = h.sessionStore.AppendMessages(c.Request.Context(), sessionID, session.AppendParams{
			RequestID: requestID,
		}, schema.AssistantMessage(content, assistantToolCalls))
	}
	logChat(c, "session_chat_completed", start, bizCtx, sessionID, req.ClientMessageID)
}

func (h *ChatHandler) Ask(c *gin.Context) {
	start := time.Now()
	var req AskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	bizCtx := middleware.GetBizContext(c)
	requestID := c.GetString(middleware.RequestIDKey)
	if bizCtx != nil {
		if req.CallerUserID == "" {
			req.CallerUserID = bizCtx.CallerUserID
		}
		if req.ChannelID == "" {
			req.ChannelID = bizCtx.ChannelID
		}
		if req.TenantID == "" {
			req.TenantID = bizCtx.TenantID
		}
	}

	history := convertHistory(req.History)
	_, messages := h.buildMessages(c.Request.Context(), history, req.Question)
	iter := h.runner.Run(c.Request.Context(), messages)
	var assistantContent strings.Builder
	var toolCalls []schema.ToolCall
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
					toolCalls = append(toolCalls, msg.ToolCalls...)
				}
			}
		}
	}

	answer := strings.TrimSpace(assistantContent.String())
	logChat(c, "stateless_ask_completed", start, bizCtx, "", req.ClientMessageID)
	response.OK(c, gin.H{
		"answer":           answer,
		"sources":          []any{},
		"matched_qa_ids":   []string{},
		"need_human":       containsTransferTool(toolCalls),
		"session_id":       "",
		"request_id":       requestID,
		"trace_id":         requestID,
		"latency_ms":       time.Since(start).Milliseconds(),
		"usage":            gin.H{},
		"caller_user_id":   req.CallerUserID,
		"channel_id":       req.ChannelID,
		"tenant_id":        req.TenantID,
		"tool_calls":       toolCalls,
		"knowledge_shared": true,
	})
}

func (h *ChatHandler) buildMessages(ctx context.Context, history []*schema.Message, content string) (*schema.Message, []*schema.Message) {
	userMsg := schema.UserMessage(content)
	messages := history
	if h.intent != nil {
		if res, err := h.intent.Analyze(ctx, content); err == nil && res != nil {
			hint := fmt.Sprintf("意图分析结果：intent=%s need_search=%t confidence=%.2f reason=%s。请根据 need_search 决定是否调用 knowledge_search；need_search=false 时不要调用。",
				res.Intent, res.NeedSearch, res.Confidence, res.Reason)
			messages = append(messages, schema.SystemMessage(hint))
		}
	}
	messages = append(messages, userMsg)
	return userMsg, messages
}

func convertHistory(history []HistoryInput) []*schema.Message {
	out := make([]*schema.Message, 0, len(history))
	for _, item := range history {
		switch strings.ToLower(strings.TrimSpace(item.Role)) {
		case "assistant":
			out = append(out, schema.AssistantMessage(item.Content, nil))
		case "system":
			out = append(out, schema.SystemMessage(item.Content))
		default:
			out = append(out, schema.UserMessage(item.Content))
		}
	}
	return out
}

func containsTransferTool(toolCalls []schema.ToolCall) bool {
	for _, call := range toolCalls {
		if strings.Contains(strings.ToLower(call.Function.Name), "transfer") {
			return true
		}
	}
	return false
}

func logChat(c *gin.Context, event string, start time.Time, bizCtx *middleware.BizContext, sessionID, clientMessageID string) {
	logger := fmt.Sprintf("[chat] event=%s request_id=%s session_id=%s latency_ms=%d client_message_id=%s", event, c.GetString(middleware.RequestIDKey), sessionID, time.Since(start).Milliseconds(), clientMessageID)
	if bizCtx != nil {
		logger += fmt.Sprintf(" auth_type=%s caller_token_id=%s caller_user_id=%s channel_id=%s tenant_id=%s",
			bizCtx.AuthType, bizCtx.CallerTokenID, bizCtx.CallerUserID, bizCtx.ChannelID, bizCtx.TenantID)
	}
	fmt.Println(logger)
}
