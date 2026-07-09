package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/db"
)

const (
	ProviderKindChat           = "chat"
	ProviderKindEmbedding      = "embedding"
	AgentConfigCustomerService = "customer_service"

	PromptAgentInstruction = "agent_instruction"
	PromptRAGAnswer        = "rag_answer"
	PromptQAClassify       = "qa_classify"
	PromptIntentRoute      = "intent_route"
	PromptChatSmalltalk    = "chat_smalltalk"
	PromptChatHandoff      = "chat_handoff"
	PromptChatOutOfScope   = "chat_out_of_scope"
	PromptChatNoHit        = "chat_no_hit"
	PromptChatContextLimit = "chat_context_limit"
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
		PromptChatSmalltalk:    defaultSmalltalkReply,
		PromptChatHandoff:      defaultHandoffReply,
		PromptChatOutOfScope:   defaultOutOfScopeReply,
		PromptChatNoHit:        defaultNoHitReply,
		PromptChatContextLimit: defaultContextLimitReply,
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
	if err := s.seedAgentConfig(ctx); err != nil {
		return err
	}
	if err := s.seedLightRules(ctx); err != nil {
		return err
	}
	return nil
}

func (s *SettingsService) seedAgentConfig(ctx context.Context) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&db.AgentConfig{}).Where("config_key = ?", AgentConfigCustomerService).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return s.db.WithContext(ctx).Create(&db.AgentConfig{
		Key:                AgentConfigCustomerService,
		DefaultReply:       defaultAgentFallbackReply,
		MaxContextChars:    12000,
		MaxContextMessages: 20,
		UpdatedAt:          time.Now(),
	}).Error
}

func (s *SettingsService) seedLightRules(ctx context.Context) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&db.ChatLightRule{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	defaults := []UpsertChatLightRuleRequest{
		{
			Name:         "寒暄问候",
			Priority:     intPtr(300),
			MatchType:    ChatRuleMatchKeywords,
			Pattern:      "你好,您好,hi,hello,在吗,有人吗",
			ActionType:   ChatRuleActionReply,
			ResponseText: defaultSmalltalkReply,
		},
		{
			Name:         "感谢结束语",
			Priority:     intPtr(260),
			MatchType:    ChatRuleMatchKeywords,
			Pattern:      "谢谢,感谢,先这样,没事了,再见,bye",
			ActionType:   ChatRuleActionReply,
			ResponseText: "不客气，如需继续咨询订单、退款、配送等问题，可以随时告诉我。",
		},
		{
			Name:       "明确转人工",
			Priority:   intPtr(400),
			MatchType:  ChatRuleMatchKeywords,
			Pattern:    "转人工,人工客服,联系人工,我要人工",
			ActionType: ChatRuleActionHandoff,
		},
	}
	for _, rule := range defaults {
		if _, err := s.CreateChatLightRule(ctx, rule); err != nil {
			return err
		}
	}
	return nil
}

