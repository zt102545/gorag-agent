package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/gorag-ai/gorag/internal/appscope"
	"github.com/gorag-ai/gorag/internal/config"
	"github.com/gorag-ai/gorag/internal/db"
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
		MaxContextMessages: 10,
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
			Name:         "瀵掓殑闂€?,
			Priority:     intPtr(300),
			MatchType:    ChatRuleMatchKeywords,
			Pattern:      "浣犲ソ,鎮ㄥソ,hi,hello,鍦ㄥ悧,鏈変汉鍚?,
			ActionType:   ChatRuleActionReply,
			ResponseText: defaultSmalltalkReply,
		},
		{
			Name:         "鎰熻阿缁撴潫璇?,
			Priority:     intPtr(260),
			MatchType:    ChatRuleMatchKeywords,
			Pattern:      "璋㈣阿,鎰熻阿,鍏堣繖鏍?娌′簨浜?鍐嶈,bye",
			ActionType:   ChatRuleActionReply,
			ResponseText: "涓嶅姘旓紝濡傞渶缁х画鍜ㄨ璁㈠崟銆侀€€娆俱€侀厤閫佺瓑闂锛屽彲浠ラ殢鏃跺憡璇夋垜銆?,
		},
		{
			Name:       "鏄庣‘杞汉宸?,
			Priority:   intPtr(400),
			MatchType:  ChatRuleMatchKeywords,
			Pattern:    "杞汉宸?浜哄伐瀹㈡湇,鑱旂郴浜哄伐,鎴戣浜哄伐",
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
		row.Name = "Chat 閰嶇疆"
		row.Provider = v.Provider
		row.BaseURL = v.BaseURL
		row.ModelName = v.ModelName
		row.APIKey = v.APIKey
		row.Temperature = v.Temperature
		row.MaxTokens = v.MaxTokens
	case config.EmbeddingConfig:
		row.Name = "Embedding 閰嶇疆"
		row.Provider = v.Provider
		row.BaseURL = v.BaseURL
		row.ModelName = v.ModelName
		row.APIKey = v.APIKey
		row.Dimensions = v.Dimensions
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
		Name:        "杩佺Щ榛樿閰嶇疆",
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
	ID           string    `json:"id,omitempty"`
	Kind         string    `json:"kind"`
	Name         string    `json:"name,omitempty"`
	Active       bool      `json:"active"`
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
	var row db.AIProviderProfile
	if err := s.db.WithContext(ctx).Where("kind = ? AND active = ?", kind, true).Order("updated_at desc").First(&row).Error; err != nil {
		return nil, err
	}
	return &ProviderView{
		ID:           row.ID,
		Kind:         row.Kind,
		Name:         row.Name,
		Active:       row.Active,
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
	Name        *string  `json:"name"`
	Provider    *string  `json:"provider"`
	BaseURL     *string  `json:"base_url"`
	ModelName   *string  `json:"model_name"`
	APIKey      *string  `json:"api_key"`
	Temperature *float32 `json:"temperature"`
	MaxTokens   *int     `json:"max_tokens"`
	Dimensions  *int     `json:"dimensions"`
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
			ID:           row.ID,
			Kind:         row.Kind,
			Name:         row.Name,
			Active:       row.Active,
			Provider:     row.Provider,
			BaseURL:      row.BaseURL,
			ModelName:    row.ModelName,
			APIKeyMasked: maskKey(row.APIKey),
			HasAPIKey:    row.APIKey != "",
			Temperature:  row.Temperature,
			MaxTokens:    row.MaxTokens,
			Dimensions:   row.Dimensions,
			UpdatedAt:    row.UpdatedAt,
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
		ID:           row.ID,
		Kind:         row.Kind,
		Name:         row.Name,
		Active:       row.Active,
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

func (s *SettingsService) GetAgentConfig(ctx context.Context) (*AgentConfigView, error) {
	var row db.AgentConfig
	if err := s.db.WithContext(ctx).Where("config_key = ?", AgentConfigCustomerService).First(&row).Error; err != nil {
		return nil, err
	}
	maxContextMessages := row.MaxContextMessages
	if maxContextMessages <= 0 || maxContextMessages > 10 {
		maxContextMessages = 10
	}
	return &AgentConfigView{
		Key:                row.Key,
		DefaultReply:       row.DefaultReply,
		MaxContextChars:    row.MaxContextChars,
		MaxContextMessages: maxContextMessages,
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
		row.MaxContextMessages = min(*req.MaxContextMessages, 10)
	}
	if row.DefaultReply == "" {
		row.DefaultReply = defaultAgentFallbackReply
	}
	if row.MaxContextChars <= 0 {
		row.MaxContextChars = 12000
	}
	if row.MaxContextMessages <= 0 {
		row.MaxContextMessages = 10
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
	return cfg
}

func (s *SettingsService) ResolveAgentConfig(ctx context.Context) AgentConfigView {
	view, err := s.GetAgentConfig(ctx)
	if err != nil {
		return AgentConfigView{
			Key:                AgentConfigCustomerService,
			DefaultReply:       defaultAgentFallbackReply,
			MaxContextChars:    12000,
			MaxContextMessages: 10,
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
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		var item db.PromptGroupItem
		err := tx.Where("group_id = ? AND prompt_key = ?", appscope.DefaultPromptGroupID, key).First(&item).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == gorm.ErrRecordNotFound {
			item = db.PromptGroupItem{ID: uuid.NewString(), GroupID: appscope.DefaultPromptGroupID, Key: key}
		}
		item.Content, item.UpdatedAt = content, time.Now()
		return tx.Save(&item).Error
	}); err != nil {
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

func (s *SettingsService) GetPromptGroupContent(ctx context.Context, groupID, key, fallback string) string {
	var row db.PromptGroupItem
	if err := s.db.WithContext(ctx).Where("group_id = ? AND prompt_key = ?", groupID, key).First(&row).Error; err == nil && row.Content != "" {
		return row.Content
	}
	return fallback
}

const defaultAgentInstruction = `浣犳槸鏅鸿兘瀹㈡湇鍔╂墜锛岃礋璐ｈВ绛旂敤鎴峰叧浜庝骇鍝併€佽鍗曘€侀€€娆俱€侀厤閫佺瓑闂銆?
宸ヤ綔鍘熷垯锛?1. 浼樺厛璋冪敤 knowledge_search 妫€绱?QA 鐭ヨ瘑搴擄紝鑾峰彇鐩镐技闂瓟瀵广€?2. 鑻ユ绱㈠埌楂樼浉浼煎害 QA锛屼紭鍏堜娇鐢ㄦ爣鍑?answer 鍥炵瓟锛屽彲閫傚害娑﹁壊浣嗕笉鏀瑰彉璇箟銆?3. 澶氫釜 QA 缁撴灉鏃讹紝缁煎悎鏈€鐩稿叧鐨?answer 浣滅瓟銆?4. 妫€绱㈡棤缁撴灉鏃讹紝濡傚疄鍛婄煡鐢ㄦ埛骞跺缓璁浆浜哄伐銆?5. 褰撶敤鎴锋槑纭姹備汉宸ュ鏈嶃€佹姇璇夊崌绾с€佹垨浣犳棤娉曡В鍐抽棶棰樻椂锛屼娇鐢?transfer_human 宸ュ叿杞帴浜哄伐銆?6. 涓嶈缂栭€犵煡璇嗗簱涓笉瀛樺湪鐨勪俊鎭€俙

const defaultRAGPrompt = `浣犳槸瀹㈡湇鍔╂墜銆傝鏍规嵁浠ヤ笅鐭ヨ瘑搴撳唴瀹瑰洖绛旂敤鎴烽棶棰樸€傝嫢鐭ヨ瘑搴撴棤鍏筹紝璇峰瀹炶鏄庛€?
鐭ヨ瘑搴擄細
{{context}}

鐢ㄦ埛闂锛歿{query}}`

const defaultClassifyPrompt = `浣犳槸瀹㈡湇鐭ヨ瘑搴撳垎绫诲姪鎵嬨€傛牴鎹棶绛斿唴瀹癸紝浠庡巻鍙插垎绫讳笌鏍囩涓€夋嫨鎴栫敓鎴愬悎閫傜殑 category 涓?tags銆?浼樺厛澶嶇敤鍘嗗彶鍒嗙被/鏍囩锛涜嫢閮戒笉鍚堥€傚啀鍒涘缓绠€鐭腑鏂囨柊鍊笺€?鍙緭鍑?JSON锛屼笉瑕?markdown锛歿"category":"...","tags":["..."]}

鍘嗗彶鍒嗙被锛歿{categories}}
鍘嗗彶鏍囩锛歿{tags}}

闂锛歿{question}}
绛旀锛歿{answer}}`

const defaultIntentRoutePrompt = `浣犳槸瀹㈡湇闂鎰忓浘鍒嗘瀽鍣ㄣ€備綘瑕佸垽鏂細杩欎釜鐢ㄦ埛闂鏄惁闇€瑕佹绱?QA 鐭ヨ瘑搴擄紙knowledge_search锛夈€?
鍙緭鍑?JSON锛歿"intent":"...","need_search":true/false,"confidence":0.0-1.0,"reason":"..."}

