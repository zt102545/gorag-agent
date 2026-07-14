package appscope

import "context"

const (
	DefaultPromptGroupID   = "00000000-0000-0000-0000-000000000001"
	DefaultKnowledgeBaseID = "00000000-0000-0000-0000-000000000002"
)

type Scope struct {
	PromptGroupID   string
	KnowledgeBaseID string
}

type contextKey struct{}

func WithContext(ctx context.Context, scope Scope) context.Context {
	if scope.PromptGroupID == "" {
		scope.PromptGroupID = DefaultPromptGroupID
	}
	if scope.KnowledgeBaseID == "" {
		scope.KnowledgeBaseID = DefaultKnowledgeBaseID
	}
	return context.WithValue(ctx, contextKey{}, scope)
}

func FromContext(ctx context.Context) Scope {
	scope, _ := ctx.Value(contextKey{}).(Scope)
	if scope.PromptGroupID == "" {
		scope.PromptGroupID = DefaultPromptGroupID
	}
	if scope.KnowledgeBaseID == "" {
		scope.KnowledgeBaseID = DefaultKnowledgeBaseID
	}
	return scope
}
