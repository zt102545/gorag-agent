package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/admin"
	settingsapp "github.com/yourorg/agent_customer_service/internal/application/settings"
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
	ragService   *rag.Service
	intent       *rag.IntentAnalyzer
	settings     *settingsapp.Service
	aiErrors     *admin.AIErrorService
}

func NewChatHandler(runner *adk.Runner, sessionStore session.Store, sseWriter *sse.Writer, ragService *rag.Service, intent *rag.IntentAnalyzer, settings *settingsapp.Service, aiErrors *admin.AIErrorService) *ChatHandler {
	return &ChatHandler{
		runner:       runner,
		sessionStore: sessionStore,
		sseWriter:    sseWriter,
		ragService:   ragService,
		intent:       intent,
		settings:     settings,
		aiErrors:     aiErrors,
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
	Intent           *rag.IntentResult
	SearchQuery      string
	SearchHits       []rag.SearchHit
	Trace            []chatTraceStep
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

	iter := h.runner.Run(c.Request.Context(), messages, adk.WithCheckPointID(sessionID))

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
		if err := h.sseWriter.WriteWithObserver(c.Writer, event, func(msg *schema.Message) {
			if msg.Role == schema.Assistant {
				assistantContent.WriteString(msg.Content)
				if len(msg.ToolCalls) > 0 {
					assistantToolCalls = append(assistantToolCalls, msg.ToolCalls...)
				}
			}
		}); err != nil {
			break
		}
		c.Writer.Flush()
	}

	content := strings.TrimSpace(assistantContent.String())
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

	history := convertHistory(req.History)
	decision, err := h.decideChat(c.Request.Context(), history, req.Question)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	if decision.Reply != "" {
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
	iter := h.runner.Run(c.Request.Context(), messages)
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
		"knowledge_shared": decision.KnowledgeShared,
	})
}

