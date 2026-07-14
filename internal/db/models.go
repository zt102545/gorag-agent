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
	ID              string      `gorm:"primaryKey;size:36" json:"id"`
	Name            string      `gorm:"size:128;not null" json:"name"`
	TokenHash       string      `gorm:"size:255;not null;index" json:"-"`
	PlainToken      string      `gorm:"type:text" json:"caller_token,omitempty"`
	Prefix          string      `gorm:"size:24;not null" json:"prefix"`
	ChannelID       string      `gorm:"size:128;index" json:"channel_id,omitempty"`
	TenantID        string      `gorm:"size:128;index" json:"tenant_id,omitempty"`
	Enabled         bool        `gorm:"not null;default:true" json:"enabled"`
	Remark          string      `gorm:"size:255" json:"remark"`
	ExpiresAt       *time.Time  `json:"expires_at,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	LastUsedAt      *time.Time  `json:"last_used_at,omitempty"`
	AppID           string      `gorm:"size:64;index" json:"app_id,omitempty"`
	AppSecretHash   string      `gorm:"size:255" json:"-"`
	PublicAccess    bool        `gorm:"not null;default:false;index" json:"public_access"`
	AllowedOrigins  StringSlice `gorm:"type:text" json:"allowed_origins,omitempty"`
	IPRatePerMinute int         `gorm:"not null;default:60" json:"ip_rate_per_minute"`
	IPDailyQuota    int         `gorm:"not null;default:2000" json:"ip_daily_quota"`
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

type UnansweredQuestion struct {
	ID             string      `gorm:"primaryKey;size:36" json:"id"`
	Question       string      `gorm:"type:text;not null" json:"question"`
	QuestionHash   string      `gorm:"size:64;uniqueIndex;not null" json:"-"`
	SearchQuery    string      `gorm:"type:text" json:"search_query,omitempty"`
	Reason         string      `gorm:"size:64;not null;index;default:no_hit" json:"reason"`
	Status         string      `gorm:"size:32;not null;index;default:pending" json:"status"`
	Occurrences    int         `gorm:"not null;default:1" json:"occurrences"`
	Answer         string      `gorm:"type:text" json:"answer,omitempty"`
	Category       string      `gorm:"size:128" json:"category,omitempty"`
	Tags           StringSlice `gorm:"type:text" json:"tags,omitempty"`
	SessionID      string      `gorm:"size:36;index" json:"session_id,omitempty"`
	RequestID      string      `gorm:"size:128;index" json:"request_id,omitempty"`
	CallerUserID   string      `gorm:"size:128;index" json:"caller_user_id,omitempty"`
	ChannelID      string      `gorm:"size:128;index" json:"channel_id,omitempty"`
	TenantID       string      `gorm:"size:128;index" json:"tenant_id,omitempty"`
	ConvertedQAID  string      `gorm:"size:36;index" json:"converted_qa_id,omitempty"`
	Source         string      `gorm:"size:32;not null;default:chat;index" json:"source"`
	SourceRef      string      `gorm:"size:128;index" json:"source_ref,omitempty"`
	EmployeeUserID string      `gorm:"size:128;index" json:"employee_user_id,omitempty"`
	RoomID         string      `gorm:"size:128;index" json:"room_id,omitempty"`
	Context        string      `gorm:"type:text" json:"context,omitempty"`
	FirstSeenAt    time.Time   `json:"first_seen_at"`
	LastSeenAt     time.Time   `json:"last_seen_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

func (UnansweredQuestion) TableName() string { return "unanswered_questions" }

