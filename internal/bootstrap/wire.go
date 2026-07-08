package bootstrap

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/yourorg/agent_customer_service/internal/agent"
	"github.com/yourorg/agent_customer_service/internal/checkpoint"
	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/graph/rag"
	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
	"github.com/yourorg/agent_customer_service/internal/model"
	"github.com/yourorg/agent_customer_service/internal/observability"
	"github.com/yourorg/agent_customer_service/internal/server"
	"github.com/yourorg/agent_customer_service/internal/server/handler"
	"github.com/yourorg/agent_customer_service/internal/session"
	redispkg "github.com/yourorg/agent_customer_service/pkg/redis"
	"github.com/yourorg/agent_customer_service/pkg/sse"
)

type Application struct {
	Router      *gin.Engine
	RedisClient *redis.Client
	QAService   *qa.Service
}

func NewApp(ctx context.Context, cfg *config.Config) (*Application, func(), error) {
	observability.Init(ctx)

	redisClient := redispkg.NewClient(cfg.Redis)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, nil, fmt.Errorf("connect redis: %w", err)
	}

	sessionStore := session.NewRedisStore(redisClient, cfg.Session)
	checkpointStore := checkpoint.NewRedisStore(redisClient, cfg.Session)

	chatModel, err := model.NewChatModel(ctx, cfg.AI.ChatModel)
	if err != nil {
		redisClient.Close()
		return nil, nil, fmt.Errorf("create chat model: %w", err)
	}

	embedder, err := model.NewEmbedder(ctx, cfg.AI.Embedding)
	if err != nil {
		redisClient.Close()
		return nil, nil, fmt.Errorf("create embedder: %w", err)
	}

	metaStore := qa.NewRedisMetaStore(redisClient)
	vectorIndexer, err := qa.NewVectorIndexer(redisClient, embedder, cfg.RAG)
	if err != nil {
		redisClient.Close()
		return nil, nil, fmt.Errorf("create qa indexer: %w", err)
	}
	qaService := qa.NewService(metaStore, vectorIndexer, redisClient, cfg.AI.Embedding.Dimensions)
	if err := qaService.EnsureIndex(ctx); err != nil {
		redisClient.Close()
		return nil, nil, fmt.Errorf("ensure qa index: %w", err)
	}

	ragComponents, err := rag.NewRetriever(ctx, redisClient, embedder, cfg.RAG)
	if err != nil {
		redisClient.Close()
		return nil, nil, fmt.Errorf("create rag retriever: %w", err)
	}
	ragService := rag.NewService(ragComponents)

	customerAgent, err := agent.NewCustomerServiceAgent(ctx, chatModel, ragService)
	if err != nil {
		redisClient.Close()
		return nil, nil, fmt.Errorf("create agent: %w", err)
	}

	runner := agent.NewRunner(ctx, customerAgent, checkpointStore)

	sseWriter := sse.NewWriter()
	router := server.NewRouter(cfg, &server.Deps{
		SessionHandler:   handler.NewSessionHandler(sessionStore),
		ChatHandler:      handler.NewChatHandler(runner.Inner(), sessionStore, sseWriter),
		ApprovalHandler:  handler.NewApprovalHandler(runner.Inner(), sessionStore, sseWriter),
		KnowledgeHandler: handler.NewKnowledgeHandler(qaService),
	})

	cleanup := func() {
		_ = redisClient.Close()
	}

	return &Application{
		Router:      router,
		RedisClient: redisClient,
		QAService:   qaService,
	}, cleanup, nil
}
