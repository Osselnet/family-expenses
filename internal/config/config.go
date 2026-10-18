package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Telegram TelegramConfig
	LLM      LLMConfig
	Web      WebConfig
}

type ServerConfig struct {
	Host string
	Port int
}

type DatabaseConfig struct {
	URL string
}

type TelegramConfig struct {
	Token          string
	PollTimeout    int
	ProxyURL       string
	MaxConcurrency int
}

type LLMConfig struct {
	Provider    string
	BaseURL     string
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
	Timeout     time.Duration
}

type WebConfig struct {
	Port int
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host: env("SERVER_HOST", "0.0.0.0"),
			Port: envInt("SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			URL: env("DATABASE_URL", "postgres://app:secret@localhost:5432/expenses?sslmode=disable"),
		},
		Telegram: TelegramConfig{
			Token:          os.Getenv("TELEGRAM_TOKEN"),
			PollTimeout:    envInt("TELEGRAM_POLL_TIMEOUT", 60),
			ProxyURL:       os.Getenv("TELEGRAM_PROXY"),
			MaxConcurrency: envInt("TELEGRAM_MAX_CONCURRENCY", 10),
		},
		LLM: LLMConfig{
			Provider:    env("LLM_PROVIDER", "ollama"),
			BaseURL:     env("LLM_BASE_URL", "http://localhost:11434/v1"),
			APIKey:      env("OPENAI_API_KEY", "ollama"),
			Model:       env("LLM_MODEL", "qwen3.8:27b"),
			Temperature: 0.1,
			MaxTokens:   envInt("LLM_MAX_TOKENS", 1024),
			Timeout:     envDuration("LLM_TIMEOUT", 120*time.Second),
		},
		Web: WebConfig{
			Port: envInt("WEB_PORT", 3000),
		},
	}
	return cfg, nil
}

func (c *Config) ServerAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func env(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		fmt.Printf("WARNING: cannot parse %s=%s, using %d\n", key, v, defaultVal)
	}
	return defaultVal
}

func envDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
		fmt.Printf("WARNING: cannot parse %s=%s, using %s\n", key, v, defaultVal)
	}
	return defaultVal
}
