package settingsapp

import (
	"context"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// DynamicChatModel resolves the active provider immediately before a model
// call.  This lets long-lived agents use a newly activated provider profile
// without rebuilding the HTTP server.
type DynamicChatModel struct {
	runtime *RuntimeRegistry
	tools   []*schema.ToolInfo
}

func (r *RuntimeRegistry) DynamicChatModel() model.ToolCallingChatModel {
	return &DynamicChatModel{runtime: r}
}

func (m *DynamicChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	chatModel, err := m.modelWithTools(ctx)
	if err != nil {
		return nil, err
	}
	return chatModel.Generate(ctx, input, opts...)
}

func (m *DynamicChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	chatModel, err := m.modelWithTools(ctx)
	if err != nil {
		return nil, err
	}
	return chatModel.Stream(ctx, input, opts...)
}

func (m *DynamicChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &DynamicChatModel{runtime: m.runtime, tools: tools}, nil
}

func (m *DynamicChatModel) modelWithTools(ctx context.Context) (model.ToolCallingChatModel, error) {
	chatModel, err := m.runtime.NewChatModel(ctx)
	if err != nil || len(m.tools) == 0 {
		return chatModel, err
	}
	return chatModel.WithTools(m.tools)
}

// DynamicEmbedder resolves the active embedding profile for each operation.
// Existing RAG and QA services can therefore stay alive across provider
// changes while their next embedding request uses the new profile.
type DynamicEmbedder struct {
	runtime *RuntimeRegistry
}

func (r *RuntimeRegistry) DynamicEmbedder() embedding.Embedder {
	return &DynamicEmbedder{runtime: r}
}

func (e *DynamicEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	embedder, err := e.runtime.NewEmbedder(ctx)
	if err != nil {
		return nil, err
	}
	return embedder.EmbedStrings(ctx, texts, opts...)
}
