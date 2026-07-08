package bootstrap

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/admin"
	"github.com/yourorg/agent_customer_service/internal/agent"
	"github.com/yourorg/agent_customer_service/internal/auth"
	"github.com/yourorg/agent_customer_service/internal/checkpoint"
	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/db"
	"github.com/yourorg/agent_customer_service/internal/graph/rag"
	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
	modelfactory "github.com/yourorg/agent_customer_service/internal/model"
	"github.com/yourorg/agent_customer_service/internal/observability"
	"github.com/yourorg/agent_customer_service/internal/server"
	"github.com/yourorg/agent_customer_service/internal/server/handler"
	"github.com/yourorg/agent_customer_service/internal/session"
	"github.com/yourorg/agent_customer_service/internal/vectorstore"
	redispkg "github.com/yourorg/agent_customer_service/pkg/redis"
	"github.com/yourorg/agent_customer_service/pkg/sse"
)

type Application struct {
	Router      *gin.Engine
	RedisClient *redis.Client
	DB          *gorm.DB
	QAService   *qa.Service
	VectorStore vectorstore.Store
}

func NewApp(ctx context.Context, cfg *config.Config) (*Application, func(), error) {
	observability.Init(ctx)

	gdb, err := db.Open(cfg.Database)
	if err != nil {
		return nil, nil, err
	}
	if err := db.AutoMigrate(gdb); err != nil {
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	redisClient := redispkg.NewClient(cfg.Redis)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, nil, fmt.Errorf("connect redis: %w", err)
	}

	authSvc := auth.NewService(gdb, cfg.JWT)
	if err := authSvc.SeedAdmin(ctx, cfg.Admin.Username, cfg.Admin.Password); err != nil {
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("seed admin: %w", err)
	}

	settingsSvc := admin.NewSettingsService(gdb, cfg)
	if err := settingsSvc.SeedDefaults(ctx); err != nil {
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("seed settings: %w", err)
	}

	sessionStore := session.NewSQLStore(gdb)
	checkpointStore := checkpoint.NewRedisStore(redisClient, cfg.Session)

	chatCfg := settingsSvc.ResolveChatConfig(ctx)
	chatModel, err := modelfactory.NewChatModel(ctx, chatCfg)
	if err != nil {
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("create chat model: %w", err)
	}

	embedCfg := settingsSvc.ResolveEmbeddingConfig(ctx)
	embedder, err := modelfactory.NewEmbedder(ctx, embedCfg)
	if err != nil {
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("create embedder: %w", err)
	}

	vs, err := vectorstore.New(ctx, cfg.VectorStore, redisClient)
	if err != nil {
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("create vector store: %w", err)
	}

	metaStore := qa.NewSQLMetaStore(gdb)
	qaService := qa.NewService(metaStore, vs, embedder, redisClient, embedCfg.Dimensions)
	if err := qaService.EnsureIndex(ctx); err != nil {
		_ = vs.Close()
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("ensure vector index: %w", err)
	}

	ragService := rag.NewService(vs, embedder, cfg.VectorStore.TopK)

	instruction := settingsSvc.GetPromptContent(ctx, admin.PromptAgentInstruction, agent.CustomerServiceInstruction)
	customerAgent, err := agent.NewCustomerServiceAgent(ctx, chatModel, ragService, instruction)
	if err != nil {
		_ = vs.Close()
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("create agent: %w", err)
	}

	runner := agent.NewRunner(ctx, customerAgent, checkpointStore)

	classifier := qa.NewClassifier(qaService, settingsSvc, func(c context.Context) (model.ToolCallingChatModel, error) {
		return modelfactory.NewChatModel(c, settingsSvc.ResolveChatConfig(c))
	}, cfg.Classify.Workers)
	classifier.Start(ctx)

	intentAnalyzer := rag.NewIntentAnalyzer(settingsSvc, func(c context.Context) (model.ToolCallingChatModel, error) {
		return modelfactory.NewChatModel(c, settingsSvc.ResolveChatConfig(c))
	}, admin.PromptIntentRoute, 0.6)

	sseWriter := sse.NewWriter()
	router := server.NewRouter(cfg, &server.Deps{
		Auth:                authSvc,
		SessionHandler:      handler.NewSessionHandler(sessionStore),
		ChatHandler:         handler.NewChatHandler(runner.Inner(), sessionStore, sseWriter, intentAnalyzer),
		ApprovalHandler:     handler.NewApprovalHandler(runner.Inner(), sessionStore, sseWriter),
		KnowledgeHandler:    handler.NewKnowledgeHandler(qaService),
		AuthHandler:         handler.NewAuthHandler(authSvc),
		UserHandler:         handler.NewUserHandler(authSvc),
		APIKeyHandler:       handler.NewAPIKeyHandler(authSvc),
		SettingsHandler:     handler.NewSettingsHandler(settingsSvc),
		AdminSessionHandler: handler.NewAdminSessionHandler(sessionStore),
	})

	cleanup := func() {
		_ = vs.Close()
		_ = redisClient.Close()
	}

	return &Application{
		Router:      router,
		RedisClient: redisClient,
		DB:          gdb,
		QAService:   qaService,
		VectorStore: vs,
	}, cleanup, nil
}
