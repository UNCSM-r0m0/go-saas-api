package config

import (
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
	OpenAIAPIKey   string `mapstructure:"OPENAI_API_KEY"`
	OpenAIBaseURL  string `mapstructure:"OPENAI_BASE_URL"`
	GeminiAPIKey   string `mapstructure:"GEMINI_API_KEY"`
	DeepSeekAPIKey string `mapstructure:"DEEPSEEK_API_KEY"`
	LMStudioURL    string `mapstructure:"LM_STUDIO_URL"`
	LMStudioAPIKey string `mapstructure:"LM_STUDIO_API_KEY"`
	SandboxServiceURL string `mapstructure:"SANDBOX_SERVICE_URL"`

	// Frontend / Public URL
	FrontendURL string `mapstructure:"FRONTEND_URL"`
	PublicURL   string `mapstructure:"PUBLIC_URL"`

	// Service URLs (for internal communication)
	AgentServiceURL    string `mapstructure:"AGENT_SERVICE_URL"`
	AuthServiceURL    string `mapstructure:"AUTH_SERVICE_URL"`
	BillingServiceURL string `mapstructure:"BILLING_SERVICE_URL"`
	UsageServiceURL   string `mapstructure:"USAGE_SERVICE_URL"`

	// Rate Limiting
	FreeMessageLimit      int `mapstructure:"FREE_USER_MESSAGE_LIMIT"`
	RegisteredMessageLimit int `mapstructure:"REGISTERED_USER_MESSAGE_LIMIT"`
	PremiumMessageLimit    int `mapstructure:"PREMIUM_USER_MESSAGE_LIMIT"`

	// File Upload
	UploadPath     string `mapstructure:"UPLOAD_PATH"`
	MaxUploadSize  int64  `mapstructure:"MAX_UPLOAD_SIZE"`

	// SMTP / Email
	SMTPHost     string `mapstructure:"SMTP_HOST"`
	SMTPPort     string `mapstructure:"SMTP_PORT"`
	SMTPUser     string `mapstructure:"SMTP_USER"`
	SMTPPassword string `mapstructure:"SMTP_PASSWORD"`
	SMTPFrom     string `mapstructure:"SMTP_FROM"`
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
		OpenAIAPIKey:      getEnv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:     getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		GeminiAPIKey:      getEnv("GEMINI_API_KEY", ""),
		DeepSeekAPIKey:    getEnv("DEEPSEEK_API_KEY", ""),
		LMStudioURL:       getEnv("LM_STUDIO_URL", ""),
		LMStudioAPIKey:    getEnv("LM_STUDIO_API_KEY", ""),
		SandboxServiceURL: getEnv("SANDBOX_SERVICE_URL", "http://localhost:3006"),
		FrontendURL:       getEnv("FRONTEND_URL", "http://localhost:5173"),
		PublicURL:         getEnv("PUBLIC_URL", "http://localhost:3000"),
		AgentServiceURL:    getEnv("AGENT_SERVICE_URL", "http://localhost:3002"),
		AuthServiceURL:    getEnv("AUTH_SERVICE_URL", "http://localhost:3001"),
		BillingServiceURL: getEnv("BILLING_SERVICE_URL", "http://localhost:3003"),
		UsageServiceURL:   getEnv("USAGE_SERVICE_URL", "http://localhost:3004"),
		FreeMessageLimit:      getInt("FREE_USER_MESSAGE_LIMIT", 3),
		RegisteredMessageLimit: getInt("REGISTERED_USER_MESSAGE_LIMIT", 10),
		PremiumMessageLimit:    getInt("PREMIUM_USER_MESSAGE_LIMIT", 100),
		UploadPath:            getEnv("UPLOAD_PATH", "./uploads"),
		MaxUploadSize:         getInt64("MAX_UPLOAD_SIZE", 10*1024*1024), // 10MB
		SMTPHost:              getEnv("SMTP_HOST", ""),
		SMTPPort:              getEnv("SMTP_PORT", "587"),
		SMTPUser:              getEnv("SMTP_USER", ""),
		SMTPPassword:          getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:              getEnv("SMTP_FROM", "noreply@example.com"),
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

func getInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
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
