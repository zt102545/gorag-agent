package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type PromptResolver interface {
	GetPromptContent(ctx context.Context, key, fallback string) string
}

type ModelFactory func(ctx context.Context) (model.ToolCallingChatModel, error)

type IntentResult struct {
	Intent     string  `json:"intent"`
	NeedSearch bool    `json:"need_search"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type IntentAnalyzer struct {
	prompts   PromptResolver
	newModel  ModelFactory
	key       string
	threshold float64
}

func NewIntentAnalyzer(prompts PromptResolver, newModel ModelFactory, promptKey string, threshold float64) *IntentAnalyzer {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.6
	}
	if promptKey == "" {
		promptKey = "intent_route"
	}
	return &IntentAnalyzer{
		prompts:   prompts,
		newModel:  newModel,
		key:       promptKey,
		threshold: threshold,
	}
}

func (a *IntentAnalyzer) Analyze(ctx context.Context, query string) (*IntentResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return &IntentResult{Intent: "empty", NeedSearch: false, Confidence: 1, Reason: "empty query"}, nil
	}
	tmpl := a.prompts.GetPromptContent(ctx, a.key, defaultIntentPrompt)
	prompt := strings.ReplaceAll(tmpl, "{{query}}", query)

	m, err := a.newModel(ctx)
	if err != nil {
		return nil, err
	}
	msg, err := m.Generate(ctx, []*schema.Message{
		schema.SystemMessage("你只能输出合法 JSON，不要输出 markdown。"),
		schema.UserMessage(prompt),
	})
	if err != nil {
		return nil, err
	}
	out, err := parseIntentJSON(msg.Content)
	if err != nil {
		return nil, err
	}
	if out.Confidence <= 0 {
		out.Confidence = 0.5
	}
	// Apply threshold gate: low confidence means "do not search" unless explicitly high.
	if out.NeedSearch && out.Confidence < a.threshold {
		out.NeedSearch = false
		if strings.TrimSpace(out.Reason) == "" {
			out.Reason = fmt.Sprintf("confidence %.2f < %.2f", out.Confidence, a.threshold)
		} else {
			out.Reason = fmt.Sprintf("%s (confidence %.2f < %.2f)", out.Reason, out.Confidence, a.threshold)
		}
	}
	return out, nil
}

func parseIntentJSON(content string) (*IntentResult, error) {
	content = strings.TrimSpace(content)
	if i := strings.Index(content, "{"); i >= 0 {
		if j := strings.LastIndex(content, "}"); j > i {
			content = content[i : j+1]
		}
	}
	var out IntentResult
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	out.Intent = strings.TrimSpace(out.Intent)
	out.Reason = strings.TrimSpace(out.Reason)
	if out.Intent == "" {
		out.Intent = "unknown"
	}
	return &out, nil
}

const defaultIntentPrompt = `你是客服问题意图分析器。你要判断：这个用户问题是否需要检索 QA 知识库（knowledge_search）。

只输出 JSON：{"intent":"...","need_search":true/false,"confidence":0.0-1.0,"reason":"..."}

判定原则（举例）：
- 需要检索（need_search=true）：产品规则/售后/退款/配送/发票/会员权益/使用说明/常见问题/政策条款/对比选择 等需要“有标准答案”的问题
- 不需要检索（need_search=false）：闲聊寒暄、情绪宣泄但无具体问题、仅说谢谢/再见、纯粹让你复述、与客服无关的开放闲聊、让你编故事/写诗、仅要求转人工（此类可直接走转人工流程）

用户问题：{{query}}`
