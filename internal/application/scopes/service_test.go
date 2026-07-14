package scopes

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/appscope"
	"github.com/yourorg/agent_customer_service/internal/db"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := gdb.AutoMigrate(&db.Prompt{}, &db.PromptGroup{}, &db.PromptGroupItem{}, &db.KnowledgeBase{}, &db.QAItem{}, &db.CallerToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return gdb
}

func TestSeedDefaults_MigratesLegacyDataIdempotently(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	legacyPrompt := db.Prompt{Key: "agent_instruction", Content: "legacy prompt", UpdatedAt: time.Now()}
	legacyQA := db.QAItem{ID: "qa-1", Question: "q", Answer: "a", ClassifyStatus: "none"}
	if err := gdb.Create(&legacyPrompt).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&legacyQA).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Model(&db.QAItem{}).Where("id = ?", legacyQA.ID).Update("knowledge_base_id", "").Error; err != nil {
		t.Fatal(err)
	}
	if err := SeedDefaults(ctx, gdb); err != nil {
		t.Fatal(err)
	}
	if err := SeedDefaults(ctx, gdb); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	var item db.PromptGroupItem
	if err := gdb.Where("group_id = ? AND prompt_key = ?", appscope.DefaultPromptGroupID, legacyPrompt.Key).First(&item).Error; err != nil {
		t.Fatal(err)
	}
	if item.Content != legacyPrompt.Content {
		t.Fatalf("prompt content = %q", item.Content)
	}
	var gotQA db.QAItem
	if err := gdb.First(&gotQA, "id = ?", legacyQA.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotQA.KnowledgeBaseID != appscope.DefaultKnowledgeBaseID {
		t.Fatalf("knowledge base = %q", gotQA.KnowledgeBaseID)
	}
	var count int64
	gdb.Model(&db.PromptGroupItem{}).Where("group_id = ? AND prompt_key = ?", appscope.DefaultPromptGroupID, legacyPrompt.Key).Count(&count)
	if count != 1 {
		t.Fatalf("default prompt copies = %d", count)
	}
}

func TestPromptGroupCopyAndDeleteBindingGuard(t *testing.T) {
	gdb := openTestDB(t)
	ctx := context.Background()
	if err := gdb.Create(&db.Prompt{Key: "chat_no_hit", Content: "default", UpdatedAt: time.Now()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := SeedDefaults(ctx, gdb); err != nil {
		t.Fatal(err)
	}
	service := NewService(gdb, nil)
	copy, err := service.CopyPromptGroup(ctx, appscope.DefaultPromptGroupID, CopyRequest{Name: "渠道 A"})
	if err != nil {
		t.Fatal(err)
	}
	items, err := service.ListPrompts(ctx, copy.ID)
	if err != nil || len(items) != 1 || items[0].Content != "default" {
		t.Fatalf("copied items = %#v, err=%v", items, err)
	}
	if err := gdb.Create(&db.CallerToken{ID: "token-1", Name: "app", TokenHash: "hash", Prefix: "prefix", Enabled: true, PromptGroupID: copy.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.DeletePromptGroup(ctx, copy.ID); err == nil {
		t.Fatal("expected bound prompt group deletion to fail")
	}
}
