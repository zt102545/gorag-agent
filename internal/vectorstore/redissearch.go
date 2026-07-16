package vectorstore

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/redis/go-redis/v9"

	"github.com/gorag-ai/gorag/internal/config"
)

type RedisSearchStore struct {
	client      *redis.Client
	indexName   string
	keyPrefix   string
	vectorField string
	dim         int
}

func NewRedisSearchStore(client *redis.Client, cfg config.RedisSearchStoreConfig) *RedisSearchStore {
	return &RedisSearchStore{
		client:      client,
		indexName:   cfg.IndexName,
		keyPrefix:   cfg.KeyPrefix,
		vectorField: cfg.VectorField,
	}
}

func (s *RedisSearchStore) key(id string) string {
	return s.keyPrefix + id
}

func (s *RedisSearchStore) Ensure(ctx context.Context, dim int) error {
	if dim <= 0 {
		return fmt.Errorf("embedding dimensions must be configured before creating redissearch index")
	}
	s.dim = dim
	_, err := s.client.FTInfo(ctx, s.indexName).Result()
	if err == nil {
		return nil
	}

	schemas := []*redis.FieldSchema{
		{FieldName: "question", FieldType: redis.SearchFieldTypeText, Weight: 4},
		{FieldName: "answer", FieldType: redis.SearchFieldTypeText, Weight: 1},
		{FieldName: "category", FieldType: redis.SearchFieldTypeTag},
		{FieldName: "tags", FieldType: redis.SearchFieldTypeText, Weight: 2},
		{
			FieldName: s.vectorField,
			FieldType: redis.SearchFieldTypeVector,
			VectorArgs: &redis.FTVectorArgs{
				FlatOptions: &redis.FTFlatOptions{
					Type:           "FLOAT32",
					Dim:            dim,
					DistanceMetric: "COSINE",
				},
			},
		},
	}
	_, err = s.client.FTCreate(ctx, s.indexName, &redis.FTCreateOptions{
		OnHash:          true,
		Prefix:          []any{s.keyPrefix},
		DefaultLanguage: "chinese",
	}, schemas...).Result()
	if err != nil && !strings.Contains(err.Error(), "Index already exists") {
		return fmt.Errorf("create redissearch index: %w", err)
	}
	return nil
}

func (s *RedisSearchStore) Upsert(ctx context.Context, rec Record) error {
	if len(rec.Vector) == 0 {
		return fmt.Errorf("vector is empty for id=%s", rec.ID)
	}
	blob := floatsToBytes(rec.Vector)
	tags := strings.Join(rec.Tags, ",")
	return s.client.HSet(ctx, s.key(rec.ID), map[string]any{
		"question":    rec.Question,
		"answer":      rec.Answer,
		"category":    rec.Category,
		"tags":        tags,
		s.vectorField: blob,
	}).Err()
}

func (s *RedisSearchStore) Delete(ctx context.Context, id string) error {
	return s.client.Del(ctx, s.key(id)).Err()
}

func (s *RedisSearchStore) SearchText(ctx context.Context, queryText string, topK int) ([]Record, error) {
	if topK <= 0 {
		topK = 3
	}
	strictQuery := buildTextQuery(queryText)
	if strictQuery == "" {
		return nil, nil
	}
	strictRecords, strictErr := s.searchBM25(ctx, strictQuery, topK)
	relaxedQuery := buildRelaxedTextQuery(queryText)
	if relaxedQuery == "" {
		return strictRecords, strictErr
	}
	relaxedRecords, relaxedErr := s.searchBM25(ctx, relaxedQuery, topK)
	if strictErr != nil && relaxedErr != nil {
		return nil, fmt.Errorf("strict search: %v; relaxed search: %w", strictErr, relaxedErr)
	}
	if strictErr != nil {
		return relaxedRecords, nil
	}
	if relaxedErr != nil {
		return strictRecords, nil
	}
	return mergeTextRecords(strictRecords, relaxedRecords, topK), nil
}

