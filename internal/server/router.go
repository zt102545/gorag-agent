package server

import (
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/server/handler"
	"github.com/yourorg/agent_customer_service/internal/server/middleware"
)

type Deps struct {
	SessionHandler   *handler.SessionHandler
	ChatHandler      *handler.ChatHandler
	ApprovalHandler  *handler.ApprovalHandler
	KnowledgeHandler *handler.KnowledgeHandler
}

func NewRouter(cfg *config.Config, deps *Deps) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logging(), middleware.CORS())

	r.GET("/health", handler.Health)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/health", handler.Health)
		sessions := v1.Group("/sessions")
		{
			sessions.POST("", deps.SessionHandler.Create)
			sessions.GET("/:id", deps.SessionHandler.Get)
			sessions.GET("/:id/history", deps.SessionHandler.History)
			sessions.DELETE("/:id", deps.SessionHandler.Delete)
			sessions.POST("/:id/chat", deps.ChatHandler.Chat)
			sessions.POST("/:id/approve", deps.ApprovalHandler.Approve)
		}
		knowledge := v1.Group("/knowledge/qa")
		{
			knowledge.POST("", deps.KnowledgeHandler.Create)
			knowledge.GET("", deps.KnowledgeHandler.List)
			knowledge.POST("/import", deps.KnowledgeHandler.Import)
			knowledge.GET("/:id", deps.KnowledgeHandler.Get)
			knowledge.PUT("/:id", deps.KnowledgeHandler.Update)
			knowledge.DELETE("/:id", deps.KnowledgeHandler.Delete)
		}
	}
	return r
}
