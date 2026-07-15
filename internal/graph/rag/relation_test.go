package rag

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type relationModel struct {
	content string
	calls   int
}

func (m *relationModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.calls++
	return schema.AssistantMessage(m.content, nil), nil
}

func (m *relationModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("not implemented")
}

func (m *relationModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func TestResolveQueryRelation_FollowUpUsesStandaloneQuery(t *testing.T) {
	fake := &relationModel{content: `{"is_follow_up":true,"standalone_query":"单分镜改写功能如何收费？","confidence":0.92,"reason":"省略了功能主体"}`}
	analyzer := NewIntentAnalyzer(nil, func(context.Context) (model.ToolCallingChatModel, error) { return fake, nil }, "", 0.6)
	history := []*schema.Message{
		schema.UserMessage("单分镜改写功能有什么用？"),
		schema.AssistantMessage("它用于只改写指定分镜。", nil),
	}

	result, err := analyzer.ResolveQueryRelation(context.Background(), history, "那如何收费？")
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsFollowUp || result.StandaloneQuery != "单分镜改写功能如何收费？" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestResolveQueryRelation_IndependentQuestionCannotContainPreviousQuestion(t *testing.T) {
	fake := &relationModel{content: `{"is_follow_up":false,"standalone_query":"第一个问题；第二个问题","confidence":0.98,"reason":"主题不同"}`}
	analyzer := NewIntentAnalyzer(nil, func(context.Context) (model.ToolCallingChatModel, error) { return fake, nil }, "", 0.6)
	history := []*schema.Message{
		schema.UserMessage("第一个问题"),
		schema.AssistantMessage("第一个回答", nil),
	}

	result, err := analyzer.ResolveQueryRelation(context.Background(), history, "第二个问题")
	if err != nil {
		t.Fatal(err)
	}
	if result.IsFollowUp || result.StandaloneQuery != "第二个问题" {
		t.Fatalf("independent question was contaminated: %#v", result)
	}
}

func TestResolveQueryRelation_LowConfidenceFallsBackToCurrentQuestion(t *testing.T) {
	fake := &relationModel{content: `{"is_follow_up":true,"standalone_query":"旧问题和当前问题","confidence":0.51,"reason":"不确定"}`}
	analyzer := NewIntentAnalyzer(nil, func(context.Context) (model.ToolCallingChatModel, error) { return fake, nil }, "", 0.6)
	history := []*schema.Message{
		schema.UserMessage("旧问题"),
		schema.AssistantMessage("旧回答", nil),
	}

	result, err := analyzer.ResolveQueryRelation(context.Background(), history, "当前问题")
	if err != nil {
		t.Fatal(err)
	}
	if result.IsFollowUp || result.StandaloneQuery != "当前问题" {
		t.Fatalf("low-confidence result did not use safe fallback: %#v", result)
	}
}

func TestResolveQueryRelation_NoCompletedTurnSkipsModel(t *testing.T) {
	fake := &relationModel{content: `{}`}
	analyzer := NewIntentAnalyzer(nil, func(context.Context) (model.ToolCallingChatModel, error) { return fake, nil }, "", 0.6)

	result, err := analyzer.ResolveQueryRelation(context.Background(), nil, "当前问题")
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 0 || result.StandaloneQuery != "当前问题" || result.IsFollowUp {
		t.Fatalf("unexpected result: calls=%d result=%#v", fake.calls, result)
	}
}
