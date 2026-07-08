package db

import (
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
	return gdb, nil
}

func AutoMigrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&AdminUser{},
		&BizAPIKey{},
		&QAItem{},
		&AIProvider{},
		&Prompt{},
		&Session{},
		&Message{},
	)
}