func (h *ChatHandler) canHandleChat(c *gin.Context) bool {
	if h.settings == nil {
		return true
	}
	if !h.settings.ChatConfigured(c.Request.Context()) {
		response.Error(c, http.StatusServiceUnavailable, "chat model is not configured in GM")
		return false
	}
	if !h.settings.EmbeddingConfigured(c.Request.Context()) {
		response.Error(c, http.StatusServiceUnavailable, "embedding model is not configured in GM")
		return false
	}
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

func (h *ChatHandler) buildMessages(ctx context.Context, history []*schema.Message, content string, decision chatDecision) (*schema.Message, []*schema.Message) {
	userMsg := schema.UserMessage(content)
	messages := history
	if decision.Intent != nil {
		hint := fmt.Sprintf("意图分析结果：intent=%s need_search=%t confidence=%.2f reason=%s。请根据 need_search 决定是否调用 knowledge_search；need_search=false 时不要调用。",
			decision.Intent.Intent, decision.Intent.NeedSearch, decision.Intent.Confidence, decision.Intent.Reason)
		messages = append(messages, schema.SystemMessage(hint))
	}
	if len(decision.SearchHits) > 0 {
		contextPrompt := "你是客服助手。请严格根据以下知识库内容回答用户问题；若知识库无法直接支持答案，不要编造。"
		if h.settings != nil {
			contextPrompt = h.settings.GetPromptContent(ctx, admin.PromptRAGAnswer, contextPrompt)
		}
		contextPrompt = strings.NewReplacer(
			"{{context}}", rag.FormatHits(decision.SearchHits),
			"{{query}}", decision.SearchQuery,
		).Replace(contextPrompt)
		messages = append(messages, schema.SystemMessage(contextPrompt))
	}
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
	if exceedsContextLimit(cfg.MaxContextMessages, cfg.MaxContextChars, history, trimmed) {
		decision.Reply = h.settings.GetPromptContent(ctx, admin.PromptChatContextLimit, cfg.DefaultReply)
		decision.UsedDefaultReply = true
		decision.addTrace("context_limit", "上下文长度检查", "blocked", "超过配置的上下文限制，直接返回默认回复", map[string]any{
			"history_messages": len(history),
			"input_chars":      len([]rune(trimmed)),
		}, nil, nil, time.Time{})
		return decision, nil
	}
	decision.addTrace("context_limit", "上下文长度检查", "finished", "未超过上下文限制", map[string]any{
		"history_messages": len(history),
		"input_chars":      len([]rune(trimmed)),
	}, nil, nil, time.Time{})
	ruleStart := time.Now()
	if ruleReply, needHuman, matched, err := h.applyLightRules(ctx, trimmed); err != nil {
		decision.addTrace("light_rules", "轻量规则匹配", "error", "读取或匹配轻量规则失败", map[string]any{
			"query": trimmed,
		}, nil, err, ruleStart)
		return decision, err
	} else if matched {
		decision.Reply = ruleReply
		decision.NeedHuman = needHuman
		decision.addTrace("light_rules", "轻量规则匹配", "matched", "命中轻量规则，直接返回规则回复", map[string]any{
			"query": trimmed,
		}, map[string]any{
			"need_human":  needHuman,
			"reply_chars": len([]rune(ruleReply)),
		}, nil, ruleStart)
		return decision, nil
	} else {
		decision.addTrace("light_rules", "轻量规则匹配", "finished", "未命中轻量规则，继续意图分析", map[string]any{
			"query": trimmed,
		}, nil, nil, ruleStart)
	}

	intentStart := time.Now()
	intentRes, err := h.analyzeIntent(ctx, trimmed)
	if err != nil {
		h.reportAIError(ctx, "intent_analyzer", err.Error(), "", "", "decide chat analyze")
		decision.addTrace("intent_analyzer", "意图分析", "error", "意图分析失败，继续尝试检索", map[string]any{
			"query": trimmed,
		}, nil, err, intentStart)
	} else {
		decision.Intent = intentRes
		output := map[string]any{"available": intentRes != nil}
		if intentRes != nil {
			output = map[string]any{
				"intent":      intentRes.Intent,
				"need_search": intentRes.NeedSearch,
				"confidence":  intentRes.Confidence,
				"reason":      intentRes.Reason,
			}
		}
		decision.addTrace("intent_analyzer", "意图分析", "finished", "完成用户问题意图判断", map[string]any{
			"query": trimmed,
		}, output, nil, intentStart)
	}
	if decision.Intent != nil && !decision.Intent.NeedSearch {
		decision.Reply = h.settings.GetPromptContent(ctx, admin.PromptChatOutOfScope, cfg.DefaultReply)
		decision.UsedDefaultReply = true
		decision.addTrace("route", "路由决策", "finished", "意图分析认为不需要检索，使用默认回复", nil, map[string]any{
			"route": "default_reply",
		}, nil, time.Time{})
		return decision, nil
	}

	rewriteStart := time.Now()
	searchQuery := rag.RewriteQuery(history, trimmed)
	if searchQuery == "" {
		searchQuery = trimmed
	}
	decision.SearchQuery = searchQuery
	decision.addTrace("query_rewrite", "检索 Query 改写", "finished", "生成知识库检索语句", map[string]any{
		"query": trimmed,
	}, map[string]any{
		"search_query": searchQuery,
		"rewritten":    searchQuery != trimmed,
	}, nil, rewriteStart)
	searchStart := time.Now()
	hits, err := h.searchKnowledge(ctx, searchQuery)
	if err != nil {
		h.reportAIError(ctx, "knowledge_search", err.Error(), "", "", "decide chat presearch")
		decision.addTrace("knowledge_search", "知识库检索", "error", "检索失败，继续交给后续流程处理", map[string]any{
			"query": searchQuery,
		}, nil, err, searchStart)
		return decision, nil
	}
	decision.SearchHits = hits
	decision.addTrace("knowledge_search", "知识库检索", "finished", "完成 QA 知识库检索", map[string]any{
		"query": searchQuery,
	}, map[string]any{
		"hit_count": len(hits),
		"sources":   buildSources(hits),
	}, nil, searchStart)
	if len(hits) == 0 {
		decision.Reply = h.settings.GetPromptContent(ctx, admin.PromptChatNoHit, cfg.DefaultReply)
		decision.UsedDefaultReply = true
		decision.addTrace("route", "路由决策", "finished", "知识库无命中，使用无命中默认回复", nil, map[string]any{
			"route": "no_hit_reply",
		}, nil, time.Time{})
		return decision, nil
	}
	decision.KnowledgeShared = true
	hitLevel := evaluateHitLevel(searchQuery, hits)
	decision.addTrace("hit_evaluation", "命中质量评估", "finished", "根据问题与首条命中相似度选择回复路径", nil, map[string]any{
		"level":        hitLevel,
		"top_question": hits[0].Question,
		"top_score":    hits[0].Score,
	}, nil, time.Time{})
	switch hitLevel {
	case "high":
		decision.Reply = strings.TrimSpace(hits[0].Answer)
		if decision.Reply == "" {
			decision.Reply = h.settings.GetPromptContent(ctx, admin.PromptChatNoHit, cfg.DefaultReply)
			decision.UsedDefaultReply = true
			decision.addTrace("route", "路由决策", "finished", "高置信命中没有答案，使用默认回复", nil, map[string]any{
				"route": "empty_answer_default_reply",
			}, nil, time.Time{})
		} else {
			decision.addTrace("route", "路由决策", "finished", "高置信命中，直接返回标准答案", nil, map[string]any{
				"route": "direct_qa_answer",
			}, nil, time.Time{})
		}
	case "mid":
		// Let the model answer based on retrieved QA context.
		decision.addTrace("route", "路由决策", "finished", "中等命中，交给模型基于检索上下文回答", nil, map[string]any{
			"route": "agent_with_rag_context",
		}, nil, time.Time{})
	default:
		decision.Reply = h.settings.GetPromptContent(ctx, admin.PromptChatNoHit, cfg.DefaultReply)
		decision.UsedDefaultReply = true
		decision.addTrace("route", "路由决策", "finished", "低置信命中，使用无命中默认回复", nil, map[string]any{
			"route": "low_confidence_reply",
		}, nil, time.Time{})
	}
	return decision, nil
}

func (h *ChatHandler) applyLightRules(ctx context.Context, content string) (reply string, needHuman bool, matched bool, err error) {
	if h.settings == nil {
		return "", false, false, nil
	}
	rules, err := h.settings.ListChatLightRules(ctx, true)
	if err != nil {
		return "", false, false, err
	}
	for _, rule := range rules {
		if !matchLightRule(rule, content) {
			continue
		}
		switch rule.ActionType {
		case admin.ChatRuleActionReply:
			reply = strings.TrimSpace(rule.ResponseText)
			if reply == "" {
				reply = h.settings.GetPromptContent(ctx, admin.PromptChatSmalltalk, "你好，你可以继续咨询订单、退款、配送、发票、售后等问题。")
			}
			return reply, false, true, nil
		case admin.ChatRuleActionHandoff:
			reply = strings.TrimSpace(rule.ResponseText)
			if reply == "" {
				reply = h.settings.GetPromptContent(ctx, admin.PromptChatHandoff, "已为你记录人工客服诉求，请稍候。")
			}
			return reply, true, true, nil
		case admin.ChatRuleActionReject:
			reply = strings.TrimSpace(rule.ResponseText)
			if reply == "" {
				reply = h.settings.GetPromptContent(ctx, admin.PromptChatOutOfScope, "当前我主要支持客服业务问题。")
			}
			return reply, false, true, nil
		case admin.ChatRuleActionContinueSearch:
			return "", false, false, nil
		}
	}
	return "", false, false, nil
}

func (h *ChatHandler) analyzeIntent(ctx context.Context, content string) (*rag.IntentResult, error) {
	if h.intent == nil {
		return nil, nil
	}
	return h.intent.Analyze(ctx, content)
}

func (h *ChatHandler) searchKnowledge(ctx context.Context, query string) ([]rag.SearchHit, error) {
	if h.ragService == nil {
		return nil, nil
	}
	return h.ragService.SearchHits(ctx, query)
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

func containsTransferTool(toolCalls []schema.ToolCall) bool {
	for _, call := range toolCalls {
		if strings.Contains(strings.ToLower(call.Function.Name), "transfer") {
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

func matchLightRule(rule admin.ChatLightRuleView, content string) bool {
	normalizedContent := normalizeRuleText(content)
	pattern := strings.TrimSpace(rule.Pattern)
	switch rule.MatchType {
	case admin.ChatRuleMatchExact:
		return normalizedContent == normalizeRuleText(pattern)
	case admin.ChatRuleMatchContains:
		return strings.Contains(normalizedContent, normalizeRuleText(pattern))
	case admin.ChatRuleMatchRegex:
		re, err := regexp.Compile(pattern)
		return err == nil && re.MatchString(content)
	case admin.ChatRuleMatchKeywords:
		for _, keyword := range strings.Split(pattern, ",") {
			if strings.Contains(normalizedContent, normalizeRuleText(keyword)) {
				return true
			}
		}
	}
	return false
}

func normalizeRuleText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "，", "", ",", "", "。", "", "！", "", "!", "", "？", "", "?", "", "\n", "", "\r", "", "\t", "")
	return replacer.Replace(value)
}

func evaluateHitLevel(query string, hits []rag.SearchHit) string {
	if len(hits) == 0 {
		return "low"
	}
	top := normalizeRuleText(hits[0].Question)
	q := normalizeRuleText(query)
	if top == q {
		return "high"
	}
	if len(q) >= 4 && (strings.Contains(top, q) || strings.Contains(q, top)) {
		return "high"
	}
	if overlapRatio(q, top) >= 0.6 {
		return "high"
	}
	return "mid"
}

func overlapRatio(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	set := make(map[rune]struct{})
	for _, r := range a {
		set[r] = struct{}{}
	}
	var overlap int
	seen := make(map[rune]struct{})
	for _, r := range b {
		if _, ok := set[r]; ok {
			if _, dup := seen[r]; !dup {
				overlap++
				seen[r] = struct{}{}
			}
		}
	}
	denom := len([]rune(a))
	if len([]rune(b)) < denom {
		denom = len([]rune(b))
	}
	if denom == 0 {
		return 0
	}
	return float64(overlap) / float64(denom)
}

func logChat(c *gin.Context, event string, start time.Time, bizCtx *middleware.BizContext, sessionID, clientMessageID string) {
	logger := fmt.Sprintf("[chat] event=%s request_id=%s session_id=%s latency_ms=%d client_message_id=%s", event, c.GetString(middleware.RequestIDKey), sessionID, time.Since(start).Milliseconds(), clientMessageID)
	if bizCtx != nil {
		logger += fmt.Sprintf(" auth_type=%s caller_token_id=%s caller_user_id=%s channel_id=%s tenant_id=%s",
			bizCtx.AuthType, bizCtx.CallerTokenID, bizCtx.CallerUserID, bizCtx.ChannelID, bizCtx.TenantID)
	}
	fmt.Println(logger)
}