func (s *RedisSearchStore) searchBM25(ctx context.Context, query string, topK int) ([]Record, error) {
	res, err := s.client.FTSearchWithArgs(ctx, s.indexName, query, &redis.FTSearchOptions{
		Return: []redis.FTSearchReturn{
			{FieldName: "question"},
			{FieldName: "answer"},
			{FieldName: "category"},
			{FieldName: "tags"},
		},
		WithScores: true,
		Scorer:     "BM25",
		Language:   "chinese",
		Limit:      topK,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("redissearch bm25: %w", err)
	}
	return recordsFromRedisDocs(res.Docs, s.keyPrefix), nil
}

func (s *RedisSearchStore) Search(ctx context.Context, queryVector []float32, topK int) ([]Record, error) {
	if topK <= 0 {
		topK = 3
	}
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector is empty")
	}
	blob := floatsToBytes(queryVector)
	query := fmt.Sprintf("*=>[KNN %d @%s $vec AS score]", topK, s.vectorField)
	res, err := s.client.FTSearchWithArgs(ctx, s.indexName, query, &redis.FTSearchOptions{
		Return: []redis.FTSearchReturn{
			{FieldName: "question"},
			{FieldName: "answer"},
			{FieldName: "category"},
			{FieldName: "tags"},
			{FieldName: "score"},
		},
		DialectVersion: 2,
		Params:         map[string]any{"vec": blob},
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("redissearch knn: %w", err)
	}

	return recordsFromRedisDocs(res.Docs, s.keyPrefix), nil
}

func (s *RedisSearchStore) Close() error { return nil }

func recordsFromRedisDocs(docs []redis.Document, keyPrefix string) []Record {
	out := make([]Record, 0, len(docs))
	for _, doc := range docs {
		id := strings.TrimPrefix(doc.ID, keyPrefix)
		rec := Record{
			ID:       id,
			Question: doc.Fields["question"],
			Answer:   doc.Fields["answer"],
			Category: doc.Fields["category"],
		}
		if doc.Score != nil {
			rec.Score = *doc.Score
		}
		if raw := strings.TrimSpace(doc.Fields["score"]); raw != "" {
			if score, err := strconv.ParseFloat(raw, 64); err == nil {
				rec.Score = score
			}
		}
		if t := doc.Fields["tags"]; t != "" {
			rec.Tags = strings.Split(t, ",")
		}
		out = append(out, rec)
	}
	return out
}

func buildTextQuery(queryText string) string {
	queryText = strings.TrimSpace(queryText)
	if queryText == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range queryText {
		if strings.ContainsRune(`,.<>{}[]"':;!@#$%^&*()-+=~|/\\?`, r) {
			builder.WriteRune('\\')
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

// buildRelaxedTextQuery creates overlapping three-rune alternatives for
// Chinese questions. RedisSearch's Chinese parser uses AND semantics for a
// plain sentence, so a small morphology change such as "鐢ㄩ€? -> "鐢? can
// otherwise eliminate an almost exact question. Restricting the alternatives
// to @question and reranking later keeps this recall expansion bounded.
func buildRelaxedTextQuery(queryText string) string {
	runes := make([]rune, 0, len([]rune(queryText)))
	hasHan := false
	for _, r := range strings.TrimSpace(queryText) {
		if unicode.In(r, unicode.Han) {
			hasHan = true
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		runes = append(runes, r)
	}
	if !hasHan || len(runes) < 4 {
		return ""
	}
	const maxGrams = 24
	grams := make([]string, 0, min(len(runes)-2, maxGrams))
	seen := make(map[string]struct{}, cap(grams))
	for i := 0; i+3 <= len(runes) && len(grams) < maxGrams; i++ {
		gram := string(runes[i : i+3])
		if _, ok := seen[gram]; ok {
			continue
		}
		seen[gram] = struct{}{}
		grams = append(grams, buildTextQuery(gram))
	}
	if len(grams) == 0 {
		return ""
	}
	return "@question:(" + strings.Join(grams, "|") + ")"
}

func mergeTextRecords(strict, relaxed []Record, limit int) []Record {
	if limit <= 0 {
		limit = 3
	}
	out := make([]Record, 0, min(len(strict)+len(relaxed), limit))
	seen := make(map[string]struct{}, cap(out))
	appendUnique := func(records []Record) {
		for _, record := range records {
			if record.ID == "" {
				continue
			}
			if _, ok := seen[record.ID]; ok {
				continue
			}
			seen[record.ID] = struct{}{}
			out = append(out, record)
			if len(out) >= limit {
				return
			}
		}
	}
	appendUnique(strict)
	if len(out) < limit {
		appendUnique(relaxed)
	}
	return out
}

func floatsToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}
