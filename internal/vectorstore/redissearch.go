package vectorstore

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/yourorg/agent_customer_service/internal/config"
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
		{FieldName: "question", FieldType: redis.SearchFieldTypeText, Weight: 1},
		{FieldName: "answer", FieldType: redis.SearchFieldTypeText},
		{FieldName: "category", FieldType: redis.SearchFieldTypeTag},
		{FieldName: "tags", FieldType: redis.SearchFieldTypeText},
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
		OnHash: true,
		Prefix: []any{s.keyPrefix},
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
		topK = 1
	}
	query := buildTextQuery(queryText)
	if query == "" {
		return nil, nil
	}
	res, err := s.client.FTSearchWithArgs(ctx, s.indexName, query, &redis.FTSearchOptions{
		Return: []redis.FTSearchReturn{
			{FieldName: "question"},
			{FieldName: "answer"},
			{FieldName: "category"},
			{FieldName: "tags"},
		},
		WithScores: true,
		Scorer:     "BM25",
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
	terms := strings.Fields(strings.TrimSpace(queryText))
	if len(terms) == 0 {
		return ""
	}
	for i, term := range terms {
		terms[i] = escapeRedisSearchTerm(term)
	}
	return strings.Join(terms, " ")
}

func escapeRedisSearchTerm(term string) string {
	var builder strings.Builder
	for _, r := range term {
		if strings.ContainsRune(`,.<>{}[]"':;!@#$%^&*()-+=~|/\?`, r) {
			builder.WriteRune('\\')
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func floatsToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}
