package unanswered

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/gorag-ai/gorag/internal/db"
)

func TestQuestionHashIsNormalizedAndTenantScoped(t *testing.T) {
	if hash("tenant-a", "  How   To Refund ") != hash("tenant-a", "how to refund") {
		t.Fatal("equivalent questions in one tenant should share a hash")
	}
	if hash("tenant-a", "how to refund") == hash("tenant-b", "how to refund") {
		t.Fatal("the same question in different tenants must not share a hash")
	}
}

func TestCapture_DraftPrefillsButDoesNotOverwriteHumanAnswer(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&db.UnansweredQuestion{}, &db.UnansweredQuestionEvent{}); err != nil {
		t.Fatal(err)
	}
	service := NewService(gdb)
	ctx := context.Background()

	// Simulate the same question already existing from another channel. The
	// first WeCom analysis should still prefill the employee's answer.
	if err := service.Capture(ctx, CaptureInput{Question: "怎么退款？", TenantID: "wecom"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Capture(ctx, CaptureInput{Question: "怎么退款？", TenantID: "wecom", Source: "wecom", SourceRef: "msg-1", DraftAnswer: "请提供订单号"}); err != nil {
		t.Fatal(err)
	}
	var row db.UnansweredQuestion
	if err := gdb.First(&row, "question_hash = ?", hash("wecom", "怎么退款？")).Error; err != nil {
		t.Fatal(err)
	}
	if row.Answer != "请提供订单号" {
		t.Fatalf("prefilled answer = %q", row.Answer)
	}

	if err := gdb.Model(&row).Update("answer", "人工审核后的答案").Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Capture(ctx, CaptureInput{Question: "怎么退款？", TenantID: "wecom", Source: "wecom", SourceRef: "msg-1", DraftAnswer: "模型重新生成的答案"}); err != nil {
		t.Fatal(err)
	}
	if err := gdb.First(&row, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.Answer != "人工审核后的答案" {
		t.Fatalf("human answer was overwritten: %q", row.Answer)
	}
}