鍒ゅ畾鍘熷垯锛堜妇渚嬶級锛?- 闇€瑕佹绱紙need_search=true锛夛細浜у搧瑙勫垯/鍞悗/閫€娆?閰嶉€?鍙戠エ/浼氬憳鏉冪泭/浣跨敤璇存槑/甯歌闂/鏀跨瓥鏉℃/瀵规瘮閫夋嫨 绛夐渶瑕佲€滄湁鏍囧噯绛旀鈥濈殑闂
- 涓嶉渶瑕佹绱紙need_search=false锛夛細闂茶亰瀵掓殑銆佹儏缁娉勪絾鏃犲叿浣撻棶棰樸€佷粎璇磋阿璋?鍐嶈銆佺函绮硅浣犲杩般€佷笌瀹㈡湇鏃犲叧鐨勫紑鏀鹃棽鑱娿€佽浣犵紪鏁呬簨/鍐欒瘲銆佷粎瑕佹眰杞汉宸ワ紙姝ょ被鍙洿鎺ヨ蛋杞汉宸ユ祦绋嬶級

鐢ㄦ埛闂锛歿{query}}`

const defaultAgentFallbackReply = `褰撳墠闂鏆備笉鏀寔缁х画鐢辨櫤鑳藉鏈嶅鐞嗭紝璇风◢鍚庡啀璇曟垨鑱旂郴浜哄伐瀹㈡湇銆俙

