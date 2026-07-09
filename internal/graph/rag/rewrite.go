package rag

import (
	"strings"

	"github.com/cloudwego/eino/schema"
)

var rewriteMarkers = []string{
	"这个", "这个问题", "这个规则", "这个订单", "这个退款",
	"那个", "那个问题", "那个订单", "那个退款",
	"它", "它的", "那", "那这个", "那这个呢", "那怎么办", "那多久",
}

func RewriteQuery(history []*schema.Message, current string) string {
	current = strings.TrimSpace(current)
	if current == "" {
		return ""
	}
	if !needsRewrite(current) {
		return current
	}
	lastUser := lastUserMessage(history)
	if lastUser == "" {
		return current
	}
	return strings.TrimSpace(lastUser + "；补充追问：" + current)
}

func needsRewrite(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	if len([]rune(content)) <= 8 {
		return true
	}
	for _, marker := range rewriteMarkers {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}

func lastUserMessage(history []*schema.Message) string {
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg == nil || msg.Role != schema.User {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content != "" {
			return content
		}
	}
	return ""
}
