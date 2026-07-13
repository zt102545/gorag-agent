package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/auth"
	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/db"
)

func TestPublicUserTokenOverridesIdentityHeaders(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&db.CallerToken{}); err != nil {
		t.Fatal(err)
	}
	authService := auth.NewService(gdb, config.JWTConfig{Secret: "test-secret", Expire: time.Hour})
	_, _, err = authService.CreateCallerToken(context.Background(), auth.CreateCallerTokenRequest{
		Name: "Public", ChannelID: "website-main", TenantID: "tenant-a", PublicAccess: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	token, _, _, err := authService.IssuePublicUserToken(context.Background(), auth.IssuePublicUserTokenRequest{
		CallerUserID: "user-1", ChannelID: "website-main",
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", BizAuth(authService), func(c *gin.Context) {
		c.JSON(http.StatusOK, GetBizContext(c))
	})
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Caller-User-Id", "other-user")
	request.Header.Set("X-Channel-Id", "other-channel")
	request.Header.Set("X-Tenant-Id", "other-tenant")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var got BizContext
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.CallerUserID != "user-1" || got.ChannelID != "website-main" || got.TenantID != "tenant-a" {
		t.Fatalf("unexpected scoped identity: %+v", got)
	}
}
