package settingsapp

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/model"

	"github.com/yourorg/agent_customer_service/internal/admin"
	"github.com/yourorg/agent_customer_service/internal/config"
	modelfactory "github.com/yourorg/agent_customer_service/internal/model"
)

type RuntimeRegistry struct {
	settings *admin.SettingsService

	mu            sync.RWMutex
	chatCfg       *config.ChatModelConfig
	embeddingCfg  *config.EmbeddingConfig
	agentCfg      *admin.AgentConfigView
	promptContent map[string]string
}

func NewRuntimeRegistry(settings *admin.SettingsService) *RuntimeRegistry {
	return &RuntimeRegistry{
		settings:      settings,
		promptContent: map[string]string{},
	}
}

func (r *RuntimeRegistry) ResolveChatConfig(ctx context.Context) config.ChatModelConfig {
	r.mu.RLock()
	if r.chatCfg != nil {
		cfg := *r.chatCfg
		r.mu.RUnlock()
		return cfg
	}
	r.mu.RUnlock()

	cfg := r.settings.ResolveChatConfig(ctx)
	r.mu.Lock()
	r.chatCfg = &cfg
	r.mu.Unlock()
	return cfg
}

func (r *RuntimeRegistry) ResolveEmbeddingConfig(ctx context.Context) config.EmbeddingConfig {
	r.mu.RLock()
	if r.embeddingCfg != nil {
		cfg := *r.embeddingCfg
		r.mu.RUnlock()
		return cfg
	}
	r.mu.RUnlock()

	cfg := r.settings.ResolveEmbeddingConfig(ctx)
	r.mu.Lock()
	r.embeddingCfg = &cfg
	r.mu.Unlock()
	return cfg
}

func (r *RuntimeRegistry) GetPromptContent(ctx context.Context, key, fallback string) string {
	r.mu.RLock()
	if content, ok := r.promptContent[key]; ok {
		r.mu.RUnlock()
		if content == "" {
			return fallback
		}
		return content
	}
	r.mu.RUnlock()

	content := r.settings.GetPromptContent(ctx, key, fallback)
	r.mu.Lock()
	r.promptContent[key] = content
	r.mu.Unlock()
	return content
}

func (r *RuntimeRegistry) ResolveAgentConfig(ctx context.Context) admin.AgentConfigView {
	r.mu.RLock()
	if r.agentCfg != nil {
		cfg := *r.agentCfg
		r.mu.RUnlock()
		return cfg
	}
	r.mu.RUnlock()

	cfg := r.settings.ResolveAgentConfig(ctx)
	r.mu.Lock()
	r.agentCfg = &cfg
	r.mu.Unlock()
	return cfg
}

func (r *RuntimeRegistry) NewChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	return modelfactory.NewChatModel(ctx, r.ResolveChatConfig(ctx))
}

func (r *RuntimeRegistry) NewEmbedder(ctx context.Context) (embedding.Embedder, error) {
	return modelfactory.NewEmbedder(ctx, r.ResolveEmbeddingConfig(ctx))
}

func (r *RuntimeRegistry) InvalidateProvider(kind string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch kind {
	case admin.ProviderKindChat:
		r.chatCfg = nil
	case admin.ProviderKindEmbedding:
		r.embeddingCfg = nil
	default:
		r.chatCfg = nil
		r.embeddingCfg = nil
	}
}

func (r *RuntimeRegistry) InvalidateAgentConfig() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agentCfg = nil
}

func (r *RuntimeRegistry) InvalidatePrompt(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.promptContent, key)
}
