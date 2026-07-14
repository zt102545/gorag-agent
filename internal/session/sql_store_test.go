package session

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

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
