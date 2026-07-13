package unanswered

import "testing"

func TestQuestionHashIsNormalizedAndTenantScoped(t *testing.T) {
	if hash("tenant-a", "  How   To Refund ") != hash("tenant-a", "how to refund") {
		t.Fatal("equivalent questions in one tenant should share a hash")
	}
	if hash("tenant-a", "how to refund") == hash("tenant-b", "how to refund") {
		t.Fatal("the same question in different tenants must not share a hash")
	}
}
