package settingsapp

import (
	"context"

	"github.com/yourorg/agent_customer_service/internal/admin"
	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/db"
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

func (s *Service) GetPromptContent(ctx context.Context, key, fallback string) string {
	return s.runtime.GetPromptContent(ctx, key, fallback)
}

func (s *Service) ResolveChatConfig(ctx context.Context) config.ChatModelConfig {
	return s.runtime.ResolveChatConfig(ctx)
}

func (s *Service) ResolveEmbeddingConfig(ctx context.Context) config.EmbeddingConfig {
	return s.runtime.ResolveEmbeddingConfig(ctx)
}

func (s *Service) RuntimeRegistry() *RuntimeRegistry {
	return s.runtime
}
