package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/gorag-ai/gorag/internal/config"
	"github.com/gorag-ai/gorag/internal/db"
	"github.com/gorag-ai/gorag/internal/knowledge/qa"
	"github.com/gorag-ai/gorag/internal/model"
	"github.com/gorag-ai/gorag/internal/vectorstore"
	redispkg "github.com/gorag-ai/gorag/pkg/redis"
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

	gdb, err := db.Open(cfg.Database)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(gdb); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	redisClient := redispkg.NewIndexerClient(cfg.Redis)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer redisClient.Close()

	embedder, err := model.NewEmbedder(ctx, cfg.Embedding)
	if err != nil {
		log.Fatalf("create embedder: %v", err)
	}

	vs, err := vectorstore.New(ctx, cfg.VectorStore, redisClient)
	if err != nil {
		log.Fatalf("create vector store: %v", err)
	}
	defer vs.Close()

	metaStore := qa.NewSQLMetaStore(gdb)
	svc := qa.NewService(metaStore, vs, embedder, redisClient, cfg.Embedding.Dimensions)
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
