package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	AI      AIConfig      `mapstructure:"ai"`
	Redis   RedisConfig   `mapstructure:"redis"`
	RAG     RAGConfig     `mapstructure:"rag"`
	Session SessionConfig `mapstructure:"session"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type AIConfig struct {
	ChatModel ChatModelConfig `mapstructure:"chat_model"`
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
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type RAGConfig struct {
	IndexName   string `mapstructure:"index_name"`
	KeyPrefix   string `mapstructure:"key_prefix"`
	VectorField string `mapstructure:"vector_field"`
	TopK        int    `mapstructure:"top_k"`
	ChunkSize   int    `mapstructure:"chunk_size"`
}

type SessionConfig struct {
	TTL time.Duration `mapstructure:"ttl"`
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
	_ = v.BindEnv("ai.embedding.api_key", "OPENAI_EMBEDDING_API_KEY")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.Session.TTL == 0 {
		cfg.Session.TTL = 24 * time.Hour
	}
	if cfg.RAG.TopK == 0 {
		cfg.RAG.TopK = 3
	}
	if cfg.RAG.KeyPrefix == "" {
		cfg.RAG.KeyPrefix = "cs_qa:"
	}
	if cfg.RAG.VectorField == "" {
		cfg.RAG.VectorField = "vector_content"
	}
	if cfg.RAG.IndexName == "" {
		cfg.RAG.IndexName = "customer_service_qa"
	}
	if cfg.AI.Embedding.Dimensions == 0 {
		cfg.AI.Embedding.Dimensions = 1536
	}

	return &cfg, nil
}

func MustLoad(env string) *Config {
	cfg, err := Load(env)
	if err != nil {
		panic(err)
	}
	return cfg
}
