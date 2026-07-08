package session

import (
	"context"
	"time"

	"github.com/cloudwego/eino/schema"
)

type Meta struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store interface {
	Create(ctx context.Context) (*Meta, error)
	Get(ctx context.Context, sessionID string) (*Meta, error)
	Delete(ctx context.Context, sessionID string) error
	GetMessages(ctx context.Context, sessionID string) ([]*schema.Message, error)
	AppendMessages(ctx context.Context, sessionID string, messages ...*schema.Message) error
}
