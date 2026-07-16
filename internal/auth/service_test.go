package auth

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/gorag-ai/gorag/internal/config"
	"github.com/gorag-ai/gorag/internal/db"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&db.CallerToken{}); err != nil {
		t.Fatal(err)
	}
	return NewService(gdb, config.JWTConfig{Secret: "test-secret-that-is-not-for-production", Expire: time.Hour})
}

func TestIssueAndValidateUserToken(t *testing.T) {
	svc := newTestService(t)
	secret, app, err := svc.CreateCallerToken(context.Background(), CreateCallerTokenRequest{Name: "H5", ChannelID: "h5", TenantID: "tenant-a"})
	if err != nil {
		t.Fatal(err)
	}
	token, expiresAt, err := svc.IssueUserToken(context.Background(), IssueUserTokenRequest{AppID: app.AppID, AppSecret: secret, CallerUserID: "user-1", ChannelID: "h5", TenantID: "tenant-a"})
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(expiresAt) < 14*time.Minute {
		t.Fatalf("unexpected expiry: %v", expiresAt)
	}
	identity, err := svc.ValidateCallerToken(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if identity.AuthType != "user_token" || identity.CallerUserID != "user-1" || identity.ChannelID != "h5" || identity.TenantID != "tenant-a" {
		t.Fatalf("unexpected identity: %+v", identity)
	}
}

func TestIssueUserTokenRejectsWrongSecretAndScope(t *testing.T) {
	svc := newTestService(t)
	secret, app, err := svc.CreateCallerToken(context.Background(), CreateCallerTokenRequest{Name: "H5", ChannelID: "h5"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.IssueUserToken(context.Background(), IssueUserTokenRequest{AppID: app.AppID, AppSecret: "wrong", CallerUserID: "user-1", ChannelID: "h5"}); err == nil {
		t.Fatal("expected wrong secret to fail")
	}
	if _, _, err := svc.IssueUserToken(context.Background(), IssueUserTokenRequest{AppID: app.AppID, AppSecret: secret, CallerUserID: "user-1", ChannelID: "app"}); err == nil {
		t.Fatal("expected channel mismatch to fail")
	}
}

func TestIssueAndValidatePublicUserToken(t *testing.T) {
	svc := newTestService(t)
	_, app, err := svc.CreateCallerToken(context.Background(), CreateCallerTokenRequest{
		Name: "Public H5", ChannelID: "website-main", TenantID: "tenant-a", PublicAccess: true,
		AllowedOrigins: []string{"https://www.example.com/"}, IPRatePerMinute: 30, IPDailyQuota: 500,
	})
	if err != nil {
		t.Fatal(err)
	}

	token, _, policy, err := svc.IssuePublicUserToken(context.Background(), IssuePublicUserTokenRequest{
		CallerUserID: "user-1", ChannelID: "website-main",
	}, "https://www.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if policy.IPRatePerMinute != 30 || policy.IPDailyQuota != 500 {
		t.Fatalf("unexpected policy: %+v", policy)
	}
	identity, err := svc.ValidateCallerToken(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if identity.AuthType != "public_user_token" || identity.CallerUserID != "user-1" || identity.ChannelID != "website-main" || identity.TenantID != "tenant-a" {
		t.Fatalf("unexpected identity: %+v", identity)
	}
	disabled := false
	if err := svc.UpdateCallerToken(context.Background(), app.ID, UpdateCallerTokenRequest{PublicAccess: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateCallerToken(context.Background(), token); err == nil {
		t.Fatal("expected token to be rejected after public access is disabled")
	}
}

func TestPublicUserTokenRejectsOriginAndDuplicateChannel(t *testing.T) {
	svc := newTestService(t)
	_, _, err := svc.CreateCallerToken(context.Background(), CreateCallerTokenRequest{
		Name: "Public H5", ChannelID: "website-main", PublicAccess: true,
		AllowedOrigins: []string{"https://www.example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := svc.IssuePublicUserToken(context.Background(), IssuePublicUserTokenRequest{CallerUserID: "user-1", ChannelID: "website-main"}, "https://evil.example"); err == nil {
		t.Fatal("expected disallowed origin to fail")
	}
	if _, _, err := svc.CreateCallerToken(context.Background(), CreateCallerTokenRequest{Name: "Duplicate", ChannelID: "website-main", PublicAccess: true}); err == nil {
		t.Fatal("expected duplicate public channel to fail")
	}
}
