package qa

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestParseCSVWithGBKChinese(t *testing.T) {
	raw := "question,answer,category,tags\r\n螺蛳动画能干什么,\"螺蛳动画是面向短视频创作的工具\",产品,视频;动画\r\n"
	encoded, err := simplifiedchinese.GBK.NewEncoder().String(raw)
	if err != nil {
		t.Fatalf("encode gbk: %v", err)
	}

	items, err := ParseCSV(strings.NewReader(encoded))
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Question != "螺蛳动画能干什么" {
		t.Fatalf("Question = %q", items[0].Question)
	}
	if items[0].Answer != "螺蛳动画是面向短视频创作的工具" {
		t.Fatalf("Answer = %q", items[0].Answer)
	}
	if items[0].Category != "产品" {
		t.Fatalf("Category = %q", items[0].Category)
	}
}

func TestParseCSVWithUTF8BOM(t *testing.T) {
	raw := "\ufeffquestion,answer\r\n你好,中文正常\r\n"

	items, err := ParseCSV(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseCSV() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Question != "你好" || items[0].Answer != "中文正常" {
		t.Fatalf("item = %+v", items[0])
	}
}
