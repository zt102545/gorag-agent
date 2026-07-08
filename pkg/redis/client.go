package redis

import (
	"github.com/redis/go-redis/v9"

	"github.com/yourorg/agent_customer_service/internal/config"
)

func NewClient(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:          cfg.Addr,
		Password:      cfg.Password,
		DB:            cfg.DB,
		Protocol:      2,
		UnstableResp3: true,
	})
}

func NewIndexerClient(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}
