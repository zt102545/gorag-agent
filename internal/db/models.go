package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	return string(b), err
}

func (s *StringSlice) Scan(value any) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported type for StringSlice: %T", value)
	}
	if len(raw) == 0 {
		*s = StringSlice{}
		return nil
	}
	return json.Unmarshal(raw, s)
}

type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}

func (m *JSONMap) Scan(value any) error {
	if value == nil {
		*m = JSONMap{}
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("unsupported type for JSONMap: %T", value)
	}
	if len(raw) == 0 {
		*m = JSONMap{}
		return nil
	}
	return json.Unmarshal(raw, m)
}

type AdminUser struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Role         string    `gorm:"size:32;not null;index" json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (AdminUser) TableName() string { return "admin_users" }

type BizAPIKey struct {
	ID         string     `gorm:"primaryKey;size:36" json:"id"`
	Name       string     `gorm:"size:128;not null" json:"name"`
	KeyHash    string     `gorm:"size:255;not null;index" json:"-"`
	Prefix     string     `gorm:"size:16;not null" json:"prefix"`
	Enabled    bool       `gorm:"not null;default:true" json:"enabled"`
	Remark     string     `gorm:"size:255" json:"remark"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func (BizAPIKey) TableName() string { return "biz_api_keys" }

type QAItem struct {
	ID             string      `gorm:"primaryKey;size:36" json:"id"`
	Question       string      `gorm:"type:text;not null" json:"question"`
	Answer         string      `gorm:"type:text;not null" json:"answer"`
	Category       string      `gorm:"size:128;index" json:"category"`
	Tags           StringSlice `gorm:"type:text" json:"tags"`
	ClassifyStatus string      `gorm:"size:32;not null;index;default:none" json:"classify_status"`
	ClassifyError  string      `gorm:"type:text" json:"classify_error,omitempty"`
	ClassifiedAt   *time.Time  `json:"classified_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

func (QAItem) TableName() string { return "qa_items" }

type AIProvider struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	Kind        string    `gorm:"uniqueIndex;size:32;not null" json:"kind"` // chat | embedding
	Provider    string    `gorm:"size:64;not null" json:"provider"`
	BaseURL     string    `gorm:"size:512" json:"base_url"`
	ModelName   string    `gorm:"size:128;not null" json:"model_name"`
	APIKey      string    `gorm:"size:512" json:"-"`
	Temperature float32   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Dimensions  int       `json:"dimensions,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (AIProvider) TableName() string { return "ai_providers" }

type Prompt struct {
	Key       string    `gorm:"column:prompt_key;primaryKey;size:64" json:"key"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Prompt) TableName() string { return "prompts" }

type Session struct {
	ID             string     `gorm:"primaryKey;size:36" json:"id"`
	ExternalUserID string     `gorm:"size:128;index" json:"external_user_id,omitempty"`
	Status         string     `gorm:"size:32;not null;index;default:active" json:"status"`
	Meta           JSONMap    `gorm:"type:text" json:"meta,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
}

func (Session) TableName() string { return "sessions" }

type Message struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	SessionID string    `gorm:"size:36;not null;index" json:"session_id"`
	Role      string    `gorm:"size:32;not null" json:"role"`
	Content   string    `gorm:"type:text" json:"content"`
	Extra     string    `gorm:"type:text" json:"extra,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (Message) TableName() string { return "messages" }