func (s *SettingsService) seedProvider(ctx context.Context, kind string, src any) error {
	var profileCount int64
	if err := s.db.WithContext(ctx).Model(&db.AIProviderProfile{}).Where("kind = ?", kind).Count(&profileCount).Error; err == nil && profileCount > 0 {
		return nil
	}

	row := &db.AIProviderProfile{
		ID:        uuid.NewString(),
		Kind:      kind,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	switch v := src.(type) {
	case config.ChatModelConfig:
		row.Name = "Chat 配置"
		row.Provider = v.Provider
		row.BaseURL = v.BaseURL
		row.ModelName = v.ModelName
		row.APIKey = v.APIKey
		row.Temperature = v.Temperature
		row.MaxTokens = v.MaxTokens
	case config.EmbeddingConfig:
		row.Name = "Embedding 配置"
		row.Provider = v.Provider
		row.BaseURL = v.BaseURL
		row.ModelName = v.ModelName
		row.APIKey = v.APIKey
		row.Dimensions = v.Dimensions
		row.UseVectorForSearch = v.UseVectorForSearch
	}
	if row.Provider != "" && row.ModelName != "" {
		return s.db.WithContext(ctx).Create(row).Error
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&db.AIProvider{}).Where("kind = ?", kind).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	legacyRow := &db.AIProvider{
		ID:        uuid.NewString(),
		Kind:      kind,
		UpdatedAt: time.Now(),
	}
	switch v := src.(type) {
	case config.ChatModelConfig:
		legacyRow.Provider = v.Provider
		if legacyRow.Provider == "" {
			legacyRow.Provider = "openai"
		}
		legacyRow.BaseURL = v.BaseURL
		legacyRow.ModelName = v.ModelName
		legacyRow.APIKey = v.APIKey
		legacyRow.Temperature = v.Temperature
		legacyRow.MaxTokens = v.MaxTokens
	case config.EmbeddingConfig:
		legacyRow.Provider = v.Provider
		if legacyRow.Provider == "" {
			legacyRow.Provider = "openai"
		}
		legacyRow.BaseURL = v.BaseURL
		legacyRow.ModelName = v.ModelName
		legacyRow.APIKey = v.APIKey
		legacyRow.Dimensions = v.Dimensions
	}
	if err := s.db.WithContext(ctx).Create(legacyRow).Error; err != nil {
		return err
	}
	return s.db.WithContext(ctx).Create(&db.AIProviderProfile{
		ID:          uuid.NewString(),
		Kind:        legacyRow.Kind,
		Name:        "迁移默认配置",
		Active:      true,
		Provider:    legacyRow.Provider,
		BaseURL:     legacyRow.BaseURL,
		ModelName:   legacyRow.ModelName,
		APIKey:      legacyRow.APIKey,
		Temperature: legacyRow.Temperature,
		MaxTokens:   legacyRow.MaxTokens,
		Dimensions:  legacyRow.Dimensions,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}).Error
}

type ProviderView struct {
	ID                 string    `json:"id,omitempty"`
	Kind               string    `json:"kind"`
	Name               string    `json:"name,omitempty"`
	Active             bool      `json:"active"`
	Provider           string    `json:"provider"`
	BaseURL            string    `json:"base_url"`
	ModelName          string    `json:"model_name"`
	APIKeyMasked       string    `json:"api_key_masked"`
	HasAPIKey          bool      `json:"has_api_key"`
	Temperature        float32   `json:"temperature,omitempty"`
	MaxTokens          int       `json:"max_tokens,omitempty"`
	Dimensions         int       `json:"dimensions,omitempty"`
	UseVectorForSearch bool      `json:"use_vector_for_search"`
	UpdatedAt          time.Time `json:"updated_at"`
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
	var row db.AIProviderProfile
	if err := s.db.WithContext(ctx).Where("kind = ? AND active = ?", kind, true).Order("updated_at desc").First(&row).Error; err != nil {
		return nil, err
	}
	return &ProviderView{
		ID:                 row.ID,
		Kind:               row.Kind,
		Name:               row.Name,
		Active:             row.Active,
		Provider:           row.Provider,
		BaseURL:            row.BaseURL,
		ModelName:          row.ModelName,
		APIKeyMasked:       maskKey(row.APIKey),
		HasAPIKey:          row.APIKey != "",
		Temperature:        row.Temperature,
		MaxTokens:          row.MaxTokens,
		Dimensions:         row.Dimensions,
		UseVectorForSearch: row.UseVectorForSearch,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

type UpdateProviderRequest struct {
	Name               *string  `json:"name"`
	Provider           *string  `json:"provider"`
	BaseURL            *string  `json:"base_url"`
	ModelName          *string  `json:"model_name"`
	APIKey             *string  `json:"api_key"`
	Temperature        *float32 `json:"temperature"`
	MaxTokens          *int     `json:"max_tokens"`
	Dimensions         *int     `json:"dimensions"`
	UseVectorForSearch *bool    `json:"use_vector_for_search"`
}

type AgentConfigView struct {
	Key                string    `json:"key"`
	DefaultReply       string    `json:"default_reply"`
	MaxContextChars    int       `json:"max_context_chars"`
	MaxContextMessages int       `json:"max_context_messages"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type UpdateAgentConfigRequest struct {
	DefaultReply       *string `json:"default_reply"`
	MaxContextChars    *int    `json:"max_context_chars"`
	MaxContextMessages *int    `json:"max_context_messages"`
}

func (s *SettingsService) UpdateProvider(ctx context.Context, kind string, req UpdateProviderRequest) (*ProviderView, error) {
	view, err := s.GetProvider(ctx, kind)
	if err != nil {
		return nil, err
	}
	return s.UpdateProviderProfile(ctx, kind, view.ID, req)
}

func (s *SettingsService) ListProviderProfiles(ctx context.Context, kind string) ([]ProviderView, error) {
	var rows []db.AIProviderProfile
	if err := s.db.WithContext(ctx).Where("kind = ?", kind).Order("active desc, updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ProviderView, 0, len(rows))
	for _, row := range rows {
		out = append(out, ProviderView{
			ID:                 row.ID,
			Kind:               row.Kind,
			Name:               row.Name,
			Active:             row.Active,
			Provider:           row.Provider,
			BaseURL:            row.BaseURL,
			ModelName:          row.ModelName,
			APIKeyMasked:       maskKey(row.APIKey),
			HasAPIKey:          row.APIKey != "",
			Temperature:        row.Temperature,
			MaxTokens:          row.MaxTokens,
			Dimensions:         row.Dimensions,
			UseVectorForSearch: row.UseVectorForSearch,
			UpdatedAt:          row.UpdatedAt,
		})
	}
	return out, nil
}

func (s *SettingsService) CreateProviderProfile(ctx context.Context, kind string, req UpdateProviderRequest) (*ProviderView, error) {
	now := time.Now()
	row := &db.AIProviderProfile{
		ID:        uuid.NewString(),
		Kind:      kind,
		Active:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	applyProviderProfileRequest(row, kind, req)
	if row.Provider == "" || row.ModelName == "" {
		return nil, fmt.Errorf("provider and model_name are required")
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return s.getProviderProfile(ctx, row.ID)
}

func (s *SettingsService) UpdateProviderProfile(ctx context.Context, kind, id string, req UpdateProviderRequest) (*ProviderView, error) {
	var row db.AIProviderProfile
	if err := s.db.WithContext(ctx).Where("id = ? AND kind = ?", id, kind).First(&row).Error; err != nil {
		return nil, err
	}
	applyProviderProfileRequest(&row, kind, req)
	row.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	return s.getProviderProfile(ctx, row.ID)
}

func (s *SettingsService) ActivateProviderProfile(ctx context.Context, kind, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&db.AIProviderProfile{}).Where("kind = ?", kind).Update("active", false).Error; err != nil {
			return err
		}
		return tx.Model(&db.AIProviderProfile{}).Where("id = ? AND kind = ?", id, kind).Updates(map[string]any{
			"active":     true,
			"updated_at": time.Now(),
		}).Error
	})
}

func (s *SettingsService) DeleteProviderProfile(ctx context.Context, kind, id string) error {
	var row db.AIProviderProfile
	if err := s.db.WithContext(ctx).Where("id = ? AND kind = ?", id, kind).First(&row).Error; err != nil {
		return err
	}
	if row.Active {
		return fmt.Errorf("active profile cannot be deleted")
	}
	return s.db.WithContext(ctx).Delete(&db.AIProviderProfile{}, "id = ? AND kind = ?", id, kind).Error
}

func (s *SettingsService) getProviderProfile(ctx context.Context, id string) (*ProviderView, error) {
	var row db.AIProviderProfile
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &ProviderView{
		ID:                 row.ID,
		Kind:               row.Kind,
		Name:               row.Name,
		Active:             row.Active,
		Provider:           row.Provider,
		BaseURL:            row.BaseURL,
		ModelName:          row.ModelName,
		APIKeyMasked:       maskKey(row.APIKey),
		HasAPIKey:          row.APIKey != "",
		Temperature:        row.Temperature,
		MaxTokens:          row.MaxTokens,
		Dimensions:         row.Dimensions,
		UseVectorForSearch: row.UseVectorForSearch,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

func (s *SettingsService) GetAgentConfig(ctx context.Context) (*AgentConfigView, error) {
	var row db.AgentConfig
	if err := s.db.WithContext(ctx).Where("config_key = ?", AgentConfigCustomerService).First(&row).Error; err != nil {
		return nil, err
	}
	return &AgentConfigView{
		Key:                row.Key,
		DefaultReply:       row.DefaultReply,
		MaxContextChars:    row.MaxContextChars,
		MaxContextMessages: row.MaxContextMessages,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

func (s *SettingsService) UpdateAgentConfig(ctx context.Context, req UpdateAgentConfigRequest) (*AgentConfigView, error) {
	var row db.AgentConfig
	if err := s.db.WithContext(ctx).Where("config_key = ?", AgentConfigCustomerService).First(&row).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		row = db.AgentConfig{Key: AgentConfigCustomerService}
	}
	if req.DefaultReply != nil {
		row.DefaultReply = *req.DefaultReply
	}
	if req.MaxContextChars != nil && *req.MaxContextChars > 0 {
		row.MaxContextChars = *req.MaxContextChars
	}
	if req.MaxContextMessages != nil && *req.MaxContextMessages > 0 {
		row.MaxContextMessages = *req.MaxContextMessages
	}
	if row.DefaultReply == "" {
		row.DefaultReply = defaultAgentFallbackReply
	}
	if row.MaxContextChars <= 0 {
		row.MaxContextChars = 12000
	}
	if row.MaxContextMessages <= 0 {
		row.MaxContextMessages = 20
	}
	row.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	return s.GetAgentConfig(ctx)
}

func (s *SettingsService) ResolveChatConfig(ctx context.Context) config.ChatModelConfig {
	cfg := s.cfg.AI.ChatModel
	var row db.AIProviderProfile
	if err := s.db.WithContext(ctx).Where("kind = ? AND active = ?", ProviderKindChat, true).Order("updated_at desc").First(&row).Error; err != nil {
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
	var row db.AIProviderProfile
	if err := s.db.WithContext(ctx).Where("kind = ? AND active = ?", ProviderKindEmbedding, true).Order("updated_at desc").First(&row).Error; err != nil {
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
	cfg.UseVectorForSearch = row.UseVectorForSearch
	return cfg
}

func (s *SettingsService) ResolveAgentConfig(ctx context.Context) AgentConfigView {
	view, err := s.GetAgentConfig(ctx)
	if err != nil {
		return AgentConfigView{
			Key:                AgentConfigCustomerService,
			DefaultReply:       defaultAgentFallbackReply,
			MaxContextChars:    12000,
			MaxContextMessages: 20,
		}
	}
	return *view
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

const defaultAgentFallbackReply = `当前问题暂不支持继续由智能客服处理，请稍后再试或联系人工客服。`

const defaultSmalltalkReply = `你好，我是智能客服助手。你可以直接咨询订单、退款、配送、发票、售后等问题，我会优先从知识库中为你查找标准答案。`

const defaultHandoffReply = `已为你记录人工客服诉求，请稍候，我们会尽快为你转接人工客服。`

const defaultOutOfScopeReply = `当前我主要支持订单、退款、配送、发票、售后等客服问题。你可以补充更具体的业务问题，或联系人工客服进一步处理。`

const defaultNoHitReply = `我暂时没有在知识库中找到与你问题直接对应的标准答案。你可以补充更具体的订单、退款、配送或售后信息，我再帮你查找；如需人工客服，也可以直接说明。`

const defaultContextLimitReply = `当前对话内容较长，建议你开启一个新会话后重新描述问题，这样我能更快从知识库中为你查找标准答案。`

func applyProviderProfileRequest(row *db.AIProviderProfile, kind string, req UpdateProviderRequest) {
	if req.Name != nil {
		row.Name = *req.Name
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
	} else if kind == ProviderKindEmbedding {
		if req.Dimensions != nil {
			row.Dimensions = *req.Dimensions
		}
		if req.UseVectorForSearch != nil {
			row.UseVectorForSearch = *req.UseVectorForSearch
		}
	}
	if row.Name == "" {
		row.Name = defaultProfileName(kind)
	}
}

func defaultProfileName(kind string) string {
	if kind == ProviderKindEmbedding {
		return "新建 Embedding 配置"
	}
	return "新建 Chat 配置"
}

func intPtr(v int) *int {
	return &v
}
