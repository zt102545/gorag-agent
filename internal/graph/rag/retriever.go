package rag

import (
	"context"
	"fmt"

	redisRetriever "github.com/cloudwego/eino-ext/components/retriever/redis"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"

	"github.com/yourorg/agent_customer_service/internal/config"
)

type Components struct {
	Retriever retriever.Retriever
	TopK      int
}

func NewRetriever(ctx context.Context, client *redis.Client, embedder embedding.Embedder, cfg config.RAGConfig) (*Components, error) {
	returnFields := []string{"question", "answer", "category", "tags"}
	r, err := redisRetriever.NewRetriever(ctx, &redisRetriever.RetrieverConfig{
		Client:            client,
		Index:             cfg.IndexName,
		VectorField:       cfg.VectorField,
		TopK:              cfg.TopK,
		Embedding:         embedder,
		ReturnFields:      returnFields,
		DocumentConverter: qaDocumentConverter(),
	})
	if err != nil {
		return nil, fmt.Errorf("create retriever: %w", err)
	}
	return &Components{
		Retriever: r,
		TopK:      cfg.TopK,
	}, nil
}

func qaDocumentConverter() func(ctx context.Context, doc redis.Document) (*schema.Document, error) {
	return func(ctx context.Context, doc redis.Document) (*schema.Document, error) {
		result := &schema.Document{
			ID:       doc.ID,
			MetaData: make(map[string]any),
		}
		for field, val := range doc.Fields {
			switch field {
			case "question":
				result.Content = val
			case "answer":
				result.MetaData["answer"] = val
			case "category":
				result.MetaData["category"] = val
			case "tags":
				result.MetaData["tags"] = val
			}
		}
		return result, nil
	}
}
