package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	widgetassets "github.com/yourorg/agent_customer_service/web/public"
)

func CustomerServiceWidget(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=300, must-revalidate")
	c.Header("ETag", widgetassets.WidgetETag)
	if c.GetHeader("If-None-Match") == widgetassets.WidgetETag {
		c.Status(http.StatusNotModified)
		return
	}

	data, err := widgetassets.Files.ReadFile("customer-service-widget.js")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", data)
}
