package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "ok", Data: data})
}

func Error(c *gin.Context, status int, message string) {
	c.JSON(status, Body{Code: status, Message: message})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}
