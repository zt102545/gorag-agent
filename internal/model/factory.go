package model

import (
	"context"
	"fmt"

	openaiEmbedding "github.com/cloudwego/eino-ext/components/embedding/openai"
	openaiModel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"

	"github.com/yourorg/agent_customer_service/internal/config"
)

func NewChatModel(ctx context.Context, cfg config.ChatModelConfig) (model.ToolCallingChatModel, error) {
	switch cfg.Provider {
	case "openai", "":
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
	switch cfg.Provider {
	case "openai", "":
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
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.Provider)
	}
}
