package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCustomerServiceWidget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/customer-service-widget.js", CustomerServiceWidget)

	request := httptest.NewRequest(http.MethodGet, "/customer-service-widget.js", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/javascript") {
		t.Fatalf("Content-Type = %q", contentType)
	}
	if !strings.Contains(response.Body.String(), "customElements.define('customer-service-widget'") {
		t.Fatal("embedded widget content is missing")
	}
	if response.Header().Get("ETag") == "" {
		t.Fatal("ETag is missing")
	}
}

func TestCustomerServiceWidgetNotModified(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/customer-service-widget.js", CustomerServiceWidget)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/customer-service-widget.js", nil))
	request := httptest.NewRequest(http.MethodGet, "/customer-service-widget.js", nil)
	request.Header.Set("If-None-Match", first.Header().Get("ETag"))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotModified)
	}
}
