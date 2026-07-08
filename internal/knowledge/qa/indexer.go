package qa

import (
	"context"
	"fmt"
	"strings"

	redisIndexer "github.com/cloudwego/eino-ext/components/indexer/redis"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"

	"github.com/yourorg/agent_customer_service/internal/config"
)

type VectorIndexer struct {
	indexer   indexer.Indexer
	cfg       config.RAGConfig
	keyPrefix string
}

func NewVectorIndexer(client *redis.Client, embedder embedding.Embedder, cfg config.RAGConfig) (*VectorIndexer, error) {
	idx, err := redisIndexer.NewIndexer(context.Background(), &redisIndexer.IndexerConfig{
		Client:           client,
		KeyPrefix:        cfg.KeyPrefix,
		BatchSize:        10,
		Embedding:        embedder,
		DocumentToHashes: qaDocumentToHashes(cfg.VectorField),
	})
	if err != nil {
		return nil, err
	}
	return &VectorIndexer{
		indexer:   idx,
		cfg:       cfg,
		keyPrefix: cfg.KeyPrefix,
	}, nil
}

func (v *VectorIndexer) EnsureIndex(ctx context.Context, client *redis.Client, dimensions int) error {
	_, err := client.FTInfo(ctx, v.cfg.IndexName).Result()
	if err == nil {
		return nil
	}

	schemas := []*redis.FieldSchema{
		{FieldName: "question", FieldType: redis.SearchFieldTypeText, Weight: 1},
		{FieldName: "answer", FieldType: redis.SearchFieldTypeText},
		{FieldName: "category", FieldType: redis.SearchFieldTypeTag},
		{FieldName: "tags", FieldType: redis.SearchFieldTypeText},
		{
			FieldName: v.cfg.VectorField,
			FieldType: redis.SearchFieldTypeVector,
			VectorArgs: &redis.FTVectorArgs{
				FlatOptions: &redis.FTFlatOptions{
					Type:           "FLOAT32",
					Dim:            dimensions,
					DistanceMetric: "COSINE",
				},
			},
		},
	}
	_, err = client.FTCreate(ctx, v.cfg.IndexName, &redis.FTCreateOptions{
		OnHash: true,
		Prefix: []any{v.keyPrefix},
	}, schemas...).Result()
	if err != nil && !strings.Contains(err.Error(), "Index already exists") {
		return fmt.Errorf("create redis index: %w", err)
	}
	return nil
}

func (v *VectorIndexer) Index(ctx context.Context, item *Item) error {
	_, err := v.indexer.Store(ctx, []*schema.Document{itemToDocument(item)})
	return err
}

func (v *VectorIndexer) Delete(ctx context.Context, client *redis.Client, id string) error {
	return client.Del(ctx, v.keyPrefix+id).Err()
}

func itemToDocument(item *Item) *schema.Document {
	return &schema.Document{
		ID:      item.ID,
		Content: item.Question,
		MetaData: map[string]any{
			"answer":   item.Answer,
			"category": item.Category,
			"tags":     strings.Join(item.Tags, ","),
		},
	}
}

func qaDocumentToHashes(vectorField string) func(ctx context.Context, doc *schema.Document) (*redisIndexer.Hashes, error) {
	return func(ctx context.Context, doc *schema.Document) (*redisIndexer.Hashes, error) {
		if doc.ID == "" {
			return nil, fmt.Errorf("doc id not set")
		}
		answer, _ := doc.MetaData["answer"].(string)
		category, _ := doc.MetaData["category"].(string)
		tags, _ := doc.MetaData["tags"].(string)

		return &redisIndexer.Hashes{
			Key: doc.ID,
			Field2Value: map[string]redisIndexer.FieldValue{
				"question": {Value: doc.Content, EmbedKey: vectorField},
				"answer":   {Value: answer},
				"category": {Value: category},
				"tags":     {Value: tags},
			},
		}, nil
	}
}
