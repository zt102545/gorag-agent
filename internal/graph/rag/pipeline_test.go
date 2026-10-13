package rag

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"

	"github.com/gorag-ai/gorag/internal/vectorstore"
)

type staticStore struct {
	mu                sync.Mutex
	textRecords       []vectorstore.Record
	vectorRecords     []vectorstore.Record
	textErr           error
	vectorErr         error
	textSearchCalls   int
	vectorSearchCalls int
	lastTextQuery     string
	lastVectorQuery   []float32
	lastLimit         int
}

func (s *staticStore) Ensure(context.Context, int) error                { return nil }
func (s *staticStore) Upsert(context.Context, vectorstore.Record) error { return nil }
func (s *staticStore) Delete(context.Context, string) error             { return nil }
func (s *staticStore) SearchText(_ context.Context, query string, limit int) ([]vectorstore.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.textSearchCalls++
	s.lastTextQuery = query
	s.lastLimit = limit
	return s.textRecords, s.textErr
}
func (s *staticStore) Search(_ context.Context, query []float32, limit int) ([]vectorstore.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vectorSearchCalls++
	s.lastVectorQuery = append([]float32(nil), query...)
	s.lastLimit = limit
	return s.vectorRecords, s.vectorErr
}
func (s *staticStore) Close() error { return nil }

type staticEmbedder struct {
	vector []float64
	err    error
}

func (e *staticEmbedder) EmbedStrings(context.Context, []string, ...embedding.Option) ([][]float64, error) {
	if e.err != nil {
		return nil, e.err
	}
	return [][]float64{e.vector}, nil
}

func TestService_SearchHitsLoadsCreatedAtFromMetadataStore(t *testing.T) {
	createdAt := time.Date(2026, 7, 1, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	store := &staticStore{
		textRecords: []vectorstore.Record{{
			ID: "qa-1", Question: "活动什么时候结束？", Answer: "7月31日结束。",
		}},
		vectorRecords: []vectorstore.Record{{
			ID: "qa-1", Question: "活动什么时候结束？", Answer: "7月31日结束。",
		}},
	}
	service := NewService(store, &staticEmbedder{vector: []float64{0.25, 0.75}}, 3)
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
	if store.textSearchCalls != 1 || store.vectorSearchCalls != 1 || store.lastTextQuery != "活动结束时间" {
		t.Fatalf("expected one text and one vector search: %+v", store)
	}
	if len(store.lastVectorQuery) != 2 || store.lastVectorQuery[0] != 0.25 || store.lastVectorQuery[1] != 0.75 {
		t.Fatalf("unexpected embedded vector: %v", store.lastVectorQuery)
	}
	if store.lastLimit != 100 {
		t.Fatalf("candidate limit=%d, want 100", store.lastLimit)
	}
}

func TestFuseRecords_PromotesDocumentsFoundByBothRetrievers(t *testing.T) {
	text := []vectorstore.Record{
		{ID: "lexical", Question: "only BM25"},
		{ID: "both", Question: "both"},
	}
	vector := []vectorstore.Record{
		{ID: "semantic", Question: "only vector"},
		{ID: "both", Answer: "merged answer"},
	}

	got := fuseRecords(text, vector, 3)
	if len(got) != 3 || got[0].ID != "both" {
		t.Fatalf("fused order=%v, want shared result first", idsFromRecords(got))
	}
	if got[0].Answer != "merged answer" {
		t.Fatalf("record fields were not merged: %+v", got[0])
	}
	if got[0].Score <= got[1].Score || got[0].Score > 1 {
		t.Fatalf("unexpected normalized scores: %+v", got)
	}
}

func TestRerankRecords_PromotesNearExactQuestionVariant(t *testing.T) {
	records := []vectorstore.Record{
		{ID: "related", Question: "什么是单分镜剧本改写", Score: 0.56},
		{ID: "target", Question: "单分镜改写功能有什么用途？", Score: 0.55},
	}
	got := rerankRecords("单分镜改写功能有什么用", records)
	if got[0].ID != "target" {
		t.Fatalf("reranked order=%v, want near-exact target first", idsFromRecords(got))
	}
	if got[0].Score <= got[1].Score {
		t.Fatalf("reranked scores are not ordered: %+v", got)
	}
}

func TestService_SearchHitsFallsBackWhenOneRetrieverFails(t *testing.T) {
	t.Run("BM25 failure uses vector", func(t *testing.T) {
		store := &staticStore{
			textErr:       errors.New("redis text unavailable"),
			vectorRecords: []vectorstore.Record{{ID: "vector"}},
		}
		service := NewService(store, &staticEmbedder{vector: []float64{1}}, 3)
		hits, err := service.SearchHits(context.Background(), "query")
		if err != nil || len(hits) != 1 || hits[0].ID != "vector" {
			t.Fatalf("hits=%+v err=%v", hits, err)
		}
	})

	t.Run("embedding failure uses BM25", func(t *testing.T) {
		store := &staticStore{textRecords: []vectorstore.Record{{ID: "text"}}}
		service := NewService(store, &staticEmbedder{err: errors.New("embedding unavailable")}, 3)
		hits, err := service.SearchHits(context.Background(), "query")
		if err != nil || len(hits) != 1 || hits[0].ID != "text" {
			t.Fatalf("hits=%+v err=%v", hits, err)
		}
	})

	t.Run("Milvus reports vector failure", func(t *testing.T) {
		store := &staticStore{textErr: vectorstore.ErrTextSearchUnsupported}
		service := NewService(store, &staticEmbedder{err: errors.New("embedding unavailable")}, 3)
		if _, err := service.SearchHits(context.Background(), "query"); err == nil {
			t.Fatal("expected vector-only backend failure")
		}
	})
}

func idsFromRecords(records []vectorstore.Record) []string {
	ids := make([]string, len(records))
	for i := range records {
		ids[i] = records[i].ID
	}
	return ids
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
