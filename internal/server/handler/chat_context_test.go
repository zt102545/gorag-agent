package handler

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/gorag-ai/gorag/internal/graph/rag"
)

func TestBuildMessages_DelegatesToolChoiceToAgent(t *testing.T) {
	h := &ChatHandler{}
	history := []*schema.Message{
		schema.UserMessage("绗竴涓棶棰?),
		schema.AssistantMessage("绗竴涓棶棰樼殑鍥炵瓟", nil),
	}
	_, messages := h.buildMessages(context.Background(), history, "绗簩涓棶棰?, chatDecision{})
	var systemText strings.Builder
	for _, message := range messages {
		if message != nil && message.Role == schema.System {
			systemText.WriteString(message.Content)
			systemText.WriteByte('\n')
		}
	}
	got := systemText.String()
	for _, want := range []string{"鐢变綘鍒ゆ柇鏈疆鏄惁闇€瑕佷娇鐢ㄥ伐鍏?, "璋冪敤 knowledge_search", "杩斿洖澶氭潯鍊欓€?QA", "鐢ㄦ埛鏄庣‘瑕佹眰浜哄伐瀹㈡湇鏃惰皟鐢?transfer_human", "鏈€缁堝彧鍥炵瓟鏈疆璇夋眰"} {
		if !strings.Contains(got, want) {
			t.Fatalf("system messages missing %q: %s", want, got)
		}
	}
}

func TestCollectAgentOutput_ReadsStreamedAssistantMessages(t *testing.T) {
	stream := schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Assistant, Content: "鍙洖绛?},
		{Role: schema.Assistant, Content: "绗簩闂?},
		{Role: schema.Tool, Content: "tool output"},
	})
	event := adk.EventFromMessage(nil, stream, schema.Assistant, "")
	var content strings.Builder
	var toolCalls []schema.ToolCall

	if err := collectAgentOutput(event, &content, &toolCalls); err != nil {
		t.Fatal(err)
	}
	if content.String() != "鍙洖绛旂浜岄棶" {
		t.Fatalf("content = %q", content.String())
	}
}

func TestCollectAgentOutput_MergesStreamedToolCallFragments(t *testing.T) {
	stream := schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: ptrInt(0), ID: "call-1", Type: "function", Function: schema.FunctionCall{Name: "knowledge_search"}}}},
		{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{Index: ptrInt(0), Function: schema.FunctionCall{Arguments: `{"query":"鎬庝箞鍏呭€?}`}}}},
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
	if toolCalls[0].Function.Name != "knowledge_search" || toolCalls[0].Function.Arguments != `{"query":"鎬庝箞鍏呭€?}` {
		t.Fatalf("tool call was not merged: %+v", toolCalls[0])
	}
}

func ptrInt(value int) *int { return &value }

func TestToolSearchCollector_DeduplicatesObservedHits(t *testing.T) {
	collector := &toolSearchCollector{}
	collector.Observe("绗竴娆℃绱?, []rag.SearchHit{{ID: "qa-1"}, {ID: "qa-2"}})
	collector.Observe("琛ュ厖妫€绱?, []rag.SearchHit{{ID: "qa-2"}, {ID: "qa-3"}})

	query, hits := collector.Snapshot()
	if query != "琛ュ厖妫€绱? || len(hits) != 3 {
		t.Fatalf("query=%q hits=%+v", query, hits)
	}
}

func TestLatestHistoryMessages_CapsAtTenAndKeepsPreviousUserQuestion(t *testing.T) {
	history := make([]*schema.Message, 0, 12)
	for i := 0; i < 5; i++ {
		history = append(history, schema.UserMessage(fmt.Sprintf("闂%d", i)))
		history = append(history, schema.AssistantMessage(fmt.Sprintf("鍥炵瓟%d", i), nil))
	}
	history = append(history, schema.UserMessage("涓婁竴娆＄敤鎴烽棶棰?))
	history = append(history, schema.AssistantMessage("涓婁竴娆″洖绛?, nil))

	got := latestHistoryMessages(history, 20)
	if len(got) != hardMaxAgentHistoryMessages {
		t.Fatalf("history length=%d, want %d", len(got), hardMaxAgentHistoryMessages)
	}
	if got[len(got)-2].Content != "涓婁竴娆＄敤鎴烽棶棰? {
		t.Fatalf("previous user question was not preserved: %+v", got)
	}
}
