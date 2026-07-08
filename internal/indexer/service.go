package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	fileLoader "github.com/cloudwego/eino-ext/components/document/loader/file"
	markdownSplitter "github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	redisIndexer "github.com/cloudwego/eino-ext/components/indexer/redis"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"

	"github.com/yourorg/agent_customer_service/internal/config"
)

type Service struct {
	indexer indexer.Indexer
	cfg     config.RAGConfig
}

func NewService(client *redis.Client, embedder embedding.Embedder, cfg config.RAGConfig) (*Service, error) {
	idx, err := redisIndexer.NewIndexer(context.Background(), &redisIndexer.IndexerConfig{
		Client:    client,
		KeyPrefix: cfg.KeyPrefix,
		BatchSize: 10,
		Embedding: embedder,
	})
	if err != nil {
		return nil, err
	}
	return &Service{indexer: idx, cfg: cfg}, nil
}

func (s *Service) EnsureIndex(ctx context.Context, client *redis.Client, dimensions int) error {
	_, err := client.FTInfo(ctx, s.cfg.IndexName).Result()
	if err == nil {
		return nil
	}

	schemas := []*redis.FieldSchema{
		{
			FieldName: "content",
			FieldType: redis.SearchFieldTypeText,
			Weight:    1,
		},
		{
			FieldName: s.cfg.VectorField,
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
	_, err = client.FTCreate(ctx, s.cfg.IndexName, &redis.FTCreateOptions{
		OnHash: true,
		Prefix: []any{s.cfg.KeyPrefix},
	}, schemas...).Result()
	if err != nil && !strings.Contains(err.Error(), "Index already exists") {
		return fmt.Errorf("create redis index: %w", err)
	}
	return nil
}

func (s *Service) IndexPath(ctx context.Context, path string) ([]string, error) {
	loader, err := fileLoader.NewFileLoader(ctx, &fileLoader.FileLoaderConfig{
		UseNameAsID: true,
	})
	if err != nil {
		return nil, err
	}

	splitter, err := markdownSplitter.NewHeaderSplitter(ctx, &markdownSplitter.HeaderConfig{
		Headers: map[string]string{
			"#":   "h1",
			"##":  "h2",
			"###": "h3",
		},
		TrimHeaders: false,
	})
	if err != nil {
		return nil, err
	}

	var allDocs []*schema.Document
	err = filepath.Walk(path, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext != ".md" && ext != ".txt" {
			return nil
		}
		docs, loadErr := loader.Load(ctx, document.Source{URI: filePath})
		if loadErr != nil {
			return loadErr
		}
		chunks, splitErr := splitter.Transform(ctx, docs)
		if splitErr != nil {
			return splitErr
		}
		allDocs = append(allDocs, chunks...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(allDocs) == 0 {
		return nil, fmt.Errorf("no documents found in %s", path)
	}
	return s.indexer.Store(ctx, allDocs)
}
