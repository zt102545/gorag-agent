package bootstrap

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/gorag-ai/gorag/internal/admin"
	settingsapp "github.com/gorag-ai/gorag/internal/application/settings"
	"github.com/gorag-ai/gorag/internal/config"
	"github.com/gorag-ai/gorag/internal/db"
	redispkg "github.com/gorag-ai/gorag/pkg/redis"
)

func newDatabase(cfg *config.Config) (*gorm.DB, error) {
	gdb, err := db.Open(cfg.Database)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(gdb); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return gdb, nil
}

func newRedisClient(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	client := redispkg.NewClient(cfg.Redis)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return client, nil
}

func newSettingsServices(gdb *gorm.DB, cfg *config.Config) (*admin.SettingsService, *settingsapp.RuntimeRegistry, *settingsapp.Service) {
	settingsSvc := admin.NewSettingsService(gdb, cfg)
	runtime := settingsapp.NewRuntimeRegistry(settingsSvc)
	return settingsSvc, runtime, settingsapp.NewService(settingsSvc, runtime)
}
