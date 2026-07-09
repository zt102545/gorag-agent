package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/yourorg/agent_customer_service/internal/config"
)

const (
	metaKeyPrefix     = "cs:session:"
	messagesKeyPrefix = "cs:messages:"
)

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

func (s *RedisStore) Create(ctx context.Context, params CreateParams) (*Meta, error) {
	now := time.Now()
	meta := &Meta{
		ID:             uuid.NewString(),
		ExternalUserID: params.ExternalUserID,
		ChannelID:      params.ChannelID,
		TenantID:       params.TenantID,
		CallerTokenID:  params.CallerTokenID,
		Meta:           params.Meta,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.saveMeta(ctx, meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func (s *RedisStore) Get(ctx context.Context, sessionID string) (*Meta, error) {
	data, err := s.client.Get(ctx, metaKey(sessionID)).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, err
	}
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *RedisStore) Delete(ctx context.Context, sessionID string) error {
	pipe := s.client.Pipeline()
	pipe.Del(ctx, metaKey(sessionID))
	pipe.Del(ctx, messagesKey(sessionID))
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisStore) GetMessages(ctx context.Context, sessionID string) ([]*schema.Message, error) {
	data, err := s.client.Get(ctx, messagesKey(sessionID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var messages []*schema.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *RedisStore) AppendMessages(ctx context.Context, sessionID string, params AppendParams, messages ...*schema.Message) error {
	if len(messages) == 0 {
		return nil
	}
	existing, err := s.GetMessages(ctx, sessionID)
	if err != nil {
		return err
	}
	if params.ClientMessageID != "" {
		meta, err := s.Get(ctx, sessionID)
		if err != nil {
			return err
		}
		if meta.Meta != nil {
			if last, ok := meta.Meta["last_client_message_id"].(string); ok && last == params.ClientMessageID {
				return nil
			}
		}
		if meta.Meta == nil {
			meta.Meta = map[string]any{}
		}
		meta.Meta["last_client_message_id"] = params.ClientMessageID
		if err := s.saveMeta(ctx, meta); err != nil {
			return err
		}
	}
	merged := append(existing, messages...)
	payload, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, messagesKey(sessionID), payload, s.ttl).Err(); err != nil {
		return err
	}
	meta, err := s.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	meta.UpdatedAt = time.Now()
	return s.saveMeta(ctx, meta)
}

func (s *RedisStore) saveMeta(ctx context.Context, meta *Meta) error {
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, metaKey(meta.ID), payload, s.ttl).Err()
}

func metaKey(sessionID string) string {
	return metaKeyPrefix + sessionID
}

func messagesKey(sessionID string) string {
	return messagesKeyPrefix + sessionID
}
