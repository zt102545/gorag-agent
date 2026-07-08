package qa

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	meta    MetaStore
	vector  *VectorIndexer
	redis   *redis.Client
	ensured bool
	dims    int
}

func NewService(meta MetaStore, vector *VectorIndexer, client *redis.Client, dimensions int) *Service {
	return &Service{
		meta:   meta,
		vector: vector,
		redis:  client,
		dims:   dimensions,
	}
}

func (s *Service) ensureIndex(ctx context.Context) error {
	if s.ensured {
		return nil
	}
	if err := s.vector.EnsureIndex(ctx, s.redis, s.dims); err != nil {
		return err
	}
	s.ensured = true
	return nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*Item, error) {
	if err := s.ensureIndex(ctx); err != nil {
		return nil, err
	}
	now := time.Now()
	item := &Item{
		ID:        uuid.NewString(),
		Question:  req.Question,
		Answer:    req.Answer,
		Category:  req.Category,
		Tags:      req.Tags,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.meta.Save(ctx, item); err != nil {
		return nil, fmt.Errorf("save meta: %w", err)
	}
	if err := s.vector.Index(ctx, item); err != nil {
		_ = s.meta.Delete(ctx, item.ID)
		return nil, fmt.Errorf("index vector: %w", err)
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
	oldCategory := existing.Category

	item := &Item{
		ID:        id,
		Question:  req.Question,
		Answer:    req.Answer,
		Category:  req.Category,
		Tags:      req.Tags,
		CreatedAt: existing.CreatedAt,
		UpdatedAt: time.Now(),
	}
	if oldCategory != item.Category {
		_ = s.meta.RemoveFromCategory(ctx, oldCategory, id)
	}
	if err := s.meta.Save(ctx, item); err != nil {
		return nil, fmt.Errorf("save meta: %w", err)
	}
	if err := s.vector.Delete(ctx, s.redis, id); err != nil {
		return nil, fmt.Errorf("delete old vector: %w", err)
	}
	if err := s.vector.Index(ctx, item); err != nil {
		return nil, fmt.Errorf("reindex vector: %w", err)
	}
	return item, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.vector.Delete(ctx, s.redis, id); err != nil {
		return fmt.Errorf("delete vector: %w", err)
	}
	return s.meta.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context, query ListQuery) (*ListResult, error) {
	return s.meta.List(ctx, query)
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
