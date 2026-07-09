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

type SearchHit struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Score    float64  `json:"score,omitempty"`
}

func NewService(store vectorstore.Store, embedder embedding.Embedder, topK int) *Service {
	if topK <= 0 {
		topK = 3
	}
	return &Service{store: store, embedder: embedder, topK: topK}
}

func (s *Service) Search(ctx context.Context, query string) (string, error) {
	hits, err := s.SearchHits(ctx, query)
	if err != nil {
		return "", err
	}
	if len(hits) == 0 {
		return "未找到相关 QA 知识库内容。", nil
	}
	return FormatHits(hits), nil
}

func (s *Service) SearchHits(ctx context.Context, query string) ([]SearchHit, error) {
	vecs, err := s.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("empty query embedding")
	}
	qv := make([]float32, len(vecs[0]))
	for i, v := range vecs[0] {
		qv[i] = float32(v)
	}

	records, err := s.store.Search(ctx, qv, s.topK)
	if err != nil {
		return nil, fmt.Errorf("retrieve documents: %w", err)
	}
	hits := make([]SearchHit, 0, len(records))
	for _, rec := range records {
		hits = append(hits, SearchHit{
			ID:       rec.ID,
			Question: rec.Question,
			Answer:   rec.Answer,
			Category: rec.Category,
			Tags:     rec.Tags,
			Score:    rec.Score,
		})
	}
	return hits, nil
}

func FormatHits(hits []SearchHit) string {
	var builder strings.Builder
	for i, rec := range hits {
		builder.WriteString(fmt.Sprintf("[%d]", i+1))
		if rec.Category != "" {
			builder.WriteString(fmt.Sprintf(" 分类: %s", rec.Category))
		}
		if len(rec.Tags) > 0 {
			builder.WriteString(fmt.Sprintf(" 标签: %s", strings.Join(rec.Tags, ", ")))
		}
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("Q: %s\n", rec.Question))
		builder.WriteString(fmt.Sprintf("A: %s\n", rec.Answer))
		if rec.Score != 0 {
			builder.WriteString(fmt.Sprintf("Score: %.4f\n", rec.Score))
		}
		if i < len(hits)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func (s *Service) BuildPrompt(query, context string) string {
	return strings.NewReplacer(
		"{{context}}", context,
		"{{query}}", query,
	).Replace(defaultPrompt)
}
