package rag

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

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
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	candidateLimit := s.topK * 20
	if candidateLimit < 100 {
		candidateLimit = 100
	}
	type textSearchResult struct {
		records []vectorstore.Record
		err     error
	}
	textResultCh := make(chan textSearchResult, 1)
	go func() {
		records, err := s.store.SearchText(ctx, query, candidateLimit)
		textResultCh <- textSearchResult{records: records, err: err}
	}()

	vecs, err := s.embedder.EmbedStrings(ctx, []string{query})
	var vectorRecords []vectorstore.Record
	vectorErr := err
	if vectorErr == nil {
		if len(vecs) == 0 || len(vecs[0]) == 0 {
			vectorErr = fmt.Errorf("empty query embedding")
		} else {
			qv := make([]float32, len(vecs[0]))
			for i, v := range vecs[0] {
				qv[i] = float32(v)
			}
			vectorRecords, vectorErr = s.store.Search(ctx, qv, candidateLimit)
		}
	}
	textResult := <-textResultCh
	textSearchUnsupported := errors.Is(textResult.err, vectorstore.ErrTextSearchUnsupported)
	if textSearchUnsupported {
		textResult.err = nil
	}
	if textSearchUnsupported && vectorErr != nil {
		return nil, fmt.Errorf("retrieve vector documents: %w", vectorErr)
	}
	if textResult.err != nil && vectorErr != nil {
		return nil, fmt.Errorf("retrieve text documents: %v; retrieve vector documents: %w", textResult.err, vectorErr)
	}
	if textResult.err != nil {
		return s.filterRecords(ctx, rerankRecords(query, fuseRecords(nil, vectorRecords, candidateLimit)))
	}
	if vectorErr != nil {
		return s.filterRecords(ctx, rerankRecords(query, fuseRecords(textResult.records, nil, candidateLimit)))
	}
	records := rerankRecords(query, fuseRecords(textResult.records, vectorRecords, candidateLimit))
	return s.filterRecords(ctx, records)
}

func (s *Service) filterRecords(ctx context.Context, records []vectorstore.Record) ([]SearchHit, error) {
	if s.filterKnowledgeBase == nil {
		if len(records) > s.topK {
			records = records[:s.topK]
		}
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

const (
	rrfRankConstant  = 60.0
	textRankWeight   = 1.25
	vectorRankWeight = 1.0
)

// fuseRecords combines independently ranked BM25 and vector results. RRF is
// intentionally used instead of raw scores because BM25 relevance and cosine
// distance are on incompatible scales. The returned score is normalized to
// [0,1], where documents ranked highly by both retrievers approach 1.
func fuseRecords(textRecords, vectorRecords []vectorstore.Record, limit int) []vectorstore.Record {
	if limit <= 0 {
		limit = 3
	}
	type fusedRecord struct {
		record   vectorstore.Record
		score    float64
		bestRank int
	}
	byID := make(map[string]*fusedRecord, len(textRecords)+len(vectorRecords))
	add := func(records []vectorstore.Record, weight float64) {
		for i, record := range records {
			if record.ID == "" {
				continue
			}
			rank := i + 1
			entry := byID[record.ID]
			if entry == nil {
				copy := record
				entry = &fusedRecord{record: copy, bestRank: rank}
				byID[record.ID] = entry
			} else {
				enrichRecord(&entry.record, record)
				if rank < entry.bestRank {
					entry.bestRank = rank
				}
			}
			entry.score += weight / (rrfRankConstant + float64(rank))
		}
	}
	add(textRecords, textRankWeight)
	add(vectorRecords, vectorRankWeight)
	maxScore := (textRankWeight + vectorRankWeight) / (rrfRankConstant + 1)
	entries := make([]*fusedRecord, 0, len(byID))
	for _, entry := range byID {
		entry.record.Score = entry.score / maxScore
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].score != entries[j].score {
			return entries[i].score > entries[j].score
		}
		if entries[i].bestRank != entries[j].bestRank {
			return entries[i].bestRank < entries[j].bestRank
		}
		return entries[i].record.ID < entries[j].record.ID
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	out := make([]vectorstore.Record, len(entries))
	for i, entry := range entries {
		out[i] = entry.record
	}
	return out
}

func enrichRecord(dst *vectorstore.Record, src vectorstore.Record) {
	if dst.Question == "" {
		dst.Question = src.Question
	}
	if dst.Answer == "" {
		dst.Answer = src.Answer
	}
	if dst.Category == "" {
		dst.Category = src.Category
	}
	if len(dst.Tags) == 0 {
		dst.Tags = src.Tags
	}
}

func rerankRecords(query string, records []vectorstore.Record) []vectorstore.Record {
	normalizedQuery := normalizeSearchText(query)
	if normalizedQuery == "" {
		return records
	}
	for i := range records {
		similarity := editSimilarity(normalizedQuery, normalizeSearchText(records[i].Question))
		if similarity < 0.6 {
			continue
		}
		records[i].Score = math.Min(1, records[i].Score+0.35*similarity)
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Score != records[j].Score {
			return records[i].Score > records[j].Score
		}
		return records[i].ID < records[j].ID
	})
	return records
}

func normalizeSearchText(value string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func editSimilarity(a, b string) float64 {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 || len(br) == 0 {
		return 0
	}
	previous := make([]int, len(br)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, left := range ar {
		current := make([]int, len(br)+1)
		current[0] = i + 1
		for j, right := range br {
			cost := 0
			if left != right {
				cost = 1
			}
			current[j+1] = min(current[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous = current
	}
	maxLength := max(len(ar), len(br))
	return 1 - float64(previous[len(br)])/float64(maxLength)
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
