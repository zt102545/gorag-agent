package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const queryRelationConfidenceThreshold = 0.65

// QueryRelationResult describes whether the current question depends on the
// preceding completed conversation turn. StandaloneQuery is only used for
// retrieval; the answer model must still answer the current question only.
type QueryRelationResult struct {
	IsFollowUp      bool    `json:"is_follow_up"`
	StandaloneQuery string  `json:"standalone_query"`
	Confidence      float64 `json:"confidence"`
	Reason          string  `json:"reason"`
}

type queryRelationInput struct {
	PreviousQuestion string `json:"previous_question"`
	PreviousAnswer   string `json:"previous_answer"`
	CurrentQuestion  string `json:"current_question"`
}

// ResolveQueryRelation uses the chat model to distinguish a real follow-up
// from a new question. On uncertainty it deliberately keeps only the current
// question so a previous topic cannot contaminate retrieval.
func (a *IntentAnalyzer) ResolveQueryRelation(ctx context.Context, history []*schema.Message, current string) (*QueryRelationResult, error) {
	current = strings.TrimSpace(current)
	result := &QueryRelationResult{
		StandaloneQuery: current,
		Confidence:      1,
		Reason:          "no completed previous turn",
	}
	if current == "" {
		return result, nil
	}
	previousQuestion, previousAnswer := lastCompletedTurn(history)
	if previousQuestion == "" || previousAnswer == "" {
		return result, nil
	}

	payload, err := json.Marshal(queryRelationInput{
		PreviousQuestion: limitRunes(previousQuestion, 1200),
		PreviousAnswer:   limitRunes(previousAnswer, 2000),
		CurrentQuestion:  current,
	})
	if err != nil {
		return result, err
	}
	m, err := a.newModel(ctx)
	if err != nil {
		return result, err
	}
	msg, err := m.Generate(ctx, []*schema.Message{
		schema.SystemMessage(queryRelationPrompt),
		schema.UserMessage(string(payload)),
	})
	if err != nil {
		return result, err
	}
	parsed, err := parseQueryRelationJSON(msg.Content)
	if err != nil {
		return result, err
	}
	parsed.Reason = strings.TrimSpace(parsed.Reason)
	parsed.StandaloneQuery = strings.TrimSpace(parsed.StandaloneQuery)

	// A complete, independent question must never be rewritten with old text.
	// Low-confidence follow-up judgments use the same safe fallback.
	if !parsed.IsFollowUp || parsed.Confidence < queryRelationConfidenceThreshold || parsed.StandaloneQuery == "" {
		parsed.IsFollowUp = false
		parsed.StandaloneQuery = current
	}
	if parsed.Confidence <= 0 {
		parsed.Confidence = 0.5
	}
	return parsed, nil
}

func parseQueryRelationJSON(content string) (*QueryRelationResult, error) {
	content = strings.TrimSpace(content)
	if i := strings.Index(content, "{"); i >= 0 {
		if j := strings.LastIndex(content, "}"); j > i {
			content = content[i : j+1]
		}
	}
	var result QueryRelationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse query relation json: %w", err)
	}
	return &result, nil
}

func lastCompletedTurn(history []*schema.Message) (string, string) {
	for assistantIndex := len(history) - 1; assistantIndex >= 0; assistantIndex-- {
		assistant := history[assistantIndex]
		if assistant == nil || assistant.Role != schema.Assistant || strings.TrimSpace(assistant.Content) == "" {
			continue
		}
		for userIndex := assistantIndex - 1; userIndex >= 0; userIndex-- {
			user := history[userIndex]
			if user == nil || user.Role != schema.User || strings.TrimSpace(user.Content) == "" {
				continue
			}
			return strings.TrimSpace(user.Content), strings.TrimSpace(assistant.Content)
		}
	}
	return "", ""
}

func limitRunes(value string, max int) string {
	runes := []rune(value)
	if max <= 0 || len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

const queryRelationPrompt = `你是多轮客服对话的上下文关系判定器。输入是 JSON，包含上一轮用户问题、上一轮客服回答和当前用户问题。
只输出合法 JSON：{"is_follow_up":true/false,"standalone_query":"...","confidence":0.0-1.0,"reason":"..."}

判定规则：
1. 当前问题省略了对象，使用“这个、它、那、多久、为什么、怎么做”等指代，并且必须依赖上一轮才能理解时，才是追问。
2. 当前问题已经有完整的主体和诉求，即使主题相近，也优先判为新问题。
3. 新问题的 standalone_query 必须原样保留当前问题，禁止拼接上一问。
4. 追问的 standalone_query 只补齐理解当前问题所必需的对象，写成一条独立问题；禁止把上一问和当前问题并列拼接，禁止要求再次回答上一问。
5. 不要根据上一轮回答继续作答，你只负责关系判定和检索问题改写。`
