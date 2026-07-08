package qa

import "context"

type MetaStore interface {
	Save(ctx context.Context, item *Item) error
	Get(ctx context.Context, id string) (*Item, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, query ListQuery) (*ListResult, error)
	AddToCategory(ctx context.Context, category, id string) error
	RemoveFromCategory(ctx context.Context, category, id string) error
	ListByCategory(ctx context.Context, category string, page, size int) (*ListResult, error)
}
