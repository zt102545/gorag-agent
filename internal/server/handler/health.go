package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/gorag-ai/gorag/pkg/response"
)

func Health(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}
