package rag

import (
	"strings"

	"github.com/cloudwego/eino/schema"
)

func RewriteQuery(_ []*schema.Message, current string) string {
	// Kept for callers outside the chat flow. Keyword-based concatenation was
	// unsafe because words such as “这个” also occur in independent questions.
	// Contextual rewriting is now handled by ResolveQueryRelation.
	return strings.TrimSpace(current)
}
