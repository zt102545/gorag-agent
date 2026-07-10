package rag

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestRewriteQueryDoesNotRewriteShortStandaloneQuestion(t *testing.T) {
	history := []*schema.Message{
		schema.UserMessage("first topic"),
		schema.AssistantMessage("first answer", nil),
	}

	got := RewriteQuery(history, "refund?")
	if got != "refund?" {
		t.Fatalf("RewriteQuery() = %q, want current question only", got)
	}
}
