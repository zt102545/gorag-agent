package checkpoint

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/gorag-ai/gorag/internal/config"
)

const keyPrefix = "cs:checkpoint:"

type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisStore(client *redis.Client, cfg config.SessionConfig) *RedisStore {
	return &RedisStore{
		client: client,
		ttl:    cfg.TTL,
	}
}

func (s *RedisStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	data, err := s.client.Get(ctx, key(checkPointID)).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (s *RedisStore) Set(ctx context.Context, checkPointID string, value []byte) error {
	return s.client.Set(ctx, key(checkPointID), value, s.ttl).Err()
}

func (s *RedisStore) Delete(ctx context.Context, checkPointID string) error {
	return s.client.Del(ctx, key(checkPointID)).Err()
}

func key(checkPointID string) string {
	return fmt.Sprintf("%s%s", keyPrefix, checkPointID)
}

var _ interface {
	Get(ctx context.Context, checkPointID string) ([]byte, bool, error)
	Set(ctx context.Context, checkPointID string, value []byte) error
} = (*RedisStore)(nil)
