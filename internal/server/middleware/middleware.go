package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDKey = "request_id"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(RequestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func Logging() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("[HTTP] %s %s %d %s %v",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			c.GetString(RequestIDKey),
			time.Since(start),
		)
	}
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
