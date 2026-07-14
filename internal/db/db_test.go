package db

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/appscope"
)

type legacyQAItem struct {
	ID       string `gorm:"primaryKey;size:36"`
	Question string `gorm:"type:text;not null"`
	Answer   string `gorm:"type:text;not null"`
}

func (legacyQAItem) TableName() string { return "qa_items" }

func TestAutoMigrate_AddsDefaultKnowledgeBaseToLegacyQA(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&legacyQAItem{}); err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&legacyQAItem{ID: "legacy", Question: "q", Answer: "a"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&QAItem{}); err != nil {
		t.Fatalf("upgrade legacy qa table: %v", err)
	}
	var item QAItem
	if err := gdb.First(&item, "id = ?", "legacy").Error; err != nil {
		t.Fatal(err)
	}
	if item.KnowledgeBaseID != appscope.DefaultKnowledgeBaseID {
		t.Fatalf("knowledge_base_id = %q", item.KnowledgeBaseID)
	}
}
