package qa

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

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
	var previous *Item
	previous, err = s.Get(ctx, item.ID)
	if err != nil && !strings.Contains(err.Error(), "qa not found:") {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, metaKey(item.ID), payload, 0)
	pipe.SAdd(ctx, allIDsKey, item.ID)
	if previous != nil && previous.Category != "" && previous.Category != item.Category {
		pipe.SRem(ctx, categoryKey(previous.Category), item.ID)
	}
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
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Size <= 0 {
		query.Size = 20
	}
	ids, err := s.idsForQuery(ctx, query.Category)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(ids))
	for _, id := range ids {
		item, err := s.Get(ctx, id)
		if err != nil {
			continue
		}
		if !matchesQuery(item, query) {
			continue
		}
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	total := int64(len(items))
	start := (query.Page - 1) * query.Size
	if start >= len(items) {
		return &ListResult{Total: total, Items: []Item{}}, nil
	}
	end := start + query.Size
	if end > len(items) {
		end = len(items)
	}
	return &ListResult{Total: total, Items: items[start:end]}, nil
}

func (s *RedisMetaStore) ListCategories(ctx context.Context, knowledgeBaseID string) ([]string, error) {
	_ = knowledgeBaseID
	keys, err := s.client.Keys(ctx, categoryKeyPrefix+"*").Result()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, strings.TrimPrefix(k, categoryKeyPrefix))
	}
	sort.Strings(out)
	return out, nil
}

func (s *RedisMetaStore) ListTags(ctx context.Context, knowledgeBaseID string) ([]string, error) {
	ids, err := s.client.SMembers(ctx, allIDsKey).Result()
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for _, id := range ids {
		item, err := s.Get(ctx, id)
		if err != nil {
			continue
		}
		if knowledgeBaseID != "" && item.KnowledgeBaseID != knowledgeBaseID {
			continue
		}
		for _, t := range item.Tags {
			if t != "" {
				set[t] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		out = append(out, t)
	}
	sort.Strings(out)
	return out, nil
}

func (s *RedisMetaStore) FilterIDs(ctx context.Context, knowledgeBaseID string, ids []string) (map[string]struct{}, error) {
	allowed := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		item, err := s.Get(ctx, id)
		if err != nil {
			continue
		}
		if item.KnowledgeBaseID == knowledgeBaseID {
			allowed[id] = struct{}{}
		}
	}
	return allowed, nil
}

func (s *RedisMetaStore) CreatedAtByIDs(ctx context.Context, knowledgeBaseID string, ids []string) (map[string]time.Time, error) {
	createdAt := make(map[string]time.Time, len(ids))
	for _, id := range ids {
		item, err := s.Get(ctx, id)
		if err != nil {
			continue
		}
		if item.KnowledgeBaseID == knowledgeBaseID {
			createdAt[id] = item.CreatedAt
		}
	}
	return createdAt, nil
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

func (s *RedisMetaStore) idsForQuery(ctx context.Context, category string) ([]string, error) {
	if category != "" {
		return s.client.SMembers(ctx, categoryKey(category)).Result()
	}
	return s.client.SMembers(ctx, allIDsKey).Result()
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

func matchesQuery(item *Item, query ListQuery) bool {
	if item == nil {
		return false
	}
	if query.ClassifyStatus != "" && item.ClassifyStatus != query.ClassifyStatus {
		return false
	}
	if query.Tag != "" {
		found := false
		for _, tag := range item.Tags {
			if strings.EqualFold(strings.TrimSpace(tag), strings.TrimSpace(query.Tag)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if query.Q != "" {
		q := strings.ToLower(strings.TrimSpace(query.Q))
		if q == "" {
			return true
		}
		question := strings.ToLower(item.Question)
		answer := strings.ToLower(item.Answer)
		if !strings.Contains(question, q) && !strings.Contains(answer, q) {
			return false
		}
	}
	return true
}
