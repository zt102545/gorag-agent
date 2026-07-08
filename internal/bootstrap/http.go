package bootstrap

import (
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/server"
)

func newHTTPRouter(cfg *config.Config, deps *server.Deps) *gin.Engine {
	return server.NewRouter(cfg, deps)
}
