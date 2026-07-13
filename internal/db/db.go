package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/yourorg/agent_customer_service/internal/config"
)

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		driver = "sqlite"
	}

	var dialector gorm.Dialector
	switch driver {
	case "sqlite":
		path := cfg.SQLite.Path
		if path == "" {
			path = "./data/app.db"
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite dir: %w", err)
		}
		dialector = sqlite.Open(path)
	case "mysql":
		dsn := cfg.MySQL.DSN
		if dsn == "" {
			return nil, fmt.Errorf("mysql dsn is required when driver=mysql")
		}
		dialector = mysql.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}

	gdb, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open database (%s): %w", driver, err)
	}
	if err := configurePool(gdb, cfg); err != nil {
		return nil, err
	}
	return gdb, nil
}

func configurePool(gdb *gorm.DB, cfg config.DatabaseConfig) error {
	sqlDB, err := gdb.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	applyPoolDefaults(sqlDB, cfg)
	return nil
}

func applyPoolDefaults(sqlDB *sql.DB, cfg config.DatabaseConfig) {
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
}

func AutoMigrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&AdminUser{},
		&CallerToken{},
		&QAItem{},
		&UnansweredQuestion{},
		&AIProvider{},
		&AIProviderProfile{},
		&Prompt{},
		&AgentConfig{},
		&ChatLightRule{},
		&AIErrorLog{},
		&Session{},
		&Message{},
	)
}
