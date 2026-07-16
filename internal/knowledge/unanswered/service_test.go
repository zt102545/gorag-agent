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
	// first analysis should still prefill the employee's answer.
	if err := service.Capture(ctx, CaptureInput{Question: "鎬庝箞閫€娆撅紵", TenantID: "default"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Capture(ctx, CaptureInput{Question: "鎬庝箞閫€娆撅紵", TenantID: "default", Source: "analysis", SourceRef: "msg-1", DraftAnswer: "璇锋彁渚涜鍗曞彿"}); err != nil {
		t.Fatal(err)
	}
	var row db.UnansweredQuestion
	if err := gdb.First(&row, "question_hash = ?", hash("default", "鎬庝箞閫€娆撅紵")).Error; err != nil {
		t.Fatal(err)
	}
	if row.Answer != "璇锋彁渚涜鍗曞彿" {
		t.Fatalf("prefilled answer = %q", row.Answer)
	}

	if err := gdb.Model(&row).Update("answer", "浜哄伐瀹℃牳鍚庣殑绛旀").Error; err != nil {
		t.Fatal(err)
	}
	if err := service.Capture(ctx, CaptureInput{Question: "鎬庝箞閫€娆撅紵", TenantID: "default", Source: "analysis", SourceRef: "msg-1", DraftAnswer: "妯″瀷閲嶆柊鐢熸垚鐨勭瓟妗?}); err != nil {
		t.Fatal(err)
	}
	if err := gdb.First(&row, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.Answer != "浜哄伐瀹℃牳鍚庣殑绛旀" {
		t.Fatalf("human answer was overwritten: %q", row.Answer)
	}
}
