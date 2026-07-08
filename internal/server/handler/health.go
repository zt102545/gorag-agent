package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/yourorg/agent_customer_service/pkg/response"
)

func Health(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}
