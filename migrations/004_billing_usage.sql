-- 004_billing_usage.sql â€” Billing, subscriptions, and usage tracking

-- Plans (product catalog)
CREATE TABLE IF NOT EXISTS plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    stripe_price_id TEXT,
    amount_cents INTEGER NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'usd',
    interval TEXT NOT NULL DEFAULT 'month' CHECK (interval IN ('month', 'year')),
    messages_per_day INTEGER NOT NULL DEFAULT 3,
    max_tokens_per_request INTEGER NOT NULL DEFAULT 2048,
    features JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Subscriptions (Stripe-backed)
CREATE TABLE IF NOT EXISTS subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
    stripe_customer_id TEXT,
    stripe_subscription_id TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'incomplete' CHECK (status IN ('incomplete', 'active', 'past_due', 'canceled', 'unpaid')),
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    canceled_at TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Usage logs (per request)
CREATE TABLE IF NOT EXISTS usage_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    tokens_input INTEGER NOT NULL DEFAULT 0,
    tokens_output INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER,
    cost_usd DECIMAL(12, 6) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Daily usage rollup (for fast limit checks)
CREATE TABLE IF NOT EXISTS usage_daily (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    requests INTEGER NOT NULL DEFAULT 0,
    tokens_input INTEGER NOT NULL DEFAULT 0,
    tokens_output INTEGER NOT NULL DEFAULT 0,
    cost_usd DECIMAL(12, 6) NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, user_id, date)
);

-- Seed default plans
INSERT INTO plans (slug, name, description, amount_cents, interval, messages_per_day, max_tokens_per_request, features)
VALUES
    ('free', 'Free', 'Limited access for anonymous and unauthenticated users', 0, 'month', 3, 2048, '{"streaming": false, "images": false}'),
    ('registered', 'Registered', 'Standard access for authenticated users', 0, 'month', 50, 4096, '{"streaming": true, "images": false}'),
    ('premium', 'Premium', 'Full access with higher limits', 999, 'month', 1000, 8192, '{"streaming": true, "images": true, "priority": true}')
ON CONFLICT (slug) DO NOTHING;

-- Indexes
CREATE INDEX idx_subscriptions_tenant_id ON subscriptions(tenant_id);
CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_stripe_sub_id ON subscriptions(stripe_subscription_id);
CREATE INDEX idx_usage_logs_tenant_id ON usage_logs(tenant_id);
CREATE INDEX idx_usage_logs_user_id ON usage_logs(user_id);
CREATE INDEX idx_usage_logs_created_at ON usage_logs(created_at);
CREATE INDEX idx_usage_daily_date ON usage_daily(date);

-- RLS
ALTER TABLE plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_daily ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_subscriptions_isolation ON subscriptions
    USING (tenant_id = current_tenant());

CREATE POLICY tenant_usage_logs_isolation ON usage_logs
    USING (tenant_id = current_tenant());

CREATE POLICY tenant_usage_daily_isolation ON usage_daily
    USING (tenant_id = current_tenant());

-- Plans are globally readable
CREATE POLICY plans_readable ON plans
    FOR SELECT USING (true);
