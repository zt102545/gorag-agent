package qa

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/yourorg/agent_customer_service/internal/vectorstore"
)

const classifyQueueKey = "cs:qa:classify:queue"

type Service struct {
	meta     MetaStore
	store    vectorstore.Store
	embedder embedding.Embedder
	redis    *redis.Client
	ensureMu sync.Mutex
	ensured  bool
	dims     int
}

func NewService(
	meta MetaStore,
	store vectorstore.Store,
	embedder embedding.Embedder,
	client *redis.Client,
	dimensions int,
) *Service {
	return &Service{
		meta:     meta,
		store:    store,
		embedder: embedder,
		redis:    client,
		dims:     dimensions,
	}
}

func (s *Service) ensureIndex(ctx context.Context) error {
	if s.ensured {
		return nil
	}
	s.ensureMu.Lock()
	defer s.ensureMu.Unlock()
	if s.ensured {
		return nil
	}
	if err := s.store.Ensure(ctx, s.dims); err != nil {
		return err
	}
	s.ensured = true
	return nil
}

func (s *Service) embedQuestion(ctx context.Context, question string) ([]float32, error) {
	vecs, err := s.embedder.EmbedStrings(ctx, []string{question})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}
	out := make([]float32, len(vecs[0]))
	for i, v := range vecs[0] {
		out[i] = float32(v)
	}
	return out, nil
}

func (s *Service) upsertItem(ctx context.Context, item *Item) error {
	vec, err := s.embedQuestion(ctx, item.Question)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	return s.store.Upsert(ctx, vectorstore.Record{
		ID:       item.ID,
		Question: item.Question,
		Answer:   item.Answer,
		Category: item.Category,
		Tags:     item.Tags,
		Vector:   vec,
	})
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*Item, error) {
	if err := s.ensureIndex(ctx); err != nil {
		return nil, err
	}
	now := time.Now()
	status := ClassifyNone
	if NeedsAutoClassify(req.Category, req.Tags) {
		status = ClassifyPending
	}
	item := &Item{
		ID:             uuid.NewString(),
		Question:       req.Question,
		Answer:         req.Answer,
		Category:       req.Category,
		Tags:           req.Tags,
		ClassifyStatus: status,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.meta.Save(ctx, item); err != nil {
		return nil, fmt.Errorf("save meta: %w", err)
	}
	if err := s.upsertItem(ctx, item); err != nil {
		_ = s.meta.Delete(ctx, item.ID)
		return nil, fmt.Errorf("index vector: %w", err)
	}
	if status == ClassifyPending {
		_ = s.EnqueueClassify(ctx, item.ID)
	}
	return item, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Item, error) {
	return s.meta.Get(ctx, id)
}

func (s *Service) Update(ctx context.Context, id string, req UpdateRequest) (*Item, error) {
	if err := s.ensureIndex(ctx); err != nil {
		return nil, err
	}
	existing, err := s.meta.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	status := existing.ClassifyStatus
	if NeedsAutoClassify(req.Category, req.Tags) {
		status = ClassifyPending
	} else if strings.TrimSpace(req.Category) != "" || len(req.Tags) > 0 {
		status = ClassifyNone
	}

	item := &Item{
		ID:             id,
		Question:       req.Question,
		Answer:         req.Answer,
		Category:       req.Category,
		Tags:           req.Tags,
		ClassifyStatus: status,
		ClassifyError:  "",
		ClassifiedAt:   existing.ClassifiedAt,
		CreatedAt:      existing.CreatedAt,
		UpdatedAt:      time.Now(),
	}
	if err := s.meta.Save(ctx, item); err != nil {
		return nil, fmt.Errorf("save meta: %w", err)
	}
	_ = s.store.Delete(ctx, id)
	if err := s.upsertItem(ctx, item); err != nil {
		return nil, fmt.Errorf("reindex vector: %w", err)
	}
	if status == ClassifyPending {
		_ = s.EnqueueClassify(ctx, item.ID)
	}
	return item, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete vector: %w", err)
	}
	return s.meta.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context, query ListQuery) (*ListResult, error) {
	return s.meta.List(ctx, query)
}

func (s *Service) ListCategories(ctx context.Context) ([]string, error) {
	return s.meta.ListCategories(ctx)
}

func (s *Service) ListTags(ctx context.Context) ([]string, error) {
	return s.meta.ListTags(ctx)
}

func (s *Service) ImportCSV(ctx context.Context, r io.Reader) (*ImportResult, error) {
	requests, err := ParseCSV(r)
	if err != nil {
		return nil, err
	}
	result := &ImportResult{}
	for i, req := range requests {
		if _, err := s.Create(ctx, req); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("row %d: %v", i+2, err))
			continue
		}
		result.Imported++
	}
	return result, nil
}

func (s *Service) EnsureIndex(ctx context.Context) error {
	return s.ensureIndex(ctx)
}

func (s *Service) EnqueueClassify(ctx context.Context, id string) error {
	return s.redis.LPush(ctx, classifyQueueKey, id).Err()
}

func (s *Service) Reclassify(ctx context.Context, id string) (*Item, error) {
	item, err := s.meta.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	item.ClassifyStatus = ClassifyPending
	item.ClassifyError = ""
	item.UpdatedAt = time.Now()
	if err := s.meta.Save(ctx, item); err != nil {
		return nil, err
	}
	if err := s.EnqueueClassify(ctx, id); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ApplyClassification(ctx context.Context, id, category string, tags []string) error {
	if err := s.ensureIndex(ctx); err != nil {
		return err
	}
	item, err := s.meta.Get(ctx, id)
	if err != nil {
		return err
	}
	now := time.Now()
	item.Category = category
	item.Tags = tags
	item.ClassifyStatus = ClassifyCompleted
	item.ClassifyError = ""
	item.ClassifiedAt = &now
	item.UpdatedAt = now
	if err := s.meta.Save(ctx, item); err != nil {
		return err
	}
	_ = s.store.Delete(ctx, id)
	return s.upsertItem(ctx, item)
}

func (s *Service) MarkClassifyStatus(ctx context.Context, id, status, errMsg string) error {
	item, err := s.meta.Get(ctx, id)
	if err != nil {
		return err
	}
	item.ClassifyStatus = status
	item.ClassifyError = errMsg
	item.UpdatedAt = time.Now()
	return s.meta.Save(ctx, item)
}

func (s *Service) Redis() *redis.Client {
	return s.redis
}

func (s *Service) Meta() MetaStore {
	return s.meta
}
