package session

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestExpiredMessagesQuery_MySQLIncludesLimitBeforeOffset(t *testing.T) {
	gdb, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "user:password@tcp(localhost:3306)/customer_service",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open dry-run MySQL database: %v", err)
	}

	var ids []string
	statement := expiredMessagesQuery(gdb, "session-1").Pluck("id", &ids).Statement
	sql := statement.SQL.String()
	if !strings.Contains(sql, "LIMIT ? OFFSET ?") {
		t.Fatalf("query must include LIMIT before OFFSET for MySQL, got %q", sql)
	}
}

func TestFirstUserTitle_TruncatesByRune(t *testing.T) {
	content := strings.Repeat("中", 41)
	title := firstUserTitle([]*schema.Message{{Role: schema.User, Content: content}})
	if !utf8.ValidString(title) {
		t.Fatalf("title is not valid UTF-8: %x", title)
	}
	if title != strings.Repeat("中", 40)+"..." {
		t.Fatalf("title = %q", title)
	}
}

func TestTrimPreview_TruncatesByRune(t *testing.T) {
	content := strings.Repeat("中", 81)
	preview := trimPreview(content)
	if !utf8.ValidString(preview) {
		t.Fatalf("preview is not valid UTF-8: %x", preview)
	}
	if preview != strings.Repeat("中", 80)+"..." {
		t.Fatalf("preview = %q", preview)
	}
}
