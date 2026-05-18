-- Align legacy local billing tables with the current billing repository schema.
-- This migration is additive and safe for existing dev data.

ALTER TABLE plans ADD COLUMN IF NOT EXISTS slug TEXT;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
ALTER TABLE plans ADD COLUMN IF NOT EXISTS amount_cents INTEGER NOT NULL DEFAULT 0;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS currency TEXT NOT NULL DEFAULT 'usd';
ALTER TABLE plans ADD COLUMN IF NOT EXISTS interval TEXT NOT NULL DEFAULT 'month';
ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_tokens_per_request INTEGER NOT NULL DEFAULT 4096;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE plans ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE plans
SET slug = lower(regexp_replace(name, '[^a-zA-Z0-9]+', '-', 'g'))
WHERE slug IS NULL OR slug = '';

UPDATE plans
SET slug = 'registered', description = COALESCE(NULLIF(description, ''), 'Registered user plan'), amount_cents = 0
WHERE lower(name) = 'registered' OR slug = 'registered';

UPDATE plans
SET slug = 'premium', description = COALESCE(NULLIF(description, ''), 'Premium subscription plan')
WHERE lower(name) = 'premium' OR slug = 'premium';

INSERT INTO plans (slug, name, description, amount_cents, currency, interval, messages_per_month, max_tokens_per_request, features, is_active, created_at, updated_at)
SELECT 'registered', 'Registered', 'Registered user plan', 0, 'usd', 'month', 100, 4096, '{}'::jsonb, true, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM plans WHERE slug = 'registered');

INSERT INTO plans (slug, name, description, amount_cents, currency, interval, messages_per_month, max_tokens_per_request, features, is_active, created_at, updated_at)
SELECT 'premium', 'Premium', 'Premium subscription plan', 2000, 'usd', 'month', 1000, 8192, '{}'::jsonb, true, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM plans WHERE slug = 'premium');


INSERT INTO plans (slug, name, description, amount_cents, currency, interval, messages_per_month, max_tokens_per_request, features, is_active, created_at, updated_at)
SELECT 'free', 'Free', 'Legacy compatibility plan mapped by the frontend to registered access', 0, 'usd', 'month', 100, 4096, '{}'::jsonb, true, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM plans WHERE slug = 'free');

CREATE UNIQUE INDEX IF NOT EXISTS plans_slug_key ON plans(slug);

ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS canceled_at TIMESTAMPTZ;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS cancel_at_period_end BOOLEAN NOT NULL DEFAULT false;
