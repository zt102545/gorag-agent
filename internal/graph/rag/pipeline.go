package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/yourorg/agent_customer_service/internal/vectorstore"
)

type Service struct {
	store               vectorstore.Store
	embedder            embedding.Embedder
	topK                int
	filterKnowledgeBase func(context.Context, string, []string) (map[string]struct{}, error)
	loadCreatedAt       func(context.Context, string, []string) (map[string]time.Time, error)
}

func (s *Service) SetKnowledgeBaseFilter(filter func(context.Context, string, []string) (map[string]struct{}, error)) {
	s.filterKnowledgeBase = filter
}

func (s *Service) SetKnowledgeBaseCreatedAtLoader(loader func(context.Context, string, []string) (map[string]time.Time, error)) {
	s.loadCreatedAt = loader
}

type SearchHit struct {
	ID        string     `json:"id"`
	Question  string     `json:"question"`
	Answer    string     `json:"answer"`
	Category  string     `json:"category,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
	Score     float64    `json:"score,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
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
	candidateLimit := s.topK * 20
	if candidateLimit < 100 {
		candidateLimit = 100
	}
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

	records, err := s.store.Search(ctx, qv, candidateLimit)
	if err != nil {
		return nil, fmt.Errorf("retrieve vector documents: %w", err)
	}
	return s.filterRecords(ctx, records)
}

func (s *Service) filterRecords(ctx context.Context, records []vectorstore.Record) ([]SearchHit, error) {
	if s.filterKnowledgeBase == nil {
		return searchHitsFromRecords(records), nil
	}
	ids := make([]string, 0, len(records))
	for _, rec := range records {
		ids = append(ids, rec.ID)
	}
	allowed, err := s.filterKnowledgeBase(ctx, "", ids)
	if err != nil {
		return nil, err
	}
	filtered := make([]vectorstore.Record, 0, s.topK)
	for _, rec := range records {
		if _, ok := allowed[rec.ID]; !ok {
			continue
		}
		filtered = append(filtered, rec)
		if len(filtered) >= s.topK {
			break
		}
	}
	hits := searchHitsFromRecords(filtered)
	if s.loadCreatedAt == nil || len(hits) == 0 {
		return hits, nil
	}
	createdAt, err := s.loadCreatedAt(ctx, "", idsFromHits(hits))
	if err != nil {
		return nil, fmt.Errorf("load QA creation times: %w", err)
	}
	for i := range hits {
		if value, ok := createdAt[hits[i].ID]; ok {
			value := value
			hits[i].CreatedAt = &value
		}
	}
	return hits, nil
}

func idsFromHits(hits []SearchHit) []string {
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.ID)
	}
	return ids
}

func searchHitsFromRecords(records []vectorstore.Record) []SearchHit {
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
	return hits
}

func FormatHits(hits []SearchHit) string {
	return FormatHitsAt(hits, time.Now())
}

func FormatHitsAt(hits []SearchHit, now time.Time) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("当前时间: %s\n\n", now.Format(time.RFC3339)))
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
		if rec.CreatedAt != nil && !rec.CreatedAt.IsZero() {
			builder.WriteString(fmt.Sprintf("QA入库时间: %s\n", rec.CreatedAt.Format(time.RFC3339)))
		}
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
