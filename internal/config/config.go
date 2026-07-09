package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	AI          AIConfig          `mapstructure:"ai"`
	Embedding   EmbeddingConfig   `mapstructure:"embedding"`
	Redis       RedisConfig       `mapstructure:"redis"`
	VectorStore VectorStoreConfig `mapstructure:"vector_store"`
	Session     SessionConfig     `mapstructure:"session"`
	Database    DatabaseConfig    `mapstructure:"database"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	Admin       AdminConfig       `mapstructure:"admin"`
	Classify    ClassifyConfig    `mapstructure:"classify"`

	// Legacy rag section for one-shot migration.
	LegacyRAG legacyRAGConfig `mapstructure:"rag"`
}

type legacyRAGConfig struct {
	IndexName   string `mapstructure:"index_name"`
	KeyPrefix   string `mapstructure:"key_prefix"`
	VectorField string `mapstructure:"vector_field"`
	TopK        int    `mapstructure:"top_k"`
	ChunkSize   int    `mapstructure:"chunk_size"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type AIConfig struct {
	ChatModel ChatModelConfig `mapstructure:"chat_model"`
	// Deprecated: prefer top-level embedding. Kept for legacy YAML.
	Embedding EmbeddingConfig `mapstructure:"embedding"`
}

type ChatModelConfig struct {
	Provider    string  `mapstructure:"provider"`
	BaseURL     string  `mapstructure:"base_url"`
	ModelName   string  `mapstructure:"model_name"`
	APIKey      string  `mapstructure:"api_key"`
	Temperature float32 `mapstructure:"temperature"`
	MaxTokens   int     `mapstructure:"max_tokens"`
}

type EmbeddingConfig struct {
	Provider   string `mapstructure:"provider"`
	BaseURL    string `mapstructure:"base_url"`
	ModelName  string `mapstructure:"model_name"`
	APIKey     string `mapstructure:"api_key"`
	Dimensions int    `mapstructure:"dimensions"`
}

type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolTimeout  time.Duration `mapstructure:"pool_timeout"`
}

type VectorStoreConfig struct {
	Provider    string                 `mapstructure:"provider"`
	TopK        int                    `mapstructure:"top_k"`
	RedisSearch RedisSearchStoreConfig `mapstructure:"redissearch"`
	Milvus      MilvusStoreConfig      `mapstructure:"milvus"`
}

type RedisSearchStoreConfig struct {
	IndexName   string `mapstructure:"index_name"`
	KeyPrefix   string `mapstructure:"key_prefix"`
	VectorField string `mapstructure:"vector_field"`
}

type MilvusStoreConfig struct {
	Address    string `mapstructure:"address"`
	URI        string `mapstructure:"uri"`
	Token      string `mapstructure:"token"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	Database   string `mapstructure:"database"`
	Collection string `mapstructure:"collection"`
	MetricType string `mapstructure:"metric_type"`
}

type SessionConfig struct {
	TTL time.Duration `mapstructure:"ttl"`
}

type DatabaseConfig struct {
	Driver          string        `mapstructure:"driver"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	SQLite          SQLiteConfig  `mapstructure:"sqlite"`
	MySQL           MySQLConfig   `mapstructure:"mysql"`
}

type SQLiteConfig struct {
	Path string `mapstructure:"path"`
}

type MySQLConfig struct {
	DSN string `mapstructure:"dsn"`
}

type JWTConfig struct {
	Secret string        `mapstructure:"secret"`
	Expire time.Duration `mapstructure:"expire"`
}

type AdminConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type ClassifyConfig struct {
	Workers int `mapstructure:"workers"`
}

func Load(env string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read base config: %w", err)
	}

	if env != "" {
		v.SetConfigName("config-" + env)
		if err := v.MergeInConfig(); err != nil {
			return nil, fmt.Errorf("merge env config %q: %w", env, err)
		}
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	_ = v.BindEnv("ai.chat_model.api_key", "OPENAI_API_KEY")
	_ = v.BindEnv("ai.chat_model.base_url", "AI_CHAT_MODEL_BASE_URL")
	_ = v.BindEnv("ai.chat_model.model_name", "AI_CHAT_MODEL_MODEL_NAME")
	_ = v.BindEnv("embedding.api_key", "EMBEDDING_API_KEY")
	_ = v.BindEnv("embedding.provider", "EMBEDDING_PROVIDER")
	_ = v.BindEnv("embedding.base_url", "EMBEDDING_BASE_URL")
	_ = v.BindEnv("embedding.model_name", "EMBEDDING_MODEL_NAME")
	_ = v.BindEnv("embedding.dimensions", "EMBEDDING_DIMENSIONS")
	_ = v.BindEnv("database.driver", "DATABASE_DRIVER")
	_ = v.BindEnv("database.mysql.dsn", "DATABASE_MYSQL_DSN")
	_ = v.BindEnv("database.sqlite.path", "DATABASE_SQLITE_PATH")
	_ = v.BindEnv("server.host", "SERVER_HOST")
	_ = v.BindEnv("server.port", "SERVER_PORT")
	_ = v.BindEnv("jwt.secret", "JWT_SECRET")
	_ = v.BindEnv("admin.username", "ADMIN_USERNAME")
	_ = v.BindEnv("admin.password", "ADMIN_PASSWORD")
	_ = v.BindEnv("vector_store.provider", "VECTOR_STORE_PROVIDER")
	_ = v.BindEnv("vector_store.milvus.address", "MILVUS_ADDRESS")
	_ = v.BindEnv("vector_store.milvus.uri", "MILVUS_URI")
	_ = v.BindEnv("vector_store.milvus.token", "MILVUS_TOKEN")
	_ = v.BindEnv("redis.addr", "REDIS_ADDR")
	_ = v.BindEnv("redis.password", "REDIS_PASSWORD")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	applyLegacyMapping(&cfg)
	applyDefaults(&cfg)

	return &cfg, nil
}

