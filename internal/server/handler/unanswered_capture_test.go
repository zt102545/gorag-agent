package handler

import (
	"testing"

	"github.com/yourorg/agent_customer_service/internal/graph/rag"
)

func TestEvaluateHitLevelIncludesLowConfidence(t *testing.T) {
	tests := []struct{ query, question, want string }{
		{"退款多久到账", "退款多久到账", "high"},
		{"退款多久到账", "退款申请需要什么材料", "mid"},
		{"退款多久到账", "如何修改登录密码", "low"},
	}
	for _, tt := range tests {
		got := evaluateHitLevel(tt.query, []rag.SearchHit{{Question: tt.question}})
		if got != tt.want {
			t.Errorf("%q vs %q: want %s, got %s", tt.query, tt.question, tt.want, got)
		}
	}
}

func TestLooksUnanswered(t *testing.T) {
	if !looksUnanswered("抱歉，知识库中没有足够信息，建议咨询人工客服") {
		t.Fatal("expected refusal to be captured")
	}
	if looksUnanswered("退款通常会在 3 个工作日内到账") {
		t.Fatal("expected grounded answer not to be captured")
	}
}
