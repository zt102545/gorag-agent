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

type CallerToken struct {
	ID         string     `gorm:"primaryKey;size:36" json:"id"`
	Name       string     `gorm:"size:128;not null" json:"name"`
	TokenHash  string     `gorm:"size:255;not null;index" json:"-"`
	Prefix     string     `gorm:"size:24;not null" json:"prefix"`
	ChannelID  string     `gorm:"size:128;index" json:"channel_id,omitempty"`
	TenantID   string     `gorm:"size:128;index" json:"tenant_id,omitempty"`
	Enabled    bool       `gorm:"not null;default:true" json:"enabled"`
	Remark     string     `gorm:"size:255" json:"remark"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func (CallerToken) TableName() string { return "caller_tokens" }

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

type AIProviderProfile struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	Kind        string    `gorm:"size:32;not null;index" json:"kind"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Active      bool      `gorm:"not null;default:false;index" json:"active"`
	Provider    string    `gorm:"size:64;not null" json:"provider"`
	BaseURL     string    `gorm:"size:512" json:"base_url"`
	ModelName   string    `gorm:"size:128;not null" json:"model_name"`
	APIKey      string    `gorm:"size:512" json:"-"`
	Temperature float32   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Dimensions  int       `json:"dimensions,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (AIProviderProfile) TableName() string { return "ai_provider_profiles" }

type Prompt struct {
	Key       string    `gorm:"column:prompt_key;primaryKey;size:64" json:"key"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Prompt) TableName() string { return "prompts" }

type AgentConfig struct {
	Key                string    `gorm:"column:config_key;primaryKey;size:64" json:"key"`
	DefaultReply       string    `gorm:"type:text;not null" json:"default_reply"`
	MaxContextChars    int       `gorm:"not null;default:12000" json:"max_context_chars"`
	MaxContextMessages int       `gorm:"not null;default:20" json:"max_context_messages"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (AgentConfig) TableName() string { return "agent_configs" }

type ChatLightRule struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	Name         string    `gorm:"size:128;not null" json:"name"`
	Enabled      bool      `gorm:"not null;default:true;index" json:"enabled"`
	Priority     int       `gorm:"not null;default:100;index" json:"priority"`
	MatchType    string    `gorm:"size:32;not null;index" json:"match_type"`
	Pattern      string    `gorm:"type:text;not null" json:"pattern"`
	ActionType   string    `gorm:"size:32;not null;index" json:"action_type"`
	ResponseText string    `gorm:"type:text" json:"response_text,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (ChatLightRule) TableName() string { return "chat_light_rules" }

type AIErrorLog struct {
	ID         string     `gorm:"primaryKey;size:36" json:"id"`
	Source     string     `gorm:"size:64;index" json:"source"`
	Stage      string     `gorm:"size:128;index" json:"stage"`
	Message    string     `gorm:"type:text;not null" json:"message"`
	RequestID  string     `gorm:"size:64;index" json:"request_id,omitempty"`
	SessionID  string     `gorm:"size:36;index" json:"session_id,omitempty"`
	Details    string     `gorm:"type:text" json:"details,omitempty"`
	Resolved   bool       `gorm:"not null;default:false;index" json:"resolved"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

func (AIErrorLog) TableName() string { return "ai_error_logs" }

type Session struct {
	ID             string     `gorm:"primaryKey;size:36" json:"id"`
	Title          string     `gorm:"size:255;index" json:"title,omitempty"`
	ExternalUserID string     `gorm:"size:128;index" json:"external_user_id,omitempty"`
	ChannelID      string     `gorm:"size:128;index" json:"channel_id,omitempty"`
	TenantID       string     `gorm:"size:128;index" json:"tenant_id,omitempty"`
	CallerTokenID  string     `gorm:"size:36;index" json:"caller_token_id,omitempty"`
	Status         string     `gorm:"size:32;not null;index;default:active" json:"status"`
	Meta           JSONMap    `gorm:"type:text" json:"meta,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
}

func (Session) TableName() string { return "sessions" }

type Message struct {
	ID              string    `gorm:"primaryKey;size:36" json:"id"`
	SessionID       string    `gorm:"size:36;not null;index" json:"session_id"`
	Role            string    `gorm:"size:32;not null" json:"role"`
	RequestID       string    `gorm:"size:64;index" json:"request_id,omitempty"`
	ClientMessageID string    `gorm:"size:128;index" json:"client_message_id,omitempty"`
	Content         string    `gorm:"type:text" json:"content"`
	Extra           string    `gorm:"type:text" json:"extra,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

func (Message) TableName() string { return "messages" }
