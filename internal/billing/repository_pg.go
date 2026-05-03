package billing

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresBillingStore implements Plan, Subscription, and Usage repositories.
type PostgresBillingStore struct {
	pool *pgxpool.Pool
}

// NewPostgresBillingStore creates a new billing store.
func NewPostgresBillingStore(pool *pgxpool.Pool) *PostgresBillingStore {
	return &PostgresBillingStore{pool: pool}
}

// ---- PlanRepository ----

// ListPlans returns all active plans.
func (s *PostgresBillingStore) ListPlans(ctx context.Context) ([]Plan, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, slug, name, description, stripe_price_id, amount_cents, currency, interval,
		       messages_per_month, max_tokens_per_request, features, is_active, created_at, updated_at
		FROM plans WHERE is_active = true ORDER BY amount_cents ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.StripePriceID, &p.AmountCents,
			&p.Currency, &p.Interval, &p.MessagesPerMonth, &p.MaxTokensPerRequest, &p.Features,
			&p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

// GetPlanBySlug fetches a plan by slug.
func (s *PostgresBillingStore) GetPlanBySlug(ctx context.Context, slug string) (*Plan, error) {
	var p Plan
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, name, description, stripe_price_id, amount_cents, currency, interval,
		       messages_per_month, max_tokens_per_request, features, is_active, created_at, updated_at
		FROM plans WHERE slug = $1
	`, slug).Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.StripePriceID, &p.AmountCents,
		&p.Currency, &p.Interval, &p.MessagesPerMonth, &p.MaxTokensPerRequest, &p.Features,
		&p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPlanByID fetches a plan by ID.
func (s *PostgresBillingStore) GetPlanByID(ctx context.Context, id uuid.UUID) (*Plan, error) {
	var p Plan
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, name, description, stripe_price_id, amount_cents, currency, interval,
		       messages_per_month, max_tokens_per_request, features, is_active, created_at, updated_at
		FROM plans WHERE id = $1
	`, id).Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.StripePriceID, &p.AmountCents,
		&p.Currency, &p.Interval, &p.MessagesPerMonth, &p.MaxTokensPerRequest, &p.Features,
		&p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPlanByStripePriceID fetches a plan by Stripe price ID.
func (s *PostgresBillingStore) GetPlanByStripePriceID(ctx context.Context, priceID string) (*Plan, error) {
	var p Plan
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, name, description, stripe_price_id, amount_cents, currency, interval,
		       messages_per_month, max_tokens_per_request, features, is_active, created_at, updated_at
		FROM plans WHERE stripe_price_id = $1
	`, priceID).Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.StripePriceID, &p.AmountCents,
		&p.Currency, &p.Interval, &p.MessagesPerMonth, &p.MaxTokensPerRequest, &p.Features,
		&p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ---- SubscriptionRepository ----

// CreateSubscription inserts a new subscription.
func (s *PostgresBillingStore) CreateSubscription(ctx context.Context, sub *Subscription) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO subscriptions (id, user_id, plan_id, stripe_customer_id, stripe_subscription_id,
			status, current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, sub.ID, sub.UserID, sub.PlanID, sub.StripeCustomerID, sub.StripeSubscriptionID,
		sub.Status, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd, sub.CreatedAt, sub.UpdatedAt)
	return err
}

// GetSubscriptionByID fetches a subscription by ID.
func (s *PostgresBillingStore) GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	var sub Subscription
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, plan_id, stripe_customer_id, stripe_subscription_id,
			status, current_period_start, current_period_end, canceled_at, cancel_at_period_end, created_at, updated_at
		FROM subscriptions WHERE id = $1
	`, id).Scan(&sub.ID, &sub.UserID, &sub.PlanID, &sub.StripeCustomerID, &sub.StripeSubscriptionID,
		&sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CanceledAt, &sub.CancelAtPeriodEnd, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetSubscriptionByUser fetches the active subscription for a user.
func (s *PostgresBillingStore) GetSubscriptionByUser(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	var sub Subscription
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, plan_id, stripe_customer_id, stripe_subscription_id,
			status, current_period_start, current_period_end, canceled_at, cancel_at_period_end, created_at, updated_at
		FROM subscriptions
		WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, userID).Scan(&sub.ID, &sub.UserID, &sub.PlanID, &sub.StripeCustomerID, &sub.StripeSubscriptionID,
		&sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CanceledAt, &sub.CancelAtPeriodEnd, &sub.CreatedAt, &sub.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetSubscriptionByStripeID fetches by Stripe subscription ID.
func (s *PostgresBillingStore) GetSubscriptionByStripeID(ctx context.Context, stripeSubID string) (*Subscription, error) {
	var sub Subscription
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, plan_id, stripe_customer_id, stripe_subscription_id,
			status, current_period_start, current_period_end, canceled_at, cancel_at_period_end, created_at, updated_at
		FROM subscriptions WHERE stripe_subscription_id = $1
	`, stripeSubID).Scan(&sub.ID, &sub.UserID, &sub.PlanID, &sub.StripeCustomerID, &sub.StripeSubscriptionID,
		&sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CanceledAt, &sub.CancelAtPeriodEnd, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// UpdateSubscription saves changes to a subscription.
func (s *PostgresBillingStore) UpdateSubscription(ctx context.Context, sub *Subscription) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE subscriptions SET
			plan_id = $1, stripe_customer_id = $2, stripe_subscription_id = $3,
			status = $4, current_period_start = $5, current_period_end = $6,
			canceled_at = $7, cancel_at_period_end = $8, updated_at = $9
		WHERE id = $10
	`, sub.PlanID, sub.StripeCustomerID, sub.StripeSubscriptionID, sub.Status,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CanceledAt, sub.CancelAtPeriodEnd,
		sub.UpdatedAt, sub.ID)
	return err
}

// CancelSubscription marks a subscription as canceled.
func (s *PostgresBillingStore) CancelSubscription(ctx context.Context, id uuid.UUID, canceledAt time.Time, cancelAtPeriodEnd bool) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE subscriptions SET
			status = 'canceled', canceled_at = $1, cancel_at_period_end = $2, updated_at = $3
		WHERE id = $4
	`, canceledAt, cancelAtPeriodEnd, time.Now().UTC(), id)
	return err
}

// ---- UsageRepository ----

// LogUsage records a single usage event.
func (s *PostgresBillingStore) LogUsage(ctx context.Context, log *UsageLog) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO usage_logs (id, user_id, conversation_id, model, provider,
			tokens_input, tokens_output, latency_ms, cost_usd, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, log.ID, log.UserID, log.ConversationID, log.Model, log.Provider,
		log.TokensInput, log.TokensOutput, log.LatencyMs, log.CostUSD, log.CreatedAt)
	return err
}

// GetDailyUsage fetches the daily rollup for a user.
func (s *PostgresBillingStore) GetDailyUsage(ctx context.Context, userID uuid.UUID, date time.Time) (*DailyUsage, error) {
	var du DailyUsage
	err := s.pool.QueryRow(ctx, `
		SELECT user_id, date, requests, tokens_input, tokens_output, cost_usd
		FROM usage_daily WHERE user_id = $1 AND date = $2
	`, userID, date).Scan(&du.UserID, &du.Date, &du.Requests, &du.TokensInput, &du.TokensOutput, &du.CostUSD)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &du, nil
}

// IncrementDailyUsage upserts daily usage counters.
func (s *PostgresBillingStore) IncrementDailyUsage(ctx context.Context, userID uuid.UUID, date time.Time, tokensIn, tokensOut int, costUSD float64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO usage_daily (user_id, date, requests, tokens_input, tokens_output, cost_usd)
		VALUES ($1, $2, 1, $3, $4, $5)
		ON CONFLICT (user_id, date)
		DO UPDATE SET
			requests = usage_daily.requests + 1,
			tokens_input = usage_daily.tokens_input + $3,
			tokens_output = usage_daily.tokens_output + $4,
			cost_usd = usage_daily.cost_usd + $5
	`, userID, date, tokensIn, tokensOut, costUSD)
	return err
}

// GetUsageStats aggregates usage over a date range.
func (s *PostgresBillingStore) GetUsageStats(ctx context.Context, userID uuid.UUID, from, to time.Time) (*UsageStats, error) {
	var stats UsageStats
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(requests), 0), COALESCE(SUM(tokens_input), 0),
		       COALESCE(SUM(tokens_output), 0), COALESCE(SUM(cost_usd), 0)
		FROM usage_daily
		WHERE user_id = $1 AND date >= $2 AND date <= $3
	`, userID, from, to).Scan(&stats.TotalRequests, &stats.TokensInput, &stats.TokensOutput, &stats.TotalCostUSD)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

var _ PlanRepository = (*PostgresBillingStore)(nil)
var _ SubscriptionRepository = (*PostgresBillingStore)(nil)
var _ UsageRepository = (*PostgresBillingStore)(nil)
