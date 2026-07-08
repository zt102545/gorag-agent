package qa

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/redis/go-redis/v9"
)

const (
	metaKeyPrefix     = "cs:qa:meta:"
	categoryKeyPrefix = "cs:qa:cat:"
	allIDsKey         = "cs:qa:all"
)

type RedisMetaStore struct {
	client *redis.Client
}

func NewRedisMetaStore(client *redis.Client) *RedisMetaStore {
	return &RedisMetaStore{client: client}
}

func (s *RedisMetaStore) Save(ctx context.Context, item *Item) error {
	payload, err := json.Marshal(item)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, metaKey(item.ID), payload, 0)
	pipe.SAdd(ctx, allIDsKey, item.ID)
	if item.Category != "" {
		pipe.SAdd(ctx, categoryKey(item.Category), item.ID)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisMetaStore) Get(ctx context.Context, id string) (*Item, error) {
	data, err := s.client.Get(ctx, metaKey(id)).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("qa not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	var item Item
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *RedisMetaStore) Delete(ctx context.Context, id string) error {
	item, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Del(ctx, metaKey(id))
	pipe.SRem(ctx, allIDsKey, id)
	if item.Category != "" {
		pipe.SRem(ctx, categoryKey(item.Category), id)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisMetaStore) List(ctx context.Context, query ListQuery) (*ListResult, error) {
	if query.Category != "" {
		return s.ListByCategory(ctx, query.Category, query.Page, query.Size)
	}
	return s.listAll(ctx, query.Page, query.Size)
}

func (s *RedisMetaStore) listAll(ctx context.Context, page, size int) (*ListResult, error) {
	ids, err := s.client.SMembers(ctx, allIDsKey).Result()
	if err != nil {
		return nil, err
	}
	sort.Strings(ids)
	return s.paginateItems(ctx, ids, page, size)
}

func (s *RedisMetaStore) ListByCategory(ctx context.Context, category string, page, size int) (*ListResult, error) {
	ids, err := s.client.SMembers(ctx, categoryKey(category)).Result()
	if err != nil {
		return nil, err
	}
	sort.Strings(ids)
	return s.paginateItems(ctx, ids, page, size)
}

func (s *RedisMetaStore) paginateItems(ctx context.Context, ids []string, page, size int) (*ListResult, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	total := int64(len(ids))
	start := (page - 1) * size
	if start >= len(ids) {
		return &ListResult{Total: total, Items: []Item{}}, nil
	}
	end := start + size
	if end > len(ids) {
		end = len(ids)
	}
	items := make([]Item, 0, end-start)
	for _, id := range ids[start:end] {
		item, err := s.Get(ctx, id)
		if err != nil {
			continue
		}
		items = append(items, *item)
	}
	return &ListResult{Total: total, Items: items}, nil
}

func (s *RedisMetaStore) AddToCategory(ctx context.Context, category, id string) error {
	if category == "" {
		return nil
	}
	return s.client.SAdd(ctx, categoryKey(category), id).Err()
}

func (s *RedisMetaStore) RemoveFromCategory(ctx context.Context, category, id string) error {
	if category == "" {
		return nil
	}
	return s.client.SRem(ctx, categoryKey(category), id).Err()
}

func metaKey(id string) string {
	return metaKeyPrefix + id
}

func categoryKey(category string) string {
	return categoryKeyPrefix + category
}
