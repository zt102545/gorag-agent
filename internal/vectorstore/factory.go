package vectorstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/yourorg/agent_customer_service/internal/config"
)

// New creates a vector Store from vector_store.provider.
// redisClient is required when provider=redissearch; may be nil for milvus-only use.
func New(ctx context.Context, cfg config.VectorStoreConfig, redisClient *redis.Client) (Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "redissearch", "redis", "":
		if redisClient == nil {
			return nil, fmt.Errorf("redis client is required for redissearch vector store")
		}
		return NewRedisSearchStore(redisClient, cfg.RedisSearch), nil
	case "milvus":
		return NewMilvusStore(ctx, cfg.Milvus)
	default:
		return nil, fmt.Errorf("unsupported vector_store.provider: %s (use redissearch|milvus)", cfg.Provider)
	}
}
