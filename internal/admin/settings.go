package admin

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/db"
)

const (
	ProviderKindChat      = "chat"
	ProviderKindEmbedding = "embedding"

	PromptAgentInstruction = "agent_instruction"
	PromptRAGAnswer        = "rag_answer"
	PromptQAClassify       = "qa_classify"
	PromptIntentRoute      = "intent_route"
)

type SettingsService struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewSettingsService(gdb *gorm.DB, cfg *config.Config) *SettingsService {
	return &SettingsService{db: gdb, cfg: cfg}
}

func (s *SettingsService) SeedDefaults(ctx context.Context) error {
	if err := s.seedProvider(ctx, ProviderKindChat, s.cfg.AI.ChatModel); err != nil {
		return err
	}
	if err := s.seedProvider(ctx, ProviderKindEmbedding, s.cfg.Embedding); err != nil {
		return err
	}
	defaults := map[string]string{
		PromptAgentInstruction: defaultAgentInstruction,
		PromptRAGAnswer:        defaultRAGPrompt,
		PromptQAClassify:       defaultClassifyPrompt,
		PromptIntentRoute:      defaultIntentRoutePrompt,
	}
	for key, content := range defaults {
		var count int64
		if err := s.db.WithContext(ctx).Model(&db.Prompt{}).Where("prompt_key = ?", key).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := s.db.WithContext(ctx).Create(&db.Prompt{
			Key:       key,
			Content:   content,
			UpdatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *SettingsService) seedProvider(ctx context.Context, kind string, src any) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&db.AIProvider{}).Where("kind = ?", kind).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	row := &db.AIProvider{
		ID:        uuid.NewString(),
		Kind:      kind,
		UpdatedAt: time.Now(),
	}
	switch v := src.(type) {
	case config.ChatModelConfig:
		row.Provider = v.Provider
		if row.Provider == "" {
			row.Provider = "openai"
		}
		row.BaseURL = v.BaseURL
		row.ModelName = v.ModelName
		row.APIKey = v.APIKey
		row.Temperature = v.Temperature
		row.MaxTokens = v.MaxTokens
	case config.EmbeddingConfig:
		row.Provider = v.Provider
		if row.Provider == "" {
			row.Provider = "openai"
		}
		row.BaseURL = v.BaseURL
		row.ModelName = v.ModelName
		row.APIKey = v.APIKey
		row.Dimensions = v.Dimensions
	}
	return s.db.WithContext(ctx).Create(row).Error
}

type ProviderView struct {
	Kind         string    `json:"kind"`
	Provider     string    `json:"provider"`
	BaseURL      string    `json:"base_url"`
	ModelName    string    `json:"model_name"`
	APIKeyMasked string    `json:"api_key_masked"`
	HasAPIKey    bool      `json:"has_api_key"`
	Temperature  float32   `json:"temperature,omitempty"`
	MaxTokens    int       `json:"max_tokens,omitempty"`
	Dimensions   int       `json:"dimensions,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func (s *SettingsService) GetProvider(ctx context.Context, kind string) (*ProviderView, error) {
	var row db.AIProvider
	if err := s.db.WithContext(ctx).Where("kind = ?", kind).First(&row).Error; err != nil {
		return nil, err
	}
	return &ProviderView{
		Kind:         row.Kind,
		Provider:     row.Provider,
		BaseURL:      row.BaseURL,
		ModelName:    row.ModelName,
		APIKeyMasked: maskKey(row.APIKey),
		HasAPIKey:    row.APIKey != "",
		Temperature:  row.Temperature,
		MaxTokens:    row.MaxTokens,
		Dimensions:   row.Dimensions,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

type UpdateProviderRequest struct {
	Provider    *string  `json:"provider"`
	BaseURL     *string  `json:"base_url"`
	ModelName   *string  `json:"model_name"`
	APIKey      *string  `json:"api_key"`
	Temperature *float32 `json:"temperature"`
	MaxTokens   *int     `json:"max_tokens"`
	Dimensions  *int     `json:"dimensions"`
}

func (s *SettingsService) UpdateProvider(ctx context.Context, kind string, req UpdateProviderRequest) (*ProviderView, error) {
	var row db.AIProvider
	if err := s.db.WithContext(ctx).Where("kind = ?", kind).First(&row).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		row = db.AIProvider{ID: uuid.NewString(), Kind: kind}
	}
	if req.Provider != nil {
		row.Provider = *req.Provider
	}
	if req.BaseURL != nil {
		row.BaseURL = *req.BaseURL
	}
	if req.ModelName != nil {
		row.ModelName = *req.ModelName
	}
	if req.APIKey != nil {
		row.APIKey = *req.APIKey
	}
	if kind == ProviderKindChat {
		if req.Temperature != nil {
			row.Temperature = *req.Temperature
		}
		if req.MaxTokens != nil {
			row.MaxTokens = *req.MaxTokens
		}
	}
	if kind == ProviderKindEmbedding {
		if req.Dimensions != nil {
			row.Dimensions = *req.Dimensions
		}
	}
	row.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	return s.GetProvider(ctx, kind)
}

func (s *SettingsService) ResolveChatConfig(ctx context.Context) config.ChatModelConfig {
	cfg := s.cfg.AI.ChatModel
	var row db.AIProvider
	if err := s.db.WithContext(ctx).Where("kind = ?", ProviderKindChat).First(&row).Error; err != nil {
		return cfg
	}
	if row.Provider != "" {
		cfg.Provider = row.Provider
	}
	cfg.BaseURL = row.BaseURL
	if row.ModelName != "" {
		cfg.ModelName = row.ModelName
	}
	if row.APIKey != "" {
		cfg.APIKey = row.APIKey
	}
	cfg.Temperature = row.Temperature
	cfg.MaxTokens = row.MaxTokens
	return cfg
}

func (s *SettingsService) ResolveEmbeddingConfig(ctx context.Context) config.EmbeddingConfig {
	cfg := s.cfg.Embedding
	var row db.AIProvider
	if err := s.db.WithContext(ctx).Where("kind = ?", ProviderKindEmbedding).First(&row).Error; err != nil {
		return cfg
	}
	if row.Provider != "" {
		cfg.Provider = row.Provider
	}
	cfg.BaseURL = row.BaseURL
	if row.ModelName != "" {
		cfg.ModelName = row.ModelName
	}
	if row.APIKey != "" {
		cfg.APIKey = row.APIKey
	}
	if row.Dimensions > 0 {
		cfg.Dimensions = row.Dimensions
	}
	return cfg
}

func (s *SettingsService) ListPrompts(ctx context.Context) ([]db.Prompt, error) {
	var rows []db.Prompt
	err := s.db.WithContext(ctx).Order("prompt_key asc").Find(&rows).Error
	return rows, err
}

func (s *SettingsService) GetPrompt(ctx context.Context, key string) (*db.Prompt, error) {
	var row db.Prompt
	if err := s.db.WithContext(ctx).Where("prompt_key = ?", key).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *SettingsService) UpdatePrompt(ctx context.Context, key, content string) (*db.Prompt, error) {
	row := &db.Prompt{Key: key, Content: content, UpdatedAt: time.Now()}
	if err := s.db.WithContext(ctx).Save(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func (s *SettingsService) GetPromptContent(ctx context.Context, key, fallback string) string {
	row, err := s.GetPrompt(ctx, key)
	if err != nil || row.Content == "" {
		return fallback
	}
	return row.Content
}

const defaultAgentInstruction = `你是智能客服助手，负责解答用户关于产品、订单、退款、配送等问题。

工作原则：
1. 优先调用 knowledge_search 检索 QA 知识库，获取相似问答对。
2. 若检索到高相似度 QA，优先使用标准 answer 回答，可适度润色但不改变语义。
3. 多个 QA 结果时，综合最相关的 answer 作答。
4. 检索无结果时，如实告知用户并建议转人工。
5. 当用户明确要求人工客服、投诉升级、或你无法解决问题时，使用 transfer_human 工具转接人工。
6. 不要编造知识库中不存在的信息。`

const defaultRAGPrompt = `你是客服助手。请根据以下知识库内容回答用户问题。若知识库无关，请如实说明。

知识库：
{{context}}

用户问题：{{query}}`

const defaultClassifyPrompt = `你是客服知识库分类助手。根据问答内容，从历史分类与标签中选择或生成合适的 category 与 tags。
优先复用历史分类/标签；若都不合适再创建简短中文新值。
只输出 JSON，不要 markdown：{"category":"...","tags":["..."]}

历史分类：{{categories}}
历史标签：{{tags}}

问题：{{question}}
答案：{{answer}}`

const defaultIntentRoutePrompt = `你是客服问题意图分析器。你要判断：这个用户问题是否需要检索 QA 知识库（knowledge_search）。

只输出 JSON：{"intent":"...","need_search":true/false,"confidence":0.0-1.0,"reason":"..."}

判定原则（举例）：
- 需要检索（need_search=true）：产品规则/售后/退款/配送/发票/会员权益/使用说明/常见问题/政策条款/对比选择 等需要“有标准答案”的问题
- 不需要检索（need_search=false）：闲聊寒暄、情绪宣泄但无具体问题、仅说谢谢/再见、纯粹让你复述、与客服无关的开放闲聊、让你编故事/写诗、仅要求转人工（此类可直接走转人工流程）

用户问题：{{query}}`
