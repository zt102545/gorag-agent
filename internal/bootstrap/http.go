package bootstrap

import (
	"github.com/gin-gonic/gin"

	"github.com/gorag-ai/gorag/internal/config"
	"github.com/gorag-ai/gorag/internal/server"
)

func newHTTPRouter(cfg *config.Config, deps *server.Deps) *gin.Engine {
	return server.NewRouter(cfg, deps)
}
