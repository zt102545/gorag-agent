package handler

import "testing"

func TestLooksUnanswered(t *testing.T) {
	if !looksUnanswered("抱歉，知识库中没有足够信息，建议咨询人工客服") {
		t.Fatal("expected refusal to be captured")
	}
	if looksUnanswered("退款通常会在 3 个工作日内到账") {
		t.Fatal("expected grounded answer not to be captured")
	}
}
