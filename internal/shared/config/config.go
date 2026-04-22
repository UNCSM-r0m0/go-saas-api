package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	// Server
	Port    string `mapstructure:"PORT"`
	Env     string `mapstructure:"ENV"`
	LogLevel string `mapstructure:"LOG_LEVEL"`

	// Database
	DatabaseURL string `mapstructure:"DATABASE_URL"`

	// Redis
	RedisURL string `mapstructure:"REDIS_URL"`

	// NATS
	NATSURL string `mapstructure:"NATS_URL"`

	// JWT
	JWTSecret     string        `mapstructure:"JWT_SECRET"`
	JWTExpiration time.Duration `mapstructure:"JWT_EXPIRATION"`

	// OAuth
	GoogleClientID     string `mapstructure:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `mapstructure:"GOOGLE_CLIENT_SECRET"`
	GitHubClientID     string `mapstructure:"GITHUB_CLIENT_ID"`
	GitHubClientSecret string `mapstructure:"GITHUB_CLIENT_SECRET"`

	// Stripe
	StripeSecretKey      string `mapstructure:"STRIPE_SECRET_KEY"`
	StripeWebhookSecret  string `mapstructure:"STRIPE_WEBHOOK_SECRET"`
	StripePremiumPriceID string `mapstructure:"STRIPE_PREMIUM_PRICE_ID"`

	// AI Providers
	OllamaURL      string `mapstructure:"OLLAMA_URL"`
	OllamaProxyURL string `mapstructure:"OLLAMA_PROXY_URL"`
	OllamaProxyKey string `mapstructure:"OLLAMA_PROXY_API_KEY"`

	// Service URLs (for internal communication)
	ChatServiceURL    string `mapstructure:"CHAT_SERVICE_URL"`
	AuthServiceURL    string `mapstructure:"AUTH_SERVICE_URL"`
	BillingServiceURL string `mapstructure:"BILLING_SERVICE_URL"`
	UsageServiceURL   string `mapstructure:"USAGE_SERVICE_URL"`

	// Rate Limiting
	FreeMessageLimit      int `mapstructure:"FREE_USER_MESSAGE_LIMIT"`
	RegisteredMessageLimit int `mapstructure:"REGISTERED_USER_MESSAGE_LIMIT"`
	PremiumMessageLimit    int `mapstructure:"PREMIUM_USER_MESSAGE_LIMIT"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Port:              getEnv("PORT", "3001"),
		Env:               getEnv("ENV", "development"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/saas_db?sslmode=disable"),
		RedisURL:          getEnv("REDIS_URL", "redis://localhost:6379"),
		NATSURL:           getEnv("NATS_URL", "nats://localhost:4222"),
		JWTSecret:         getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiration:     getDuration("JWT_EXPIRATION", 15*time.Minute),
		GoogleClientID:    getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GitHubClientID:    getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		StripeSecretKey:   getEnv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getEnv("STRIPE_WEBHOOK_SECRET", ""),
		StripePremiumPriceID: getEnv("STRIPE_PREMIUM_PRICE_ID", ""),
		OllamaURL:         getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaProxyURL:    getEnv("OLLAMA_PROXY_URL", ""),
		OllamaProxyKey:    getEnv("OLLAMA_PROXY_API_KEY", ""),
		ChatServiceURL:    getEnv("CHAT_SERVICE_URL", "http://localhost:3002"),
		AuthServiceURL:    getEnv("AUTH_SERVICE_URL", "http://localhost:3003"),
		BillingServiceURL: getEnv("BILLING_SERVICE_URL", "http://localhost:3004"),
		UsageServiceURL:   getEnv("USAGE_SERVICE_URL", "http://localhost:3005"),
		FreeMessageLimit:      getInt("FREE_USER_MESSAGE_LIMIT", 3),
		RegisteredMessageLimit: getInt("REGISTERED_USER_MESSAGE_LIMIT", 50),
		PremiumMessageLimit:    getInt("PREMIUM_USER_MESSAGE_LIMIT", 1000),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.Env) == "development" || strings.ToLower(c.Env) == "dev"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Env) == "production" || strings.ToLower(c.Env) == "prod"
}
