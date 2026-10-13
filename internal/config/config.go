package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Host        string
	BaseURL     string
	DatabaseURL string
	RedisURL    string
}

type Option func(*Config)

func WithPort(port string) Option {
	return func(c *Config) {
		if port != "" {
			c.Port = port
		}
	}
}

func WithHost(host string) Option {
	return func(c *Config) {
		if host != "" {
			c.Host = host
		}
	}
}

func WithBaseURL(baseURL string) Option {
	return func(c *Config) {
		if baseURL != "" {
			c.BaseURL = baseURL
		}
	}
}

func WithDatabaseURL(dbURL string) Option {
	return func(c *Config) {
		if dbURL != "" {
			c.DatabaseURL = dbURL
		}
	}
}

func WithRedisURL(redisURL string) Option {
	return func(c *Config) {
		if redisURL != "" {
			c.RedisURL = redisURL
		}
	}
}

func New(opts ...Option) *Config {
	_ = godotenv.Load()

	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		Host:        getEnv("HOST", "0.0.0.0"),
		BaseURL:     getEnv("BASE_URL", ""),
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://aks:aks7536@localhost:5432/articles"),
		RedisURL:    getEnv("REDIS_URL", "redis://127.0.0.1:6379"),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:" + cfg.Port
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
