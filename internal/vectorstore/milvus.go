package vectorstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/milvus-io/milvus/client/v3/column"
	"github.com/milvus-io/milvus/client/v3/entity"
	"github.com/milvus-io/milvus/client/v3/index"
	"github.com/milvus-io/milvus/client/v3/milvusclient"

	"github.com/gorag-ai/gorag/internal/config"
)

type MilvusStore struct {
	client     *milvusclient.Client
	collection string
	metric     entity.MetricType
	dim        int
}

func NewMilvusStore(ctx context.Context, cfg config.MilvusStoreConfig) (*MilvusStore, error) {
	address := strings.TrimSpace(cfg.URI)
	if address == "" {
		address = strings.TrimSpace(cfg.Address)
	}
	if address == "" {
		return nil, fmt.Errorf("milvus address or uri is required")
	}

	cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address:  address,
		Username: cfg.Username,
		Password: cfg.Password,
		DBName:   cfg.Database,
		APIKey:   cfg.Token,
	})
	if err != nil {
		return nil, fmt.Errorf("connect milvus: %w", err)
	}

	metric := entity.COSINE
	switch strings.ToUpper(cfg.MetricType) {
	case "L2":
		metric = entity.L2
	case "IP":
		metric = entity.IP
	case "COSINE", "":
		metric = entity.COSINE
	default:
		return nil, fmt.Errorf("unsupported milvus metric_type: %s", cfg.MetricType)
	}

	return &MilvusStore{
		client:     cli,
		collection: cfg.Collection,
		metric:     metric,
	}, nil
}

func (s *MilvusStore) Ensure(ctx context.Context, dim int) error {
	if dim <= 0 {
		return fmt.Errorf("embedding dimensions must be configured before creating milvus collection")
	}
	s.dim = dim
	has, err := s.client.HasCollection(ctx, milvusclient.NewHasCollectionOption(s.collection))
	if err != nil {
		return fmt.Errorf("has collection: %w", err)
	}
	if !has {
		schema := entity.NewSchema().
			WithDescription("customer service QA vectors").
			WithField(entity.NewField().
				WithName("id").
				WithDataType(entity.FieldTypeVarChar).
				WithIsPrimaryKey(true).
				WithMaxLength(64)).
			WithField(entity.NewField().
				WithName("question").
				WithDataType(entity.FieldTypeVarChar).
				WithMaxLength(4096)).
			WithField(entity.NewField().
				WithName("answer").
				WithDataType(entity.FieldTypeVarChar).
				WithMaxLength(8192)).
			WithField(entity.NewField().
				WithName("category").
				WithDataType(entity.FieldTypeVarChar).
				WithMaxLength(256)).
			WithField(entity.NewField().
				WithName("tags").
				WithDataType(entity.FieldTypeVarChar).
				WithMaxLength(1024)).
			WithField(entity.NewField().
				WithName("vector").
				WithDataType(entity.FieldTypeFloatVector).
				WithDim(int64(dim)))

		if err := s.client.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(s.collection, schema).
			WithIndexOptions(milvusclient.NewCreateIndexOption(s.collection, "vector", index.NewHNSWIndex(s.metric, 16, 200)))); err != nil {
			return fmt.Errorf("create collection: %w", err)
		}
	}
	loadTask, err := s.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(s.collection))
	if err != nil {
		return fmt.Errorf("load collection: %w", err)
	}
	if err := loadTask.Await(ctx); err != nil {
		return fmt.Errorf("load collection: %w", err)
	}
	return nil
}

func (s *MilvusStore) Upsert(ctx context.Context, rec Record) error {
	if len(rec.Vector) == 0 {
		return fmt.Errorf("vector is empty for id=%s", rec.ID)
	}
	tags := strings.Join(rec.Tags, ",")
	idCol := column.NewColumnVarChar("id", []string{rec.ID})
	qCol := column.NewColumnVarChar("question", []string{rec.Question})
	aCol := column.NewColumnVarChar("answer", []string{rec.Answer})
	cCol := column.NewColumnVarChar("category", []string{rec.Category})
	tCol := column.NewColumnVarChar("tags", []string{tags})
	vCol := column.NewColumnFloatVector("vector", s.dim, [][]float32{rec.Vector})
	_, err := s.client.Upsert(ctx, milvusclient.NewColumnBasedInsertOption(s.collection, idCol, qCol, aCol, cCol, tCol, vCol))
	if err != nil {
		return fmt.Errorf("milvus upsert: %w", err)
	}
	return nil
}

func (s *MilvusStore) Delete(ctx context.Context, id string) error {
	_, err := s.client.Delete(ctx, milvusclient.NewDeleteOption(s.collection).WithStringIDs("id", []string{id}))
	if err != nil {
		return fmt.Errorf("milvus delete: %w", err)
	}
	return nil
}

func (s *MilvusStore) SearchText(context.Context, string, int) ([]Record, error) {
	return nil, ErrTextSearchUnsupported
}

func (s *MilvusStore) Search(ctx context.Context, queryVector []float32, topK int) ([]Record, error) {
	if topK <= 0 {
		topK = 3
	}
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("query vector is empty")
	}
	resultSets, err := s.client.Search(ctx, milvusclient.NewSearchOption(
		s.collection,
		topK,
		[]entity.Vector{entity.FloatVector(queryVector)},
	).WithANNSField("vector").
		WithOutputFields("id", "question", "answer", "category", "tags").
		WithAnnParam(index.NewHNSWAnnParam(16)))
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}
	out := make([]Record, 0)
	for _, r := range resultSets {
		if r.Err != nil {
			return nil, r.Err
		}
		n := r.ResultCount
		ids := columnStrings(r.GetColumn("id"), n)
		// Prefer explicit id field; fall back to result IDs column.
		if len(ids) == 0 || (n > 0 && ids[0] == "") {
			ids = columnStrings(r.IDs, n)
		}
		questions := columnStrings(r.GetColumn("question"), n)
		answers := columnStrings(r.GetColumn("answer"), n)
		categories := columnStrings(r.GetColumn("category"), n)
		tagsCol := columnStrings(r.GetColumn("tags"), n)
		for i := 0; i < n; i++ {
			rec := Record{
				ID:       safeAt(ids, i),
				Question: safeAt(questions, i),
				Answer:   safeAt(answers, i),
				Category: safeAt(categories, i),
			}
			if i < len(r.Scores) {
				rec.Score = float64(r.Scores[i])
			}
			if t := safeAt(tagsCol, i); t != "" {
				rec.Tags = strings.Split(t, ",")
			}
			out = append(out, rec)
		}
	}
	return out, nil
}

func (s *MilvusStore) Close() error {
	if s.client != nil {
		return s.client.Close(context.Background())
	}
	return nil
}

func columnStrings(col column.Column, n int) []string {
	out := make([]string, n)
	if col == nil {
		return out
	}
	for i := 0; i < n; i++ {
		if v, err := col.GetAsString(i); err == nil {
			out[i] = v
		}
	}
	return out
}

func safeAt(ss []string, i int) string {
	if i < 0 || i >= len(ss) {
		return ""
	}
	return ss[i]
}
