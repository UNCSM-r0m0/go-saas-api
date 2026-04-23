package billing

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PlanRepository defines persistence for plans.
type PlanRepository interface {
	ListPlans(ctx context.Context) ([]Plan, error)
	GetPlanBySlug(ctx context.Context, slug string) (*Plan, error)
	GetPlanByID(ctx context.Context, id uuid.UUID) (*Plan, error)
	GetPlanByStripePriceID(ctx context.Context, priceID string) (*Plan, error)
}

// SubscriptionRepository defines persistence for subscriptions.
type SubscriptionRepository interface {
	CreateSubscription(ctx context.Context, sub *Subscription) error
	GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*Subscription, error)
	GetSubscriptionByUser(ctx context.Context, tenantID, userID uuid.UUID) (*Subscription, error)
	GetSubscriptionByStripeID(ctx context.Context, stripeSubID string) (*Subscription, error)
	UpdateSubscription(ctx context.Context, sub *Subscription) error
	CancelSubscription(ctx context.Context, id uuid.UUID, canceledAt time.Time, cancelAtPeriodEnd bool) error
}

// UsageRepository defines persistence for usage tracking.
type UsageRepository interface {
	LogUsage(ctx context.Context, log *UsageLog) error
	GetDailyUsage(ctx context.Context, tenantID, userID uuid.UUID, date time.Time) (*DailyUsage, error)
	IncrementDailyUsage(ctx context.Context, tenantID, userID uuid.UUID, date time.Time, tokensIn, tokensOut int, costUSD float64) error
	GetUsageStats(ctx context.Context, tenantID, userID uuid.UUID, from, to time.Time) (*UsageStats, error)
}

// UsageStats aggregates usage over a period.
type UsageStats struct {
	TotalRequests  int     `json:"total_requests"`
	TokensInput    int     `json:"tokens_input"`
	TokensOutput   int     `json:"tokens_output"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
}
