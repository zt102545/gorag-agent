package bootstrap

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	knowledgeapp "github.com/yourorg/agent_customer_service/internal/application/knowledge"
	sessionapp "github.com/yourorg/agent_customer_service/internal/application/session"
	settingsapp "github.com/yourorg/agent_customer_service/internal/application/settings"
	"github.com/yourorg/agent_customer_service/internal/checkpoint"
	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
	"github.com/yourorg/agent_customer_service/internal/session"
	"github.com/yourorg/agent_customer_service/internal/vectorstore"
)

func newSessionModule(gdb *gorm.DB) (*session.SQLStore, *sessionapp.Service) {
	store := session.NewSQLStore(gdb)
	return store, sessionapp.NewService(store)
}

func newCheckpointStore(redisClient *redis.Client, cfg *config.Config) *checkpoint.RedisStore {
	return checkpoint.NewRedisStore(redisClient, cfg.Session)
}

func newKnowledgeModule(
	ctx context.Context,
	gdb *gorm.DB,
	redisClient *redis.Client,
	vs vectorstore.Store,
	runtime *settingsapp.RuntimeRegistry,
) (*qa.Service, *knowledgeapp.Service, *knowledgeapp.ClassificationJobService, error) {
	embedCfg := runtime.ResolveEmbeddingConfig(ctx)
	embedder, err := runtime.NewEmbedder(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create embedder: %w", err)
	}
	metaStore := qa.NewSQLMetaStore(gdb)
	qaService := qa.NewService(metaStore, vs, embedder, redisClient, embedCfg.Dimensions)
	if err := qaService.EnsureIndex(ctx); err != nil {
		return nil, nil, nil, fmt.Errorf("ensure vector index: %w", err)
	}
	return qaService, knowledgeapp.NewService(qaService), knowledgeapp.NewClassificationJobService(qaService), nil
}

func newModelFactory(runtime *settingsapp.RuntimeRegistry) func(context.Context) (model.ToolCallingChatModel, error) {
	return func(ctx context.Context) (model.ToolCallingChatModel, error) {
		return runtime.NewChatModel(ctx)
	}
}
