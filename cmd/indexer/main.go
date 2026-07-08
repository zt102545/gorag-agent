package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
	"github.com/yourorg/agent_customer_service/internal/model"
	redispkg "github.com/yourorg/agent_customer_service/pkg/redis"
)

func main() {
	file := flag.String("file", "./data/knowledge/qa.csv", "QA CSV file path")
	flag.Parse()

	_ = godotenv.Load()

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}

	cfg, err := config.Load(env)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	redisClient := redispkg.NewIndexerClient(cfg.Redis)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer redisClient.Close()

	embedder, err := model.NewEmbedder(ctx, cfg.AI.Embedding)
	if err != nil {
		log.Fatalf("create embedder: %v", err)
	}

	metaStore := qa.NewRedisMetaStore(redisClient)
	vectorIndexer, err := qa.NewVectorIndexer(redisClient, embedder, cfg.RAG)
	if err != nil {
		log.Fatalf("create qa indexer: %v", err)
	}
	svc := qa.NewService(metaStore, vectorIndexer, redisClient, cfg.AI.Embedding.Dimensions)
	if err := svc.EnsureIndex(ctx); err != nil {
		log.Fatalf("ensure index: %v", err)
	}

	f, err := os.Open(*file)
	if err != nil {
		log.Fatalf("open file: %v", err)
	}
	defer f.Close()

	result, err := svc.ImportCSV(ctx, f)
	if err != nil {
		log.Fatalf("import csv: %v", err)
	}

	log.Printf("imported %d qa items from %s (failed: %d)", result.Imported, *file, result.Failed)
	for _, e := range result.Errors {
		log.Printf("  error: %s", e)
	}
}
