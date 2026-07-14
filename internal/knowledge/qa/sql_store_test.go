package qa

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/db"
)

func TestSQLMetaStore_CreatedAtByIDsFiltersKnowledgeBase(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&db.QAItem{}); err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	rows := []db.QAItem{
		{ID: "qa-1", KnowledgeBaseID: "kb-1", Question: "Q1", Answer: "A1", CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: "qa-2", KnowledgeBaseID: "kb-2", Question: "Q2", Answer: "A2", CreatedAt: createdAt.Add(time.Hour), UpdatedAt: createdAt},
	}
	if err := gdb.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	got, err := NewSQLMetaStore(gdb).CreatedAtByIDs(context.Background(), "kb-1", []string{"qa-1", "qa-2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got["qa-1"].Equal(createdAt) {
		t.Fatalf("unexpected creation times: %v", got)
	}
}
