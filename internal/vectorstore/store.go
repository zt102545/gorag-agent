package vectorstore

import (
	"context"
	"errors"
)

var ErrTextSearchUnsupported = errors.New("text search is not supported by this vector store")

// Record is one QA document stored in the vector backend.
type Record struct {
	ID       string
	Question string
	Answer   string
	Category string
	Tags     []string
	Score    float64
	Vector   []float32
}

// Store is the pluggable vector database (RedisSearch / Milvus).
type Store interface {
	Ensure(ctx context.Context, dim int) error
	Upsert(ctx context.Context, rec Record) error
	Delete(ctx context.Context, id string) error
	SearchText(ctx context.Context, query string, topK int) ([]Record, error)
	Search(ctx context.Context, queryVector []float32, topK int) ([]Record, error)
	Close() error
}
