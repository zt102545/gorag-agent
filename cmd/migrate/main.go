package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"github.com/yourorg/agent_customer_service/internal/config"
	"github.com/yourorg/agent_customer_service/internal/db"
	"github.com/yourorg/agent_customer_service/internal/dbmigration"
)

func main() {
	sourcePath := flag.String("source", "", "source SQLite file (default: DATABASE_SQLITE_PATH/config value)")
	mysqlDSN := flag.String("mysql-dsn", "", "target MySQL DSN (default: DATABASE_MYSQL_DSN/config value)")
	batchSize := flag.Int("batch-size", dbmigration.DefaultBatchSize, "rows copied per batch")
	allowExisting := flag.Bool("allow-existing", false, "upsert into a target that already contains business data")
	flag.Parse()

	_ = godotenv.Load()
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}
	cfg, err := config.Load(env)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if strings.TrimSpace(*sourcePath) == "" {
		*sourcePath = cfg.Database.SQLite.Path
	}
	if strings.TrimSpace(*mysqlDSN) == "" {
		*mysqlDSN = cfg.Database.MySQL.DSN
	}
	if strings.TrimSpace(*mysqlDSN) == "" {
		log.Fatal("target MySQL DSN is required: set -mysql-dsn or DATABASE_MYSQL_DSN")
	}
	info, err := os.Stat(*sourcePath)
	if err != nil {
		log.Fatalf("inspect source SQLite file: %v", err)
	}
	if info.IsDir() {
		log.Fatalf("source SQLite path is a directory: %s", *sourcePath)
	}

	sourceCfg := cfg.Database
	sourceCfg.Driver = "sqlite"
	sourceCfg.SQLite.Path = *sourcePath
	sourceCfg.MaxOpenConns = 1
	sourceCfg.MaxIdleConns = 1
	targetCfg := cfg.Database
	targetCfg.Driver = "mysql"
	targetCfg.MySQL.DSN = *mysqlDSN

	source, err := db.Open(sourceCfg)
	if err != nil {
		log.Fatalf("open source SQLite: %v", err)
	}
	defer closeDatabase(source)
	target, err := db.Open(targetCfg)
	if err != nil {
		log.Fatalf("open target MySQL: %v", err)
	}
	defer closeDatabase(target)

	result, err := dbmigration.New(source, target).Run(context.Background(), dbmigration.Options{
		BatchSize:     *batchSize,
		AllowExisting: *allowExisting,
	})
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}
	for _, table := range result.Tables {
		if table.Skipped {
			log.Printf("table %-28s skipped (not present in source)", table.Table)
			continue
		}
		log.Printf("table %-28s source=%d copied=%d target=%d sanitized_strings=%d", table.Table, table.SourceRows, table.CopiedRows, table.TargetRows, table.SanitizedStrings)
	}
	log.Printf("migration completed: copied %d rows", result.CopiedRows())
}

func closeDatabase(gdb interface{ DB() (*sql.DB, error) }) {
	sqlDB, err := gdb.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
