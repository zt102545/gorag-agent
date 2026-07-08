package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/yourorg/agent_customer_service/internal/vectorstore"
)

type Service struct {
	store    vectorstore.Store
	embedder embedding.Embedder
	topK     int
}

func NewService(store vectorstore.Store, embedder embedding.Embedder, topK int) *Service {
	if topK <= 0 {
		topK = 3
	}
	return &Service{store: store, embedder: embedder, topK: topK}
}

func (s *Service) Search(ctx context.Context, query string) (string, error) {
	vecs, err := s.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return "", fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return "", fmt.Errorf("empty query embedding")
	}
	qv := make([]float32, len(vecs[0]))
	for i, v := range vecs[0] {
		qv[i] = float32(v)
	}

	records, err := s.store.Search(ctx, qv, s.topK)
	if err != nil {
		return "", fmt.Errorf("retrieve documents: %w", err)
	}
	if len(records) == 0 {
		return "未找到相关 QA 知识库内容。", nil
	}

	var builder strings.Builder
	for i, rec := range records {
		builder.WriteString(fmt.Sprintf("[%d]", i+1))
		if rec.Category != "" {
			builder.WriteString(fmt.Sprintf(" 分类: %s", rec.Category))
		}
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Q: %s\n", rec.Question))
		builder.WriteString(fmt.Sprintf("A: %s\n", rec.Answer))
		if i < len(records)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String(), nil
}

func (s *Service) BuildPrompt(query, context string) string {
	return strings.NewReplacer(
		"{{context}}", context,
		"{{query}}", query,
	).Replace(defaultPrompt)
}