// UnansweredQuestionEvent keeps every occurrence behind an aggregated
// unanswered question. The aggregate is convenient for operations; events
// preserve the evidence needed to understand and prioritise a knowledge gap.
type UnansweredQuestionEvent struct {
	ID               string    `gorm:"primaryKey;size:36" json:"id"`
	QuestionID       string    `gorm:"size:36;not null;index" json:"question_id"`
	Question         string    `gorm:"type:text;not null" json:"question"`
	SearchQuery      string    `gorm:"type:text" json:"search_query,omitempty"`
	Reason           string    `gorm:"size:64;not null;index" json:"reason"`
	Intent           string    `gorm:"size:64;index" json:"intent,omitempty"`
	IntentConfidence float64   `json:"intent_confidence,omitempty"`
	HitCount         int       `json:"hit_count"`
	TopScore         float64   `json:"top_score,omitempty"`
	FinalAnswer      string    `gorm:"type:text" json:"final_answer,omitempty"`
	SessionID        string    `gorm:"size:36;index" json:"session_id,omitempty"`
	RequestID        string    `gorm:"size:128;index" json:"request_id,omitempty"`
	CallerUserID     string    `gorm:"size:128;index" json:"caller_user_id,omitempty"`
	ChannelID        string    `gorm:"size:128;index" json:"channel_id,omitempty"`
	TenantID         string    `gorm:"size:128;index" json:"tenant_id,omitempty"`
	Source           string    `gorm:"size:32;not null;default:chat;index" json:"source"`
	SourceRef        string    `gorm:"size:128;index" json:"source_ref,omitempty"`
	EmployeeUserID   string    `gorm:"size:128;index" json:"employee_user_id,omitempty"`
	RoomID           string    `gorm:"size:128;index" json:"room_id,omitempty"`
	Context          string    `gorm:"type:text" json:"context,omitempty"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
}

func (UnansweredQuestionEvent) TableName() string { return "unanswered_question_events" }

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
	ID                 string    `gorm:"primaryKey;size:36" json:"id"`
	Kind               string    `gorm:"size:32;not null;index" json:"kind"`
	Name               string    `gorm:"size:128;not null" json:"name"`
	Active             bool      `gorm:"not null;default:false;index" json:"active"`
	Provider           string    `gorm:"size:64;not null" json:"provider"`
	BaseURL            string    `gorm:"size:512" json:"base_url"`
	ModelName          string    `gorm:"size:128;not null" json:"model_name"`
	APIKey             string    `gorm:"size:512" json:"-"`
	Temperature        float32   `json:"temperature,omitempty"`
	MaxTokens          int       `json:"max_tokens,omitempty"`
	Dimensions         int       `json:"dimensions,omitempty"`
	UseVectorForSearch bool      `gorm:"not null;default:false" json:"use_vector_for_search"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
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

// WeComArchiveConfig is the singleton configuration for the one supported
// enterprise. Secret and private key are never returned by management APIs.
type WeComArchiveConfig struct {
	ID                  string      `gorm:"primaryKey;size:36" json:"id"`
	CorpID              string      `gorm:"size:128;not null" json:"corp_id"`
	Secret              string      `gorm:"type:text" json:"-"`
	PrivateKey          string      `gorm:"type:text" json:"-"`
	PublicKeyVersion    int         `gorm:"not null;default:1" json:"public_key_version"`
	ArchiveUserID       string      `gorm:"size:128;index" json:"archive_user_id"`
	GroupIDs            StringSlice `gorm:"type:text" json:"group_ids"`
	Enabled             bool        `gorm:"not null;default:false;index" json:"enabled"`
	PollIntervalSeconds int         `gorm:"not null;default:10" json:"poll_interval_seconds"`
	AnalyzeTime         string      `gorm:"size:5;not null;default:23:50" json:"analyze_time"`
	QAStartAt           *time.Time  `gorm:"index" json:"qa_start_at,omitempty"`
	LastSeq             uint64      `gorm:"not null;default:0" json:"last_seq"`
	LastSyncAt          *time.Time  `json:"last_sync_at,omitempty"`
	LastAnalyzeAt       *time.Time  `json:"last_analyze_at,omitempty"`
	LastAnalyzeDate     string      `gorm:"size:10;index" json:"last_analyze_date,omitempty"`
	LastError           string      `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
}

func (WeComArchiveConfig) TableName() string { return "wecom_archive_configs" }

type WeComMessage struct {
	ID         string      `gorm:"primaryKey;size:36" json:"id"`
	Seq        uint64      `gorm:"uniqueIndex;not null" json:"seq"`
	MsgID      string      `gorm:"size:128;uniqueIndex;not null" json:"msg_id"`
	Action     string      `gorm:"size:32;index" json:"action"`
	FromUserID string      `gorm:"size:128;index" json:"from_user_id"`
	ToUserIDs  StringSlice `gorm:"type:text" json:"to_user_ids"`
	RoomID     string      `gorm:"size:128;index" json:"room_id,omitempty"`
	MsgType    string      `gorm:"size:32;index" json:"msg_type"`
	Content    string      `gorm:"type:text" json:"content"`
	Direction  string      `gorm:"size:32;index" json:"direction"`
	MessageAt  time.Time   `gorm:"index" json:"message_at"`
	AnalyzedAt *time.Time  `gorm:"index" json:"analyzed_at,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
}

func (WeComMessage) TableName() string { return "wecom_messages" }

type WeComAnalysisRun struct {
	ID              string     `gorm:"primaryKey;size:36" json:"id"`
	AnalysisDate    string     `gorm:"size:10;index" json:"analysis_date"`
	Status          string     `gorm:"size:32;index" json:"status"`
	MessageCount    int        `json:"message_count"`
	QuestionCount   int        `json:"question_count"`
	KnownCount      int        `json:"known_count"`
	UnansweredCount int        `json:"unanswered_count"`
	Error           string     `gorm:"type:text" json:"error,omitempty"`
	StartedAt       time.Time  `gorm:"index" json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

func (WeComAnalysisRun) TableName() string { return "wecom_analysis_runs" }
