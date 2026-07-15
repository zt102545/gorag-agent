package handler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/yourorg/agent_customer_service/internal/graph/rag"
)

func TestBuildMessages_DelegatesToolChoiceToAgent(t *testing.T) {
	h := &ChatHandler{}
	history := []*schema.Message{
		schema.UserMessage("第一个问题"),
		schema.AssistantMessage("第一个问题的回答", nil),
	}
	_, messages := h.buildMessages(context.Background(), history, "第二个问题", chatDecision{})
	var systemText strings.Builder
	for _, message := range messages {
		if message != nil && message.Role == schema.System {
			systemText.WriteString(message.Content)
			systemText.WriteByte('\n')
		}
	}
	got := systemText.String()
	for _, want := range []string{"由你判断本轮是否需要使用工具", "调用 knowledge_search", "返回多条候选 QA", "用户明确要求人工客服时调用 transfer_human", "最终只回答本轮诉求"} {
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

func TestCollectAgentOutput_MergesStreamedToolCallFragments(t *testing.T) {
	stream := schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: ptrInt(0), ID: "call-1", Type: "function", Function: schema.FunctionCall{Name: "knowledge_search"}}}},
		{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: ptrInt(0), Function: schema.FunctionCall{Arguments: `{"query":"怎么充值"}`}}}},
	})
	event := adk.EventFromMessage(nil, stream, schema.Assistant, "")
	var content strings.Builder
	var toolCalls []schema.ToolCall

	if err := collectAgentOutput(event, &content, &toolCalls); err != nil {
		t.Fatal(err)
	}
	if len(toolCalls) != 1 {
		t.Fatalf("tool calls = %d, want 1: %+v", len(toolCalls), toolCalls)
	}
	if toolCalls[0].Function.Name != "knowledge_search" || toolCalls[0].Function.Arguments != `{"query":"怎么充值"}` {
		t.Fatalf("tool call was not merged: %+v", toolCalls[0])
	}
}

func ptrInt(value int) *int { return &value }

func TestToolSearchCollector_DeduplicatesObservedHits(t *testing.T) {
	collector := &toolSearchCollector{}
	collector.Observe("第一次检索", []rag.SearchHit{{ID: "qa-1"}, {ID: "qa-2"}})
	collector.Observe("补充检索", []rag.SearchHit{{ID: "qa-2"}, {ID: "qa-3"}})

	query, hits := collector.Snapshot()
	if query != "补充检索" || len(hits) != 3 {
		t.Fatalf("query=%q hits=%+v", query, hits)
	}
}

func TestLatestHistoryMessages_CapsAtTenAndKeepsPreviousUserQuestion(t *testing.T) {
	history := make([]*schema.Message, 0, 12)
	for i := 0; i < 5; i++ {
		history = append(history, schema.UserMessage(fmt.Sprintf("问题%d", i)))
		history = append(history, schema.AssistantMessage(fmt.Sprintf("回答%d", i), nil))
	}
	history = append(history, schema.UserMessage("上一次用户问题"))
	history = append(history, schema.AssistantMessage("上一次回答", nil))

	got := latestHistoryMessages(history, 20)
	if len(got) != hardMaxAgentHistoryMessages {
		t.Fatalf("history length=%d, want %d", len(got), hardMaxAgentHistoryMessages)
	}
	if got[len(got)-2].Content != "上一次用户问题" {
		t.Fatalf("previous user question was not preserved: %+v", got)
	}
}
