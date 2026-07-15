package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/yourorg/agent_customer_service/internal/graph/rag"
)

func TestBuildMessages_FocusesOnlyCurrentIndependentQuestion(t *testing.T) {
	h := &ChatHandler{}
	history := []*schema.Message{
		schema.UserMessage("第一个问题"),
		schema.AssistantMessage("第一个问题的回答", nil),
	}
	decision := chatDecision{
		QueryRelation: &rag.QueryRelationResult{IsFollowUp: false, StandaloneQuery: "第二个问题", Confidence: 0.9},
		SearchQuery:   "第二个问题",
		SearchHits:    []rag.SearchHit{{Question: "第二个问题", Answer: "第二个回答"}},
	}

	_, messages := h.buildMessages(context.Background(), history, "第二个问题", decision)
	var systemText strings.Builder
	for _, message := range messages {
		if message != nil && message.Role == schema.System {
			systemText.WriteString(message.Content)
			systemText.WriteByte('\n')
		}
	}
	got := systemText.String()
	for _, want := range []string{"本轮用户问题是：第二个问题", "上下文关系：新问题", "不要再次回答、复述或总结上一轮", "不得凭常识推断价格、优惠、期限、规则或功能"} {
		if !strings.Contains(got, want) {
			t.Fatalf("system messages missing %q: %s", want, got)
		}
	}
}

func TestCollectAgentOutput_ReadsStreamedAssistantMessages(t *testing.T) {
	stream := schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Assistant, Content: "只回答"},
		{Role: schema.Assistant, Content: "第二问"},
		{Role: schema.Tool, Content: "tool output"},
	})
	event := adk.EventFromMessage(nil, stream, schema.Assistant, "")
	var content strings.Builder
	var toolCalls []schema.ToolCall

	if err := collectAgentOutput(event, &content, &toolCalls); err != nil {
		t.Fatal(err)
	}
	if content.String() != "只回答第二问" {
		t.Fatalf("content = %q", content.String())
	}
}
