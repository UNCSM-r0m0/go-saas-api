package billing

import (
	"time"

	"github.com/google/uuid"
)

// Plan represents a pricing plan.
type Plan struct {
	ID                  uuid.UUID      `json:"id"`
	Slug                string         `json:"slug"`
	Name                string         `json:"name"`
	Description         string         `json:"description"`
	StripePriceID       string         `json:"stripe_price_id,omitempty"`
	AmountCents         int            `json:"amount_cents"`
	Currency            string         `json:"currency"`
	Interval            string         `json:"interval"`
	MessagesPerDay      int            `json:"messages_per_day"`
	MaxTokensPerRequest int            `json:"max_tokens_per_request"`
	Features            map[string]any `json:"features"`
	IsActive            bool           `json:"is_active"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

// Subscription represents a user's subscription.
type Subscription struct {
	ID                   uuid.UUID  `json:"id"`
	TenantID             uuid.UUID  `json:"tenant_id"`
	UserID               uuid.UUID  `json:"user_id"`
	PlanID               uuid.UUID  `json:"plan_id"`
	StripeCustomerID     string     `json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string     `json:"stripe_subscription_id,omitempty"`
	Status               string     `json:"status"`
	CurrentPeriodStart   *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end,omitempty"`
	CanceledAt           *time.Time `json:"canceled_at,omitempty"`
	CancelAtPeriodEnd    bool       `json:"cancel_at_period_end"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// UsageLog represents a single AI request usage record.
type UsageLog struct {
	ID             uuid.UUID `json:"id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	UserID         uuid.UUID `json:"user_id"`
	ConversationID *uuid.UUID `json:"conversation_id,omitempty"`
	Model          string    `json:"model"`
	Provider       string    `json:"provider"`
	TokensInput    int       `json:"tokens_input"`
	TokensOutput   int       `json:"tokens_output"`
	LatencyMs      int       `json:"latency_ms"`
	CostUSD        float64   `json:"cost_usd"`
	CreatedAt      time.Time `json:"created_at"`
}

// DailyUsage represents aggregated daily usage.
type DailyUsage struct {
	TenantID     uuid.UUID `json:"tenant_id"`
	UserID       uuid.UUID `json:"user_id"`
	Date         time.Time `json:"date"`
	Requests     int       `json:"requests"`
	TokensInput  int       `json:"tokens_input"`
	TokensOutput int       `json:"tokens_output"`
	CostUSD      float64   `json:"cost_usd"`
}
