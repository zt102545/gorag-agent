package server

import (
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/auth"
	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/server/handler"
	"github.com/yourorg/agent_customer_service/internal/server/middleware"
)

type Deps struct {
	Auth                *auth.Service
	SessionHandler      *handler.SessionHandler
	ChatHandler         *handler.ChatHandler
	ApprovalHandler     *handler.ApprovalHandler
	KnowledgeHandler    *handler.KnowledgeHandler
	AuthHandler         *handler.AuthHandler
	UserHandler         *handler.UserHandler
	CallerTokenHandler  *handler.CallerTokenHandler
	SettingsHandler     *handler.SettingsHandler
	AdminSessionHandler *handler.AdminSessionHandler
	AIErrorHandler      *handler.AIErrorHandler
}

func NewRouter(cfg *config.Config, deps *Deps) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logging(), middleware.CORS())

	r.GET("/health", handler.Health)
	r.GET("/api/v1/health", handler.Health)

	biz := r.Group("/api/v1/biz", middleware.BizAuth(deps.Auth), middleware.BizRateLimit())
	{
		biz.POST("/chat/ask", deps.ChatHandler.Ask)
		biz.GET("/users/:callerUserID/sessions", deps.SessionHandler.ListByUser)

		sessions := biz.Group("/sessions")
		{
			sessions.POST("", deps.SessionHandler.Create)
			sessions.GET("/:id", deps.SessionHandler.Get)
			sessions.GET("/:id/history", deps.SessionHandler.History)
			sessions.DELETE("/:id", deps.SessionHandler.Delete)
			sessions.POST("/:id/chat", middleware.SessionSerial(), deps.ChatHandler.Chat)
			sessions.POST("/:id/approve", middleware.SessionSerial(), deps.ApprovalHandler.Approve)
		}
	}

	admin := r.Group("/api/v1/admin")
	{
		admin.POST("/auth/login", deps.AuthHandler.Login)

		authed := admin.Group("", middleware.JWTAuth(deps.Auth))
		{
			authed.GET("/auth/me", deps.AuthHandler.Me)

			qaRead := authed.Group("/knowledge/qa", middleware.RequireRole(auth.RoleViewer))
			{
				qaRead.GET("", deps.KnowledgeHandler.List)
				qaRead.GET("/categories", deps.KnowledgeHandler.Categories)
				qaRead.GET("/tags", deps.KnowledgeHandler.Tags)
				qaRead.GET("/:id", deps.KnowledgeHandler.Get)
			}
			qaWrite := authed.Group("/knowledge/qa", middleware.RequireRole(auth.RoleEditor))
			{
				qaWrite.POST("", deps.KnowledgeHandler.Create)
				qaWrite.POST("/import", deps.KnowledgeHandler.Import)
				qaWrite.POST("/:id/reclassify", deps.KnowledgeHandler.Reclassify)
				qaWrite.PUT("/:id", deps.KnowledgeHandler.Update)
				qaWrite.DELETE("/:id", deps.KnowledgeHandler.Delete)
			}

			sessions := authed.Group("/sessions", middleware.RequireRole(auth.RoleViewer))
			{
				sessions.GET("", deps.AdminSessionHandler.List)
				sessions.GET("/:id", deps.AdminSessionHandler.Get)
			}

			// GM 测试客服（与 Biz 共用同一套对话能力，JWT 鉴权）
			playground := authed.Group("/playground", middleware.RequireRole(auth.RoleViewer))
			{
				playground.POST("/sessions", deps.SessionHandler.Create)
				playground.GET("/sessions/:id", deps.SessionHandler.Get)
				playground.GET("/sessions/:id/history", deps.SessionHandler.History)
				playground.DELETE("/sessions/:id", deps.SessionHandler.Delete)
				playground.POST("/sessions/:id/chat", deps.ChatHandler.Chat)
				playground.POST("/sessions/:id/approve", deps.ApprovalHandler.Approve)
			}

			promptsRead := authed.Group("/prompts", middleware.RequireRole(auth.RoleEditor))
			{
				promptsRead.GET("", deps.SettingsHandler.ListPrompts)
				promptsRead.GET("/:key", deps.SettingsHandler.GetPrompt)
			}

			adminOnly := authed.Group("", middleware.RequireRole(auth.RoleAdmin))
			{
				adminOnly.GET("/users", deps.UserHandler.List)
				adminOnly.POST("/users", deps.UserHandler.Create)
				adminOnly.PUT("/users/:id", deps.UserHandler.Update)
				adminOnly.DELETE("/users/:id", deps.UserHandler.Delete)

				adminOnly.GET("/ai-errors", deps.AIErrorHandler.List)
				adminOnly.GET("/ai-errors/unresolved-count", deps.AIErrorHandler.UnresolvedCount)
				adminOnly.PUT("/ai-errors/:id/resolve", deps.AIErrorHandler.Resolve)

				adminOnly.GET("/caller-tokens", deps.CallerTokenHandler.List)
				adminOnly.POST("/caller-tokens", deps.CallerTokenHandler.Create)
				adminOnly.PUT("/caller-tokens/:id", deps.CallerTokenHandler.Update)
				adminOnly.DELETE("/caller-tokens/:id", deps.CallerTokenHandler.Delete)

				adminOnly.GET("/providers/chat", deps.SettingsHandler.GetChatProvider)
				adminOnly.PUT("/providers/chat", deps.SettingsHandler.UpdateChatProvider)
				adminOnly.GET("/providers/embedding", deps.SettingsHandler.GetEmbeddingProvider)
				adminOnly.PUT("/providers/embedding", deps.SettingsHandler.UpdateEmbeddingProvider)
				adminOnly.GET("/providers/:kind/profiles", deps.SettingsHandler.ListProviderProfiles)
				adminOnly.POST("/providers/:kind/profiles", deps.SettingsHandler.CreateProviderProfile)
				adminOnly.PUT("/providers/:kind/profiles/:id", deps.SettingsHandler.UpdateProviderProfile)
				adminOnly.PUT("/providers/:kind/profiles/:id/activate", deps.SettingsHandler.ActivateProviderProfile)
				adminOnly.DELETE("/providers/:kind/profiles/:id", deps.SettingsHandler.DeleteProviderProfile)
				adminOnly.GET("/agent-config", deps.SettingsHandler.GetAgentConfig)
				adminOnly.PUT("/agent-config", deps.SettingsHandler.UpdateAgentConfig)
				adminOnly.GET("/chat-light-rules", deps.SettingsHandler.ListChatLightRules)
				adminOnly.POST("/chat-light-rules", deps.SettingsHandler.CreateChatLightRule)
				adminOnly.PUT("/chat-light-rules/:id", deps.SettingsHandler.UpdateChatLightRule)
				adminOnly.DELETE("/chat-light-rules/:id", deps.SettingsHandler.DeleteChatLightRule)

				adminOnly.PUT("/prompts/:key", deps.SettingsHandler.UpdatePrompt)
			}
		}
	}

	// External GM console compatibility API for a trusted intranet deployment.
	// It exposes only the operational views requested by the GM UI and does not
	// change native admin/biz APIs.
	gm := r.Group("/api/v1/gm/ai-customer-service")
	{
		gm.GET("/qa", deps.KnowledgeHandler.GMList)
		gm.POST("/qa/import", deps.KnowledgeHandler.GMImport)
		gm.GET("/sessions", deps.AdminSessionHandler.GMList)
		gm.GET("/sessions/:id", deps.AdminSessionHandler.GMGet)
		gm.GET("/prompts", deps.SettingsHandler.GMListPrompts)
		gm.PUT("/prompts/:key", deps.SettingsHandler.GMUpdatePrompt)

		// Test chat only needs a caller token. It reuses the same BizAuth, session
		// persistence and chat pipeline as external callers.
		test := gm.Group("/test", middleware.BizAuth(deps.Auth), middleware.BizRateLimit())
		{
			test.POST("/sessions", deps.SessionHandler.Create)
			test.POST("/sessions/:id/chat", middleware.SessionSerial(), deps.ChatHandler.Chat)
		}
	}

	return r
}
