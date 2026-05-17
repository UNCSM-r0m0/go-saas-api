package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Port     string `mapstructure:"PORT"`
	Env      string `mapstructure:"ENV"`
	LogLevel string `mapstructure:"LOG_LEVEL"`

	DatabaseURL string `mapstructure:"DATABASE_URL"`
	RedisURL    string `mapstructure:"REDIS_URL"`
	NATSURL     string `mapstructure:"NATS_URL"`

	JWTSecret     string        `mapstructure:"JWT_SECRET"`
	JWTExpiration time.Duration `mapstructure:"JWT_EXPIRATION"`

	GoogleClientID     string `mapstructure:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `mapstructure:"GOOGLE_CLIENT_SECRET"`
	GitHubClientID     string `mapstructure:"GITHUB_CLIENT_ID"`
	GitHubClientSecret string `mapstructure:"GITHUB_CLIENT_SECRET"`

	StripeSecretKey      string `mapstructure:"STRIPE_SECRET_KEY"`
	StripeWebhookSecret  string `mapstructure:"STRIPE_WEBHOOK_SECRET"`
	StripePremiumPriceID string `mapstructure:"STRIPE_PREMIUM_PRICE_ID"`

	MasterEncryptionKey string `mapstructure:"MASTER_ENCRYPTION_KEY"`

	SandboxServiceURL  string `mapstructure:"SANDBOX_SERVICE_URL"`
	DocumentServiceURL string `mapstructure:"DOCUMENT_SERVICE_URL"`

	FrontendURL string `mapstructure:"FRONTEND_URL"`
	PublicURL   string `mapstructure:"PUBLIC_URL"`

	AgentServiceURL    string `mapstructure:"AGENT_SERVICE_URL"`
	AuthServiceURL    string `mapstructure:"AUTH_SERVICE_URL"`
	BillingServiceURL string `mapstructure:"BILLING_SERVICE_URL"`
	UsageServiceURL   string `mapstructure:"USAGE_SERVICE_URL"`

	RegisteredMessageLimit int `mapstructure:"REGISTERED_USER_MESSAGE_LIMIT"`
	PremiumMessageLimit    int `mapstructure:"PREMIUM_USER_MESSAGE_LIMIT"`

	UploadPath    string `mapstructure:"UPLOAD_PATH"`
	MaxUploadSize int64  `mapstructure:"MAX_UPLOAD_SIZE"`

	SandboxRateLimit  int  `mapstructure:"SANDBOX_RATE_LIMIT"`
	NATSEventsEnabled bool `mapstructure:"NATS_EVENTS_ENABLED"`

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
		MasterEncryptionKey: getEnv("MASTER_ENCRYPTION_KEY", ""),
		SandboxServiceURL:  getEnv("SANDBOX_SERVICE_URL", "http://localhost:3006"),
		DocumentServiceURL: getEnv("DOCUMENT_SERVICE_URL", "http://localhost:3007"),
		FrontendURL:       getEnv("FRONTEND_URL", "http://localhost:5173"),
		PublicURL:         getEnv("PUBLIC_URL", "http://localhost:3000"),
		AgentServiceURL:    getEnv("AGENT_SERVICE_URL", "http://localhost:3002"),
		AuthServiceURL:    getEnv("AUTH_SERVICE_URL", "http://localhost:3001"),
		BillingServiceURL: getEnv("BILLING_SERVICE_URL", "http://localhost:3003"),
		UsageServiceURL:   getEnv("USAGE_SERVICE_URL", "http://localhost:3004"),
		RegisteredMessageLimit: getInt("REGISTERED_USER_MESSAGE_LIMIT", 10),
		PremiumMessageLimit:    getInt("PREMIUM_USER_MESSAGE_LIMIT", 100),
		UploadPath:            getEnv("UPLOAD_PATH", "./uploads"),
		MaxUploadSize:         getInt64("MAX_UPLOAD_SIZE", 10*1024*1024),
		SandboxRateLimit:      getInt("SANDBOX_RATE_LIMIT", 5),
		NATSEventsEnabled:     getEnv("NATS_EVENTS_ENABLED", "true") == "true",
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
