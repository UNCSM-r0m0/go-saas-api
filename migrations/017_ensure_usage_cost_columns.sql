-- 017_ensure_usage_cost_columns.sql — Ensure usage cost columns exist for older databases

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS provider TEXT DEFAULT '';
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS latency_ms INTEGER DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS cost_usd DECIMAL(12, 6) NOT NULL DEFAULT 0;

ALTER TABLE usage_daily ADD COLUMN IF NOT EXISTS cost_usd DECIMAL(12, 6) NOT NULL DEFAULT 0;