const defaultSmalltalkReply = `浣犲ソ锛屾垜鏄櫤鑳藉鏈嶅姪鎵嬨€備綘鍙互鐩存帴鍜ㄨ璁㈠崟銆侀€€娆俱€侀厤閫併€佸彂绁ㄣ€佸敭鍚庣瓑闂锛屾垜浼氫紭鍏堜粠鐭ヨ瘑搴撲腑涓轰綘鏌ユ壘鏍囧噯绛旀銆俙

const defaultHandoffReply = `宸蹭负浣犺褰曚汉宸ュ鏈嶈瘔姹傦紝璇风◢鍊欙紝鎴戜滑浼氬敖蹇负浣犺浆鎺ヤ汉宸ュ鏈嶃€俙

const defaultOutOfScopeReply = `褰撳墠鎴戜富瑕佹敮鎸佽鍗曘€侀€€娆俱€侀厤閫併€佸彂绁ㄣ€佸敭鍚庣瓑瀹㈡湇闂銆備綘鍙互琛ュ厖鏇村叿浣撶殑涓氬姟闂锛屾垨鑱旂郴浜哄伐瀹㈡湇杩涗竴姝ュ鐞嗐€俙

const defaultNoHitReply = `鎴戞殏鏃舵病鏈夊湪鐭ヨ瘑搴撲腑鎵惧埌涓庝綘闂鐩存帴瀵瑰簲鐨勬爣鍑嗙瓟妗堛€備綘鍙互琛ュ厖鏇村叿浣撶殑璁㈠崟銆侀€€娆俱€侀厤閫佹垨鍞悗淇℃伅锛屾垜鍐嶅府浣犳煡鎵撅紱濡傞渶浜哄伐瀹㈡湇锛屼篃鍙互鐩存帴璇存槑銆俙

const defaultContextLimitReply = `褰撳墠瀵硅瘽鍐呭杈冮暱锛屽缓璁綘寮€鍚竴涓柊浼氳瘽鍚庨噸鏂版弿杩伴棶棰橈紝杩欐牱鎴戣兘鏇村揩浠庣煡璇嗗簱涓负浣犳煡鎵炬爣鍑嗙瓟妗堛€俙

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
	if kind == ProviderKindEmbedding {
		if req.Dimensions != nil {
			row.Dimensions = *req.Dimensions
		}
	}
	if row.Name == "" {
		row.Name = defaultProfileName(kind)
	}
}

func defaultProfileName(kind string) string {
	if kind == ProviderKindEmbedding {
		return "鏂板缓 Embedding 閰嶇疆"
	}
	return "鏂板缓 Chat 閰嶇疆"
}

func intPtr(v int) *int {
	return &v
}
