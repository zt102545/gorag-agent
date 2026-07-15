package vectorstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"

	"github.com/yourorg/agent_customer_service/internal/config"
)

type MilvusStore struct {
	client     client.Client
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

	mcfg := client.Config{
		Address:  address,
		Username: cfg.Username,
		Password: cfg.Password,
		DBName:   cfg.Database,
		APIKey:   cfg.Token,
	}
	if strings.HasPrefix(strings.ToLower(address), "https://") {
		mcfg.EnableTLSAuth = true
	}

	cli, err := client.NewClient(ctx, mcfg)
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
	exists, err := s.client.HasCollection(ctx, s.collection)
	if err != nil {
		return fmt.Errorf("has collection: %w", err)
	}
	if !exists {
		schema := &entity.Schema{
			CollectionName: s.collection,
			Description:    "customer service QA vectors",
			Fields: []*entity.Field{
				{
					Name:       "id",
					DataType:   entity.FieldTypeVarChar,
					PrimaryKey: true,
					AutoID:     false,
					TypeParams: map[string]string{"max_length": "64"},
				},
				{
					Name:       "question",
					DataType:   entity.FieldTypeVarChar,
					TypeParams: map[string]string{"max_length": "4096"},
				},
				{
					Name:       "answer",
					DataType:   entity.FieldTypeVarChar,
					TypeParams: map[string]string{"max_length": "8192"},
				},
				{
					Name:       "category",
					DataType:   entity.FieldTypeVarChar,
					TypeParams: map[string]string{"max_length": "256"},
				},
				{
					Name:       "tags",
					DataType:   entity.FieldTypeVarChar,
					TypeParams: map[string]string{"max_length": "1024"},
				},
				{
					Name:     "vector",
					DataType: entity.FieldTypeFloatVector,
					TypeParams: map[string]string{
						"dim": fmt.Sprintf("%d", dim),
					},
				},
			},
		}
		if err := s.client.CreateCollection(ctx, schema, 1); err != nil {
			return fmt.Errorf("create collection: %w", err)
		}
		idx, err := entity.NewIndexHNSW(s.metric, 16, 200)
		if err != nil {
			return fmt.Errorf("new hnsw index: %w", err)
		}
		if err := s.client.CreateIndex(ctx, s.collection, "vector", idx, false); err != nil {
			return fmt.Errorf("create index: %w", err)
		}
	}
	if err := s.client.LoadCollection(ctx, s.collection, false); err != nil {
		return fmt.Errorf("load collection: %w", err)
	}
	return nil
}

func (s *MilvusStore) Upsert(ctx context.Context, rec Record) error {
	if len(rec.Vector) == 0 {
		return fmt.Errorf("vector is empty for id=%s", rec.ID)
	}
	tags := strings.Join(rec.Tags, ",")
	idCol := entity.NewColumnVarChar("id", []string{rec.ID})
	qCol := entity.NewColumnVarChar("question", []string{rec.Question})
	aCol := entity.NewColumnVarChar("answer", []string{rec.Answer})
	cCol := entity.NewColumnVarChar("category", []string{rec.Category})
	tCol := entity.NewColumnVarChar("tags", []string{tags})
	vCol := entity.NewColumnFloatVector("vector", s.dim, [][]float32{rec.Vector})
	_, err := s.client.Upsert(ctx, s.collection, "", idCol, qCol, aCol, cCol, tCol, vCol)
	if err != nil {
		return fmt.Errorf("milvus upsert: %w", err)
	}
	return nil
}

func (s *MilvusStore) Delete(ctx context.Context, id string) error {
	expr := fmt.Sprintf(`id == "%s"`, strings.ReplaceAll(id, `"`, ``))
	return s.client.Delete(ctx, s.collection, "", expr)
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
	sp, err := entity.NewIndexHNSWSearchParam(16)
	if err != nil {
		return nil, err
	}
	results, err := s.client.Search(
		ctx,
		s.collection,
		nil,
		"",
		[]string{"id", "question", "answer", "category", "tags"},
		[]entity.Vector{entity.FloatVector(queryVector)},
		"vector",
		s.metric,
		topK,
		sp,
	)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}
	out := make([]Record, 0)
	for _, r := range results {
		if r.Err != nil {
			return nil, r.Err
		}
		n := r.ResultCount
		ids := columnStrings(r.Fields.GetColumn("id"), n)
		// Prefer explicit id field; fall back to result IDs column.
		if len(ids) == 0 || (n > 0 && ids[0] == "") {
			ids = columnStrings(r.IDs, n)
		}
		questions := columnStrings(r.Fields.GetColumn("question"), n)
		answers := columnStrings(r.Fields.GetColumn("answer"), n)
		categories := columnStrings(r.Fields.GetColumn("category"), n)
		tagsCol := columnStrings(r.Fields.GetColumn("tags"), n)
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
		s.client.Close()
	}
	return nil
}

func columnStrings(col entity.Column, n int) []string {
	if col == nil {
		return make([]string, n)
	}
	switch c := col.(type) {
	case *entity.ColumnVarChar:
		data := c.Data()
		out := make([]string, n)
		for i := 0; i < n && i < len(data); i++ {
			out[i] = data[i]
		}
		return out
	default:
		out := make([]string, n)
		for i := 0; i < n; i++ {
			v, err := col.GetAsString(i)
			if err == nil {
				out[i] = v
			}
		}
		return out
	}
}

func safeAt(ss []string, i int) string {
	if i < 0 || i >= len(ss) {
		return ""
	}
	return ss[i]
}
