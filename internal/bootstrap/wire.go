package bootstrap

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/admin"
	"github.com/yourorg/agent_customer_service/internal/agent"
	scopesapp "github.com/yourorg/agent_customer_service/internal/application/scopes"
	"github.com/yourorg/agent_customer_service/internal/auth"
	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/graph/rag"
	"github.com/yourorg/agent_customer_service/internal/knowledge/qa"
	"github.com/yourorg/agent_customer_service/internal/knowledge/unanswered"
	"github.com/yourorg/agent_customer_service/internal/observability"
	"github.com/yourorg/agent_customer_service/internal/server"
	"github.com/yourorg/agent_customer_service/internal/server/handler"
	"github.com/yourorg/agent_customer_service/internal/server/middleware"
	"github.com/yourorg/agent_customer_service/internal/vectorstore"
	"github.com/yourorg/agent_customer_service/internal/wecom"
	wecomsdk "github.com/yourorg/agent_customer_service/internal/wecom/sdk"
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

	gdb, err := newDatabase(cfg)
	if err != nil {
		return nil, nil, err
	}
	redisClient, err := newRedisClient(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	authSvc := auth.NewService(gdb, cfg.JWT)
	publicLimiter := middleware.NewPublicIPLimiter(redisClient)
	aiErrorSvc := admin.NewAIErrorService(gdb)
	if err := authSvc.SeedAdmin(ctx, cfg.Admin.Username, cfg.Admin.Password); err != nil {
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("seed admin: %w", err)
	}

	settingsSvc, runtime, settingsApp := newSettingsServices(gdb, cfg)
	if err := settingsSvc.SeedDefaults(ctx); err != nil {
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("seed settings: %w", err)
	}
	if err := scopesapp.SeedDefaults(ctx, gdb); err != nil {
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("seed prompt and knowledge scopes: %w", err)
	}

	sessionStore, sessionApp := newSessionModule(gdb)
	checkpointStore := newCheckpointStore(redisClient, cfg)

	// Keep stable wrappers in the long-lived services. They resolve the active
	// profile at call time, so saving or activating a model profile is hot-applied.
	chatModel := runtime.DynamicChatModel()
	embedder := runtime.DynamicEmbedder()

	vs, err := vectorstore.New(ctx, cfg.VectorStore, redisClient)
	if err != nil {
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("create vector store: %w", err)
	}

	qaService, knowledgeApp, knowledgeJobs, err := newKnowledgeModule(ctx, gdb, redisClient, vs, runtime)
	if err != nil {
		_ = vs.Close()
		_ = redisClient.Close()
		return nil, nil, err
	}
	unansweredService := unanswered.NewService(gdb)

	ragService := rag.NewService(vs, embedder, cfg.VectorStore.TopK)
	ragService.SetKnowledgeBaseFilter(qaService.FilterKnowledgeBaseIDs)
	ragService.SetKnowledgeBaseCreatedAtLoader(qaService.KnowledgeBaseCreatedAt)

	instruction := runtime.GetPromptContent(ctx, admin.PromptAgentInstruction, agent.CustomerServiceInstruction)
	customerAgent, err := agent.NewCustomerServiceAgent(ctx, chatModel, ragService, instruction)
	if err != nil {
		_ = vs.Close()
		_ = redisClient.Close()
		return nil, nil, fmt.Errorf("create agent: %w", err)
	}

	runner := agent.NewRunner(ctx, customerAgent, checkpointStore)

	classifier := qa.NewClassifier(qaService, runtime, newModelFactory(runtime), cfg.Classify.Workers, aiErrorSvc)
	classifier.Start(ctx)

	wecomService := wecom.NewService(gdb, wecomsdk.NativeFactory{}, ragService, newModelFactory(runtime), unansweredService, cfg.WeCom.EncryptionKey)
	wecomService.Start(ctx)
	scopeService := scopesapp.NewService(gdb, qaService)

	sseWriter := sse.NewWriter()
	router := newHTTPRouter(cfg, &server.Deps{
		Auth:                authSvc,
		SessionHandler:      handler.NewSessionHandler(sessionApp),
		ChatHandler:         handler.NewChatHandler(runner.Inner(), sessionStore, sseWriter, settingsApp, aiErrorSvc, unansweredService),
		ApprovalHandler:     handler.NewApprovalHandler(runner.Inner(), sessionStore, sseWriter),
		KnowledgeHandler:    handler.NewKnowledgeHandler(knowledgeApp, knowledgeJobs, settingsApp),
		AuthHandler:         handler.NewAuthHandler(authSvc),
		UserHandler:         handler.NewUserHandler(authSvc),
		CallerTokenHandler:  handler.NewCallerTokenHandler(authSvc, publicLimiter),
		SettingsHandler:     handler.NewSettingsHandler(settingsApp),
		AdminSessionHandler: handler.NewAdminSessionHandler(sessionApp),
		AIErrorHandler:      handler.NewAIErrorHandler(aiErrorSvc),
		UnansweredHandler:   handler.NewUnansweredHandler(unansweredService, knowledgeApp),
		ScopeHandler:        handler.NewScopeHandler(scopeService, settingsApp),
		WeComHandler:        handler.NewWeComHandler(wecomService),
		PublicLimiter:       publicLimiter,
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
