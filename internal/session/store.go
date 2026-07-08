package session

import (
	"context"
	"time"

	"github.com/cloudwego/eino/schema"
)

type Meta struct {
	ID             string         `json:"id"`
	Title          string         `json:"title,omitempty"`
	ExternalUserID string         `json:"external_user_id,omitempty"`
	ChannelID      string         `json:"channel_id,omitempty"`
	TenantID       string         `json:"tenant_id,omitempty"`
	CallerTokenID  string         `json:"caller_token_id,omitempty"`
	APIKeyID       string         `json:"api_key_id,omitempty"`
	Status         string         `json:"status,omitempty"`
	Meta           map[string]any `json:"meta,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	ClosedAt       *time.Time     `json:"closed_at,omitempty"`
}

type CreateParams struct {
	ExternalUserID string         `json:"external_user_id,omitempty"`
	ChannelID      string         `json:"channel_id,omitempty"`
	TenantID       string         `json:"tenant_id,omitempty"`
	CallerTokenID  string         `json:"caller_token_id,omitempty"`
	APIKeyID       string         `json:"api_key_id,omitempty"`
	Meta           map[string]any `json:"meta,omitempty"`
}

type ListQuery struct {
	SessionID      string
	Q              string
	Status         string
	ExternalUserID string
	ChannelID      string
	TenantID       string
	CallerTokenID  string
	APIKeyID       string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	Page           int
	Size           int
}

type MessageView struct {
	ID              string         `json:"id"`
	Role            string         `json:"role"`
	Content         string         `json:"content"`
	RequestID       string         `json:"request_id,omitempty"`
	ClientMessageID string         `json:"client_message_id,omitempty"`
	Extra           map[string]any `json:"extra,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

type SessionListItem struct {
	Meta
	MessageCount       int64      `json:"message_count"`
	LastMessageRole    string     `json:"last_message_role,omitempty"`
	LastMessagePreview string     `json:"last_message_preview,omitempty"`
	LastMessageAt      *time.Time `json:"last_message_at,omitempty"`
}

type AppendParams struct {
	RequestID       string
	ClientMessageID string
}

type Store interface {
	Create(ctx context.Context, params CreateParams) (*Meta, error)
	Get(ctx context.Context, sessionID string) (*Meta, error)
	Delete(ctx context.Context, sessionID string) error
	GetMessages(ctx context.Context, sessionID string) ([]*schema.Message, error)
	AppendMessages(ctx context.Context, sessionID string, params AppendParams, messages ...*schema.Message) error
}
