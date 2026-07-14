package qa

import (
	"context"
	"time"
)

type MetaStore interface {
	Save(ctx context.Context, item *Item) error
	Get(ctx context.Context, id string) (*Item, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, query ListQuery) (*ListResult, error)
	AddToCategory(ctx context.Context, category, id string) error
	RemoveFromCategory(ctx context.Context, category, id string) error
	ListByCategory(ctx context.Context, category string, page, size int) (*ListResult, error)
	ListCategories(ctx context.Context, knowledgeBaseID string) ([]string, error)
	ListTags(ctx context.Context, knowledgeBaseID string) ([]string, error)
	FilterIDs(ctx context.Context, knowledgeBaseID string, ids []string) (map[string]struct{}, error)
	CreatedAtByIDs(ctx context.Context, knowledgeBaseID string, ids []string) (map[string]time.Time, error)
}