func applyLegacyMapping(cfg *Config) {
	if cfg.Embedding.ModelName == "" && cfg.Embedding.Provider == "" && cfg.Embedding.APIKey == "" {
		if cfg.AI.Embedding.ModelName != "" || cfg.AI.Embedding.Provider != "" || cfg.AI.Embedding.APIKey != "" {
			log.Printf("[config] migrating ai.embedding → top-level embedding")
			cfg.Embedding = cfg.AI.Embedding
		}
	}
	if cfg.Embedding.APIKey == "" {
		if key := os.Getenv("OPENAI_EMBEDDING_API_KEY"); key != "" {
			cfg.Embedding.APIKey = key
		}
	}

	if cfg.VectorStore.Provider == "" && (cfg.LegacyRAG.IndexName != "" || cfg.LegacyRAG.KeyPrefix != "" || cfg.LegacyRAG.TopK > 0) {
		log.Printf("[config] migrating rag → vector_store (provider=redissearch)")
		cfg.VectorStore.Provider = "redissearch"
		if cfg.VectorStore.TopK == 0 {
			cfg.VectorStore.TopK = cfg.LegacyRAG.TopK
		}
		if cfg.VectorStore.RedisSearch.IndexName == "" {
			cfg.VectorStore.RedisSearch.IndexName = cfg.LegacyRAG.IndexName
		}
		if cfg.VectorStore.RedisSearch.KeyPrefix == "" {
			cfg.VectorStore.RedisSearch.KeyPrefix = cfg.LegacyRAG.KeyPrefix
		}
		if cfg.VectorStore.RedisSearch.VectorField == "" {
			cfg.VectorStore.RedisSearch.VectorField = cfg.LegacyRAG.VectorField
		}
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	if cfg.Session.TTL == 0 {
		cfg.Session.TTL = 24 * time.Hour
	}
	if cfg.VectorStore.Provider == "" {
		cfg.VectorStore.Provider = "redissearch"
	}
	if cfg.VectorStore.TopK == 0 {
		cfg.VectorStore.TopK = 3
	}
	if cfg.VectorStore.RedisSearch.IndexName == "" {
		cfg.VectorStore.RedisSearch.IndexName = "customer_service_qa"
	}
	if cfg.VectorStore.RedisSearch.KeyPrefix == "" {
		cfg.VectorStore.RedisSearch.KeyPrefix = "cs_qa:"
	}
	if cfg.VectorStore.RedisSearch.VectorField == "" {
		cfg.VectorStore.RedisSearch.VectorField = "vector_content"
	}
	if cfg.VectorStore.Milvus.Address == "" && cfg.VectorStore.Milvus.URI == "" {
		cfg.VectorStore.Milvus.Address = "localhost:19530"
	}
	if cfg.VectorStore.Milvus.Database == "" {
		cfg.VectorStore.Milvus.Database = "default"
	}
	if cfg.VectorStore.Milvus.Collection == "" {
		cfg.VectorStore.Milvus.Collection = "customer_service_qa"
	}
	if cfg.VectorStore.Milvus.MetricType == "" {
		cfg.VectorStore.Milvus.MetricType = "COSINE"
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "sqlite"
	}
	if cfg.Database.MaxOpenConns <= 0 {
		cfg.Database.MaxOpenConns = 50
	}
	if cfg.Database.MaxIdleConns <= 0 {
		cfg.Database.MaxIdleConns = 10
	}
	if cfg.Database.ConnMaxLifetime <= 0 {
		cfg.Database.ConnMaxLifetime = 30 * time.Minute
	}
	if cfg.Database.SQLite.Path == "" {
		cfg.Database.SQLite.Path = "./data/app.db"
	}
	if cfg.JWT.Secret == "" {
		cfg.JWT.Secret = "dev-change-me-jwt-secret"
	}
	if cfg.JWT.Expire == 0 {
		cfg.JWT.Expire = 24 * time.Hour
	}
	if cfg.Admin.Username == "" {
		cfg.Admin.Username = "admin"
	}
	if cfg.Admin.Password == "" {
		cfg.Admin.Password = "admin123"
	}
	if cfg.Classify.Workers <= 0 {
		cfg.Classify.Workers = 1
	}
	if cfg.Redis.PoolSize <= 0 {
		cfg.Redis.PoolSize = 50
	}
	if cfg.Redis.MinIdleConns < 0 {
		cfg.Redis.MinIdleConns = 0
	}
	if cfg.Redis.DialTimeout <= 0 {
		cfg.Redis.DialTimeout = 3 * time.Second
	}
	if cfg.Redis.ReadTimeout <= 0 {
		cfg.Redis.ReadTimeout = 2 * time.Second
	}
	if cfg.Redis.WriteTimeout <= 0 {
		cfg.Redis.WriteTimeout = 2 * time.Second
	}
	if cfg.Redis.PoolTimeout <= 0 {
		cfg.Redis.PoolTimeout = 4 * time.Second
	}
}

func MustLoad(env string) *Config {
	cfg, err := Load(env)
	if err != nil {
		panic(err)
	}
	return cfg
}
