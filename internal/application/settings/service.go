package settingsapp

import (
	"context"

	"github.com/gorag-ai/gorag/internal/admin"
	"github.com/gorag-ai/gorag/internal/config"
	"github.com/gorag-ai/gorag/internal/db"
)

type Service struct {
	settings *admin.SettingsService
	runtime  *RuntimeRegistry
}

func NewService(settings *admin.SettingsService, runtime *RuntimeRegistry) *Service {
	return &Service{
		settings: settings,
		runtime:  runtime,
	}
}

func (s *Service) GetProvider(ctx context.Context, kind string) (*admin.ProviderView, error) {
	return s.settings.GetProvider(ctx, kind)
}

func (s *Service) UpdateProvider(ctx context.Context, kind string, req admin.UpdateProviderRequest) (*admin.ProviderView, error) {
	view, err := s.settings.UpdateProvider(ctx, kind, req)
	if err != nil {
		return nil, err
	}
	s.runtime.InvalidateProvider(kind)
	return view, nil
}

func (s *Service) ListProviderProfiles(ctx context.Context, kind string) ([]admin.ProviderView, error) {
	return s.settings.ListProviderProfiles(ctx, kind)
}

func (s *Service) CreateProviderProfile(ctx context.Context, kind string, req admin.UpdateProviderRequest) (*admin.ProviderView, error) {
	row, err := s.settings.CreateProviderProfile(ctx, kind, req)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) UpdateProviderProfile(ctx context.Context, kind, id string, req admin.UpdateProviderRequest) (*admin.ProviderView, error) {
	row, err := s.settings.UpdateProviderProfile(ctx, kind, id, req)
	if err != nil {
		return nil, err
	}
	s.runtime.InvalidateProvider(kind)
	return row, nil
}

func (s *Service) ActivateProviderProfile(ctx context.Context, kind, id string) error {
	if err := s.settings.ActivateProviderProfile(ctx, kind, id); err != nil {
		return err
	}
	s.runtime.InvalidateProvider(kind)
	return nil
}

func (s *Service) DeleteProviderProfile(ctx context.Context, kind, id string) error {
	return s.settings.DeleteProviderProfile(ctx, kind, id)
}

func (s *Service) ListPrompts(ctx context.Context) ([]db.Prompt, error) {
	return s.settings.ListPrompts(ctx)
}

func (s *Service) GetPrompt(ctx context.Context, key string) (*db.Prompt, error) {
	return s.settings.GetPrompt(ctx, key)
}

func (s *Service) UpdatePrompt(ctx context.Context, key, content string) (*db.Prompt, error) {
	row, err := s.settings.UpdatePrompt(ctx, key, content)
	if err != nil {
		return nil, err
	}
	s.runtime.InvalidatePrompt(key)
	return row, nil
}

func (s *Service) GetAgentConfig(ctx context.Context) (*admin.AgentConfigView, error) {
	return s.settings.GetAgentConfig(ctx)
}

func (s *Service) UpdateAgentConfig(ctx context.Context, req admin.UpdateAgentConfigRequest) (*admin.AgentConfigView, error) {
	row, err := s.settings.UpdateAgentConfig(ctx, req)
	if err != nil {
		return nil, err
	}
	s.runtime.InvalidateAgentConfig()
	return row, nil
}

func (s *Service) ListChatLightRules(ctx context.Context, enabledOnly bool) ([]admin.ChatLightRuleView, error) {
	return s.settings.ListChatLightRules(ctx, enabledOnly)
}

func (s *Service) CreateChatLightRule(ctx context.Context, req admin.UpsertChatLightRuleRequest) (*admin.ChatLightRuleView, error) {
	return s.settings.CreateChatLightRule(ctx, req)
}

func (s *Service) UpdateChatLightRule(ctx context.Context, id string, req admin.UpsertChatLightRuleRequest) (*admin.ChatLightRuleView, error) {
	return s.settings.UpdateChatLightRule(ctx, id, req)
}

func (s *Service) DeleteChatLightRule(ctx context.Context, id string) error {
	return s.settings.DeleteChatLightRule(ctx, id)
}

func (s *Service) GetPromptContent(ctx context.Context, key, fallback string) string {
	return s.runtime.GetPromptContent(ctx, key, fallback)
}

func (s *Service) ResolveChatConfig(ctx context.Context) config.ChatModelConfig {
	return s.runtime.ResolveChatConfig(ctx)
}

func (s *Service) ResolveEmbeddingConfig(ctx context.Context) config.EmbeddingConfig {
	return s.runtime.ResolveEmbeddingConfig(ctx)
}

func (s *Service) ResolveAgentConfig(ctx context.Context) admin.AgentConfigView {
	return s.runtime.ResolveAgentConfig(ctx)
}

func (s *Service) ChatConfigured(ctx context.Context) bool {
	cfg := s.runtime.ResolveChatConfig(ctx)
	return cfg.Provider != "" && cfg.ModelName != ""
}

func (s *Service) EmbeddingConfigured(ctx context.Context) bool {
	cfg := s.runtime.ResolveEmbeddingConfig(ctx)
	return cfg.Provider != "" && cfg.ModelName != "" && cfg.Dimensions > 0
}

func (s *Service) RuntimeRegistry() *RuntimeRegistry {
	return s.runtime
}

func (s *Service) InvalidatePrompt(key string) { s.runtime.InvalidatePrompt(key) }
