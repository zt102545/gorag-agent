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
		decision.addTrace("agent_runner", "Agent 鎵ц", "finished", "妯″瀷鐢熸垚鍥炲骞跺彲鑳藉彂璧峰伐鍏疯皟鐢?, nil, map[string]any{
			"content_chars": len([]rune(content)),
			"tool_calls":    len(assistantToolCalls),
		}, nil, start)
		for _, call := range assistantToolCalls {
			decision.addTrace("tool_call", "宸ュ叿璋冪敤", "finished", call.Function.Name, map[string]any{
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
	decision.addTrace("agent_runner", "Agent 鎵ц", "finished", "妯″瀷鐢熸垚鍥炲骞跺彲鑳藉彂璧峰伐鍏疯皟鐢?, nil, map[string]any{
		"content_chars": len([]rune(answer)),
		"tool_calls":    len(toolCalls),
	}, nil, start)
	for _, call := range toolCalls {
		decision.addTrace("tool_call", "宸ュ叿璋冪敤", "finished", call.Function.Name, map[string]any{
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
	messages = append(messages, schema.SystemMessage(`鐢变綘鍒ゆ柇鏈疆鏄惁闇€瑕佷娇鐢ㄥ伐鍏凤細
- 娑夊強浜у搧鍔熻兘銆佹搷浣滄柟娉曘€佽处鍙枫€佸厖鍊笺€佽鍒欍€佸敭鍚庣瓑闇€瑕佷簨瀹炰緷鎹殑闂锛岃皟鐢?knowledge_search銆傝拷闂椂鍏堢粨鍚堝巻鍙叉妸鎸囦唬琛ュ叏鎴愰€傚悎妫€绱㈢殑鐙珛闂銆?- knowledge_search 浼氳繑鍥炲鏉″€欓€?QA銆備綘蹇呴』姣旇緝闂鐩稿叧鎬у拰绛旀鍐呭锛屽彧閲囩敤鑳界洿鎺ユ敮鎸佹湰杞洖绛旂殑鍊欓€夛紱鍙互缁煎悎澶氭潯锛屼絾涓嶈鍥犱负杩斿洖浜嗗€欓€夊氨榛樿瀹冧滑閮芥纭浉鍏炽€?- 鐭ヨ瘑搴撹瘉鎹冻澶熸椂锛岀敤鑷劧銆佺畝鐭殑瀹㈡湇鍙ｅ惢缁勭粐鏈€缁堢瓟澶嶏紝涓嶈鏆撮湶妫€绱㈣繃绋嬨€佸垎鏁版垨宸ュ叿鍚嶇О锛涜瘉鎹笉瓒虫椂鏄庣‘璇存槑鏆傛椂娌℃湁鏌ュ埌锛屼笉瑕佺紪閫犮€?- 瀵掓殑銆佹劅璋㈢瓑鏃犻渶涓氬姟浜嬪疄鐨勯棶棰樺彲浠ョ洿鎺ュ洖澶嶏紝涓嶅繀妫€绱紱鐢ㄦ埛鏄庣‘瑕佹眰浜哄伐瀹㈡湇鏃惰皟鐢?transfer_human銆?- 浣犱細鏀跺埌鏈€杩戞渶澶?10 鏉″巻鍙叉秷鎭€傛棤璁哄綋鍓嶉棶棰樻渶缁堝睘浜庤拷闂繕鏄嫭绔嬫柊闂锛岄兘瑕佸弬鑰冨叾涓笂涓€鏉＄敤鎴烽棶棰樺悗鑷鍒ゆ柇涓婁笅鏂囧叧绯伙紱涓嶈杈撳嚭鍏崇郴鍒嗙被杩囩▼锛屼篃涓嶈璇锋眰棰濆鐨勬剰鍥惧垽鏂眰銆?- 鍘嗗彶瀵硅瘽鍙敤浜庣悊瑙ｆ湰杞棶棰樺拰娑堣В鎸囦唬锛屾渶缁堝彧鍥炵瓟鏈疆璇夋眰銆俙))
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
		decision.addTrace("context_limit", "涓婁笅鏂囬暱搴︽鏌?, "blocked", "鏈€杩?10 鏉′笂涓嬫枃鐨勫瓧绗︽暟浠嶈秴杩囬檺鍒讹紝鐩存帴杩斿洖榛樿鍥炲", map[string]any{
			"history_messages": len(history),
			"input_chars":      len([]rune(trimmed)),
		}, nil, nil, time.Time{})
		return decision, nil
	}
	decision.addTrace("context_limit", "涓婁笅鏂囬暱搴︽鏌?, "finished", "宸叉惡甯︽渶杩戜笉瓒呰繃 10 鏉″巻鍙叉秷鎭紝鏈秴杩囧瓧绗﹂檺鍒?, map[string]any{
		"history_messages": len(history),
		"input_chars":      len([]rune(trimmed)),
	}, nil, nil, time.Time{})
	decision.addTrace("agent_delegation", "Agent 鑷富鍐崇瓥", "finished", "姝ｅ父璇锋眰缁熶竴浜ょ粰 Agent锛岀敱妯″瀷鑷鍐冲畾鏄惁璋冪敤鐭ヨ瘑搴撴垨杞汉宸ュ伐鍏?, map[string]any{
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
	for _, marker := range []string{"涓嶇煡閬?, "涓嶆竻妤?, "鏃犳硶鍥炵瓟", "鏃犳硶纭畾", "娌℃湁瓒冲淇℃伅", "鏈壘鍒扮浉鍏?, "鐭ヨ瘑搴撲腑娌℃湁", "寤鸿鍜ㄨ浜哄伐", "璇疯仈绯讳汉宸ュ鏈?} {
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
	replacer := strings.NewReplacer(" ", "", "锛?, "", ",", "", "銆?, "", "锛?, "", "!", "", "锛?, "", "?", "", "\n", "", "\r", "", "\t", "")
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
