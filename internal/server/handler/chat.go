package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"

	"github.com/gorag-ai/gorag/internal/admin"
	agenttools "github.com/gorag-ai/gorag/internal/agent/tools"
	settingsapp "github.com/gorag-ai/gorag/internal/application/settings"
	"github.com/gorag-ai/gorag/internal/graph/rag"
	"github.com/gorag-ai/gorag/internal/knowledge/unanswered"
	"github.com/gorag-ai/gorag/internal/server/middleware"
	"github.com/gorag-ai/gorag/internal/session"
	"github.com/gorag-ai/gorag/pkg/response"
	"github.com/gorag-ai/gorag/pkg/sse"
)

type ChatHandler struct {
	runner       *adk.Runner
	sessionStore session.Store
	sseWriter    *sse.Writer
	settings     *settingsapp.Service
	aiErrors     *admin.AIErrorService
	unanswered   *unanswered.Service
}

func NewChatHandler(runner *adk.Runner, sessionStore session.Store, sseWriter *sse.Writer, settings *settingsapp.Service, aiErrors *admin.AIErrorService, unansweredService *unanswered.Service) *ChatHandler {
	return &ChatHandler{
		runner:       runner,
		sessionStore: sessionStore,
		sseWriter:    sseWriter,
		settings:     settings,
		aiErrors:     aiErrors,
		unanswered:   unansweredService,
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

type chatDecision struct {
	Reply            string
	NeedHuman        bool
	UsedDefaultReply bool
	KnowledgeShared  bool
	SearchQuery      string
	SearchHits       []rag.SearchHit
	Trace            []chatTraceStep
	Unanswered       bool
	UnansweredReason string
}

type chatTraceStep struct {
	Name       string         `json:"name"`
	Label      string         `json:"label"`
	Status     string         `json:"status"`
	Detail     string         `json:"detail,omitempty"`
	Input      map[string]any `json:"input,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

func (d *chatDecision) addTrace(name, label, status, detail string, input, output map[string]any, err error, started time.Time) {
	step := chatTraceStep{
		Name:      name,
		Label:     label,
		Status:    status,
		Detail:    detail,
		Input:     input,
		Output:    output,
		CreatedAt: time.Now(),
	}
	if err != nil {
		step.Error = err.Error()
	}
	if !started.IsZero() {
		step.DurationMS = time.Since(started).Milliseconds()
	}
	d.Trace = append(d.Trace, step)
}

type sourceView struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Score    float64  `json:"score,omitempty"`
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
	history = h.limitAgentHistory(c.Request.Context(), history)

	bizCtx := middleware.GetBizContext(c)
	requestID := c.GetString(middleware.RequestIDKey)
	if !h.canHandleChat(c) {
		return
	}
	decision, err := h.decideChat(c.Request.Context(), history, req.Message)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if decision.Reply != "" {
		h.captureUnanswered(c.Request.Context(), decision, req.Message, decision.Reply, sessionID, requestID, bizCtx)
		persistCtx, cancel := persistenceContext(c.Request.Context())
		defer cancel()
		if err := h.sessionStore.AppendMessages(persistCtx, sessionID, session.AppendParams{
			RequestID:       requestID,
			ClientMessageID: req.ClientMessageID,
			Extra:           messageExtra(decision.Trace),
		}, schema.UserMessage(req.Message), schema.AssistantMessage(decision.Reply, nil)); err != nil {
			h.reportAIError(persistCtx, "session_persist", err.Error(), requestID, sessionID, "persist direct reply")
			response.InternalError(c, err.Error())
			return
		}
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
		c.Header("X-Request-ID", requestID)
		if err := h.sseWriter.WriteEvent(c.Writer, sse.Event{
			Type:    "message",
			Content: decision.Reply,
		}); err == nil {
			c.Writer.Flush()
		}
		logChat(c, "session_chat_fallback", start, bizCtx, sessionID, req.ClientMessageID)
		return
	}
	userMsg, messages := h.buildMessages(c.Request.Context(), history, req.Message, decision)
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

	searches := &toolSearchCollector{}
	runCtx := agenttools.WithSearchObserver(c.Request.Context(), searches.Observe)
	iter := h.runner.Run(runCtx, messages, adk.WithCheckPointID(sessionID))

	var assistantContent strings.Builder
	var assistantToolCalls []schema.ToolCall
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			h.reportAIError(c.Request.Context(), "chat_runner", event.Err.Error(), requestID, sessionID, "session streaming runner event")
		}
		var eventAssistantChunks []*schema.Message
		if err := h.sseWriter.WriteWithObserver(c.Writer, event, func(msg *schema.Message) {
			if msg != nil && msg.Role == schema.Assistant {
				eventAssistantChunks = append(eventAssistantChunks, msg)
			}
		}); err != nil {
			break
		}
		if err := collectAssistantChunks(eventAssistantChunks, &assistantContent, &assistantToolCalls); err != nil {
			h.reportAIError(c.Request.Context(), "chat_runner_stream", err.Error(), requestID, sessionID, "merge session streaming output")
		}
		c.Writer.Flush()
	}

	content := strings.TrimSpace(assistantContent.String())
	decision.SearchQuery, decision.SearchHits = searches.Snapshot()
	decision.KnowledgeShared = containsToolCall(assistantToolCalls, "knowledge_search")
	decision.NeedHuman = containsToolCall(assistantToolCalls, "transfer_human")
	h.captureUnanswered(c.Request.Context(), decision, req.Message, content, sessionID, requestID, bizCtx)
	if content != "" || len(assistantToolCalls) > 0 {
		decision.addTrace("agent_runner", "Agent 执行", "finished", "模型生成回复并可能发起工具调用", nil, map[string]any{
			"content_chars": len([]rune(content)),
			"tool_calls":    len(assistantToolCalls),
		}, nil, start)
		for _, call := range assistantToolCalls {
			decision.addTrace("tool_call", "工具调用", "finished", call.Function.Name, map[string]any{
				"name":      call.Function.Name,
				"arguments": call.Function.Arguments,
			}, nil, nil, time.Time{})
		}
		persistCtx, cancel := persistenceContext(c.Request.Context())
		defer cancel()
		if err := h.sessionStore.AppendMessages(persistCtx, sessionID, session.AppendParams{
			RequestID: requestID,
			Extra:     messageExtra(decision.Trace),
		}, schema.AssistantMessage(content, assistantToolCalls)); err != nil {
			h.reportAIError(persistCtx, "session_persist", err.Error(), requestID, sessionID, "persist streaming reply")
		}
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
	if !h.canHandleChat(c) {
		return
	}
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

	history := h.limitAgentHistory(c.Request.Context(), convertHistory(req.History))
	decision, err := h.decideChat(c.Request.Context(), history, req.Question)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if decision.Reply != "" {
		h.captureUnanswered(c.Request.Context(), decision, req.Question, decision.Reply, "", requestID, bizCtx)
		logChat(c, "stateless_ask_fallback", start, bizCtx, "", req.ClientMessageID)
		response.OK(c, gin.H{
			"answer":             decision.Reply,
			"sources":            buildSources(decision.SearchHits),
			"matched_qa_ids":     buildMatchedQAIDs(decision.SearchHits),
			"need_human":         decision.NeedHuman,
			"session_id":         "",
			"request_id":         requestID,
			"trace_id":           requestID,
			"latency_ms":         time.Since(start).Milliseconds(),
			"usage":              gin.H{},
			"caller_user_id":     req.CallerUserID,
			"channel_id":         req.ChannelID,
			"tenant_id":          req.TenantID,
			"tool_calls":         []schema.ToolCall{},
			"knowledge_shared":   decision.KnowledgeShared,
			"used_default_reply": decision.UsedDefaultReply,
		})
		return
	}
	_, messages := h.buildMessages(c.Request.Context(), history, req.Question, decision)
	searches := &toolSearchCollector{}
	runCtx := agenttools.WithSearchObserver(c.Request.Context(), searches.Observe)
	iter := h.runner.Run(runCtx, messages)
	var assistantContent strings.Builder
	var toolCalls []schema.ToolCall
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			h.reportAIError(c.Request.Context(), "chat_runner", event.Err.Error(), requestID, "", "stateless ask runner event")
		}
		if err := collectAgentOutput(event, &assistantContent, &toolCalls); err != nil {
			h.reportAIError(c.Request.Context(), "chat_runner_stream", err.Error(), requestID, "", "stateless ask consume runner stream")
		}
	}

	answer := strings.TrimSpace(assistantContent.String())
	decision.SearchQuery, decision.SearchHits = searches.Snapshot()
	decision.KnowledgeShared = containsToolCall(toolCalls, "knowledge_search")
	decision.NeedHuman = containsToolCall(toolCalls, "transfer_human")
	h.captureUnanswered(c.Request.Context(), decision, req.Question, answer, "", requestID, bizCtx)
	decision.addTrace("agent_runner", "Agent 执行", "finished", "模型生成回复并可能发起工具调用", nil, map[string]any{
		"content_chars": len([]rune(answer)),
		"tool_calls":    len(toolCalls),
	}, nil, start)
	for _, call := range toolCalls {
		decision.addTrace("tool_call", "工具调用", "finished", call.Function.Name, map[string]any{
			"name":      call.Function.Name,
			"arguments": call.Function.Arguments,
		}, nil, nil, time.Time{})
	}
	logChat(c, "stateless_ask_completed", start, bizCtx, "", req.ClientMessageID)
	response.OK(c, gin.H{
		"answer":           answer,
		"sources":          buildSources(decision.SearchHits),
		"matched_qa_ids":   buildMatchedQAIDs(decision.SearchHits),
		"need_human":       decision.NeedHuman,
		"session_id":       "",
		"request_id":       requestID,
		"trace_id":         requestID,
		"latency_ms":       time.Since(start).Milliseconds(),
		"usage":            gin.H{},
		"caller_user_id":   req.CallerUserID,
		"channel_id":       req.ChannelID,
		"tenant_id":        req.TenantID,
		"tool_calls":       toolCalls,
		"knowledge_shared": decision.KnowledgeShared,
	})
}

func collectAgentOutput(event *adk.AgentEvent, content *strings.Builder, toolCalls *[]schema.ToolCall) error {
	if event == nil || event.Output == nil || event.Output.MessageOutput == nil {
		return nil
	}
	output := event.Output.MessageOutput
	if output.Message != nil {
		return collectAssistantChunks([]*schema.Message{output.Message}, content, toolCalls)
	}
	if output.MessageStream == nil {
		return nil
	}
	defer output.MessageStream.Close()
	var chunks []*schema.Message
	for {
		chunk, err := output.MessageStream.Recv()
		if errors.Is(err, io.EOF) {
			return collectAssistantChunks(chunks, content, toolCalls)
		}
		if err != nil {
			return err
		}
		if chunk != nil && chunk.Role == schema.Assistant {
			chunks = append(chunks, chunk)
		}
	}
}

func collectAssistantChunks(chunks []*schema.Message, content *strings.Builder, toolCalls *[]schema.ToolCall) error {
	if len(chunks) == 0 {
		return nil
	}
	message, err := schema.ConcatMessages(chunks)
	if err != nil {
		return err
	}
	if message == nil || message.Role != schema.Assistant {
		return nil
	}
	content.WriteString(message.Content)
	if len(message.ToolCalls) > 0 {
		*toolCalls = append(*toolCalls, message.ToolCalls...)
	}
	return nil
}

type toolSearchCollector struct {
	mu      sync.Mutex
	query   string
	hits    []rag.SearchHit
	seenIDs map[string]struct{}
}

func (c *toolSearchCollector) Observe(query string, hits []rag.SearchHit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.query = strings.TrimSpace(query)
	if c.seenIDs == nil {
		c.seenIDs = make(map[string]struct{}, len(hits))
	}
	for _, hit := range hits {
		if hit.ID != "" {
			if _, exists := c.seenIDs[hit.ID]; exists {
				continue
			}
			c.seenIDs[hit.ID] = struct{}{}
		}
		c.hits = append(c.hits, hit)
	}
}

func (c *toolSearchCollector) Snapshot() (string, []rag.SearchHit) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.query, append([]rag.SearchHit(nil), c.hits...)
}

func (h *ChatHandler) canHandleChat(c *gin.Context) bool {
	if h.settings == nil {
		return true
	}
	if !h.settings.ChatConfigured(c.Request.Context()) {
		response.Error(c, http.StatusServiceUnavailable, "chat model is not configured in GM")
		return false
	}
	// 未配置 embedding 时不再拦截：RAG 检索会退化为纯关键词（BM25）。
	return true
}

func exceedsContextLimit(maxMessages, maxChars int, history []*schema.Message, currentInput string) bool {
	if maxMessages > 0 && len(history) >= maxMessages {
		return true
	}
	if maxChars <= 0 {
		return false
	}
	total := len(currentInput)
	for _, message := range history {
		if message == nil {
			continue
		}
		total += len(message.Content)
		if len(message.ToolCalls) > 0 {
			if b, err := json.Marshal(message.ToolCalls); err == nil {
				total += len(b)
			}
		}
		if total > maxChars {
			return true
		}
	}
	return total > maxChars
}

const hardMaxAgentHistoryMessages = 10

func (h *ChatHandler) limitAgentHistory(ctx context.Context, history []*schema.Message) []*schema.Message {
	limit := hardMaxAgentHistoryMessages
	if h.settings != nil {
		configured := h.settings.ResolveAgentConfig(ctx).MaxContextMessages
		if configured > 0 && configured < limit {
			limit = configured
		}
	}
	return latestHistoryMessages(history, limit)
}

func latestHistoryMessages(history []*schema.Message, limit int) []*schema.Message {
	if limit <= 0 || limit > hardMaxAgentHistoryMessages {
		limit = hardMaxAgentHistoryMessages
	}
	if len(history) <= limit {
		return history
	}
	return history[len(history)-limit:]
}

func (h *ChatHandler) buildMessages(ctx context.Context, history []*schema.Message, content string, _ chatDecision) (*schema.Message, []*schema.Message) {
	userMsg := schema.UserMessage(content)
	messages := make([]*schema.Message, 0, len(history)+3)
	if h.settings != nil {
		instruction := h.settings.GetPromptContent(ctx, admin.PromptAgentInstruction, "")
		if strings.TrimSpace(instruction) != "" {
			messages = append(messages, schema.SystemMessage(instruction))
		}
	}
	messages = append(messages, schema.SystemMessage(`由你判断本轮是否需要使用工具：
- 涉及产品功能、操作方法、账号、充值、规则、售后等需要事实依据的问题，调用 knowledge_search。追问时先结合历史把指代补全成适合检索的独立问题。
- knowledge_search 会返回多条候选 QA。你必须比较问题相关性和答案内容，只采用能直接支持本轮回答的候选；可以综合多条，但不要因为返回了候选就默认它们都正确相关。
- 知识库证据足够时，用自然、简短的客服口吻组织最终答复，不要暴露检索过程、分数或工具名称；证据不足时明确说明暂时没有查到，不要编造。
- 寒暄、感谢等无需业务事实的问题可以直接回复，不必检索；用户明确要求人工客服时调用 transfer_human。
- 你会收到最近最多 10 条历史消息。无论当前问题最终属于追问还是独立新问题，都要参考其中上一条用户问题后自行判断上下文关系；不要输出关系分类过程，也不要请求额外的意图判断层。
- 历史对话只用于理解本轮问题和消解指代，最终只回答本轮诉求。`))
	messages = append(messages, history...)
	messages = append(messages, userMsg)
	return userMsg, messages
}

func (h *ChatHandler) decideChat(ctx context.Context, history []*schema.Message, content string) (chatDecision, error) {
	var decision chatDecision
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return decision, nil
	}
	if h.settings == nil {
		return decision, nil
	}
	cfg := h.settings.ResolveAgentConfig(ctx)
	if exceedsContextLimit(0, cfg.MaxContextChars, history, trimmed) {
		decision.Reply = h.settings.GetPromptContent(ctx, admin.PromptChatContextLimit, cfg.DefaultReply)
		decision.UsedDefaultReply = true
		decision.addTrace("context_limit", "上下文长度检查", "blocked", "最近 10 条上下文的字符数仍超过限制，直接返回默认回复", map[string]any{
			"history_messages": len(history),
			"input_chars":      len([]rune(trimmed)),
		}, nil, nil, time.Time{})
		return decision, nil
	}
	decision.addTrace("context_limit", "上下文长度检查", "finished", "已携带最近不超过 10 条历史消息，未超过字符限制", map[string]any{
		"history_messages": len(history),
		"input_chars":      len([]rune(trimmed)),
	}, nil, nil, time.Time{})
	decision.addTrace("agent_delegation", "Agent 自主决策", "finished", "正常请求统一交给 Agent，由模型自行决定是否调用知识库或转人工工具", map[string]any{
		"query": trimmed,
	}, map[string]any{
		"route": "model_driven_agent",
	}, nil, time.Time{})
	return decision, nil
}

func (h *ChatHandler) captureUnanswered(ctx context.Context, decision chatDecision, question, finalAnswer, sessionID, requestID string, biz *middleware.BizContext) {
	if h.unanswered == nil {
		return
	}
	reason := decision.UnansweredReason
	if !decision.Unanswered {
		if len(decision.SearchHits) == 0 && !decision.KnowledgeShared {
			return
		}
		switch {
		case strings.TrimSpace(finalAnswer) == "":
			reason = unanswered.ReasonModelEmpty
		case looksUnanswered(finalAnswer):
			reason = unanswered.ReasonModelUnanswered
		default:
			return
		}
	}
	input := unanswered.CaptureInput{Question: question, SearchQuery: decision.SearchQuery, Reason: reason, FinalAnswer: finalAnswer, SessionID: sessionID, RequestID: requestID, HitCount: len(decision.SearchHits)}
	if len(decision.SearchHits) > 0 {
		input.TopScore = decision.SearchHits[0].Score
	}
	if biz != nil {
		input.CallerUserID = biz.CallerUserID
		input.ChannelID = biz.ChannelID
		input.TenantID = biz.TenantID
	}
	if err := h.unanswered.Capture(ctx, input); err != nil {
		h.reportAIError(ctx, "capture_unanswered", err.Error(), requestID, sessionID, "capture unanswered customer question")
	}
}

func looksUnanswered(answer string) bool {
	normalized := normalizeRuleText(answer)
	for _, marker := range []string{"不知道", "不清楚", "无法回答", "无法确定", "没有足够信息", "未找到相关", "知识库中没有", "建议咨询人工", "请联系人工客服"} {
		if strings.Contains(normalized, normalizeRuleText(marker)) {
			return true
		}
	}
	return false
}

func (h *ChatHandler) reportAIError(ctx context.Context, stage, message, requestID, sessionID, details string) {
	if h.aiErrors == nil || strings.TrimSpace(message) == "" {
		return
	}
	_ = h.aiErrors.Report(ctx, admin.AIErrorInput{
		Source:    "chat",
		Stage:     stage,
		Message:   message,
		RequestID: requestID,
		SessionID: sessionID,
		Details:   details,
	})
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

func containsToolCall(toolCalls []schema.ToolCall, toolName string) bool {
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	for _, call := range toolCalls {
		if strings.EqualFold(strings.TrimSpace(call.Function.Name), toolName) {
			return true
		}
	}
	return false
}

func buildMatchedQAIDs(hits []rag.SearchHit) []string {
	out := make([]string, 0, len(hits))
	for _, hit := range hits {
		if hit.ID != "" {
			out = append(out, hit.ID)
		}
	}
	return out
}

func buildSources(hits []rag.SearchHit) []sourceView {
	out := make([]sourceView, 0, len(hits))
	for _, hit := range hits {
		out = append(out, sourceView{
			ID:       hit.ID,
			Question: hit.Question,
			Category: hit.Category,
			Tags:     hit.Tags,
			Score:    hit.Score,
		})
	}
	return out
}

func messageExtra(trace []chatTraceStep) map[string]any {
	if len(trace) == 0 {
		return nil
	}
	return map[string]any{"trace": trace}
}

func persistenceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
}

func normalizeRuleText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "，", "", ",", "", "。", "", "！", "", "!", "", "？", "", "?", "", "\n", "", "\r", "", "\t", "")
	return replacer.Replace(value)
}

func logChat(c *gin.Context, event string, start time.Time, bizCtx *middleware.BizContext, sessionID, clientMessageID string) {
	logger := fmt.Sprintf("[chat] event=%s request_id=%s session_id=%s latency_ms=%d client_message_id=%s", event, c.GetString(middleware.RequestIDKey), sessionID, time.Since(start).Milliseconds(), clientMessageID)
	if bizCtx != nil {
		logger += fmt.Sprintf(" auth_type=%s caller_token_id=%s caller_user_id=%s channel_id=%s tenant_id=%s",
			bizCtx.AuthType, bizCtx.CallerTokenID, bizCtx.CallerUserID, bizCtx.ChannelID, bizCtx.TenantID)
	}
	fmt.Println(logger)
}
