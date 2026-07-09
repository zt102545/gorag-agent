package model

import (
	"context"
	"errors"
	"fmt"
	"strings"

	arkEmbedding "github.com/cloudwego/eino-ext/components/embedding/ark"
	openaiEmbedding "github.com/cloudwego/eino-ext/components/embedding/openai"
	openaiModel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/yourorg/agent_customer_service/internal/config"
)

var (
	ErrChatModelNotConfigured = errors.New("chat model is not configured in GM")
	ErrEmbeddingNotConfigured = errors.New("embedding model is not configured in GM")
)

func NewChatModel(ctx context.Context, cfg config.ChatModelConfig) (model.ToolCallingChatModel, error) {
	if cfg.Provider == "" || cfg.ModelName == "" {
		return &unconfiguredChatModel{err: ErrChatModelNotConfigured}, nil
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "openai", "openai_compatible", "openai-compatible", "compatible", "longcat":
		temp := cfg.Temperature
		var maxTokens *int
		if cfg.MaxTokens > 0 {
			maxTokens = &cfg.MaxTokens
		}
		return openaiModel.NewChatModel(ctx, &openaiModel.ChatModelConfig{
			BaseURL:     cfg.BaseURL,
			APIKey:      cfg.APIKey,
			Model:       cfg.ModelName,
			Temperature: &temp,
			MaxTokens:   maxTokens,
		})
	default:
		return nil, fmt.Errorf("unsupported chat model provider: %s", cfg.Provider)
	}
}

func NewEmbedder(ctx context.Context, cfg config.EmbeddingConfig) (embedding.Embedder, error) {
	if cfg.Provider == "" || cfg.ModelName == "" {
		return &unconfiguredEmbedder{err: ErrEmbeddingNotConfigured}, nil
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "openai":
		var dims *int
		if cfg.Dimensions > 0 {
			dims = &cfg.Dimensions
		}
		return openaiEmbedding.NewEmbedder(ctx, &openaiEmbedding.EmbeddingConfig{
			BaseURL:    cfg.BaseURL,
			APIKey:     cfg.APIKey,
			Model:      cfg.ModelName,
			Dimensions: dims,
		})
	case "ark", "volc", "volcengine", "火山", "火山方舟":
		var dims *int
		if cfg.Dimensions > 0 {
			dims = &cfg.Dimensions
		}
		return arkEmbedding.NewEmbedder(ctx, &arkEmbedding.EmbeddingConfig{
			BaseURL:    cfg.BaseURL,
			APIKey:     cfg.APIKey,
			Model:      cfg.ModelName,
			Dimensions: dims,
		})
	case "ark_multimodal", "ark-multimodal", "volc_multimodal", "volcengine_multimodal", "火山多模态", "火山方舟多模态":
		var dims *int
		if cfg.Dimensions > 0 {
			dims = &cfg.Dimensions
		}
		apiType := arkEmbedding.APITypeMultiModal
		return arkEmbedding.NewEmbedder(ctx, &arkEmbedding.EmbeddingConfig{
			BaseURL:    cfg.BaseURL,
			APIKey:     cfg.APIKey,
			Model:      cfg.ModelName,
			APIType:    &apiType,
			Dimensions: dims,
		})
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Provider)
	}
}

type unconfiguredChatModel struct {
	err error
}

func (m *unconfiguredChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return nil, m.err
}

func (m *unconfiguredChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, m.err
}

func (m *unconfiguredChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type unconfiguredEmbedder struct {
	err error
}

func (e *unconfiguredEmbedder) EmbedStrings(_ context.Context, _ []string, _ ...embedding.Option) ([][]float64, error) {
	return nil, e.err
}
