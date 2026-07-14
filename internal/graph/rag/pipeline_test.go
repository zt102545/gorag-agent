package rag

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yourorg/agent_customer_service/internal/vectorstore"
)

type staticStore struct {
	records []vectorstore.Record
}

func (s *staticStore) Ensure(context.Context, int) error                { return nil }
func (s *staticStore) Upsert(context.Context, vectorstore.Record) error { return nil }
func (s *staticStore) Delete(context.Context, string) error             { return nil }
func (s *staticStore) SearchText(context.Context, string, int) ([]vectorstore.Record, error) {
	return s.records, nil
}
func (s *staticStore) Search(context.Context, []float32, int) ([]vectorstore.Record, error) {
	return s.records, nil
}
func (s *staticStore) Close() error { return nil }

func TestService_SearchHitsLoadsCreatedAtFromMetadataStore(t *testing.T) {
	createdAt := time.Date(2026, 7, 1, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	service := NewService(&staticStore{records: []vectorstore.Record{{
		ID: "qa-1", Question: "活动什么时候结束？", Answer: "7月31日结束。",
	}}}, nil, 3)
	service.SetKnowledgeBaseFilter(func(context.Context, string, []string) (map[string]struct{}, error) {
		return map[string]struct{}{"qa-1": {}}, nil
	})
	service.SetKnowledgeBaseCreatedAtLoader(func(_ context.Context, _ string, ids []string) (map[string]time.Time, error) {
		if len(ids) != 1 || ids[0] != "qa-1" {
			t.Fatalf("unexpected ids: %v", ids)
		}
		return map[string]time.Time{"qa-1": createdAt}, nil
	})

	hits, err := service.SearchHits(context.Background(), "活动结束时间")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].CreatedAt == nil || !hits[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("created_at was not loaded: %+v", hits)
	}
}

func TestFormatHitsAtIncludesCurrentAndQACreationTimes(t *testing.T) {
	now := time.Date(2026, 7, 14, 16, 20, 0, 0, time.FixedZone("CST", 8*60*60))
	createdAt := time.Date(2026, 6, 10, 8, 15, 0, 0, time.UTC)
	formatted := FormatHitsAt([]SearchHit{{
		ID: "qa-1", Question: "Q", Answer: "A", CreatedAt: &createdAt,
	}}, now)

	for _, want := range []string{
		"当前时间: 2026-07-14T16:20:00+08:00",
		"QA入库时间: 2026-06-10T08:15:00Z",
	} {
		if !strings.Contains(formatted, want) {
			t.Fatalf("formatted result %q does not contain %q", formatted, want)
		}
	}
}
