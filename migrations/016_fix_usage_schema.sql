-- 016_fix_usage_schema.sql
-- Normalize usage_logs and usage_daily so that cost tracking works correctly.
-- The billing repository was not inserting provider, latency_ms, or cost_usd,
-- and usage_daily was not accumulating cost_usd.

-- Make provider nullable so inserts without it don't fail.
-- Existing rows keep their current values; new rows without provider get ''.
ALTER TABLE usage_logs ALTER COLUMN provider DROP NOT NULL;
ALTER TABLE usage_logs ALTER COLUMN provider SET DEFAULT '';

-- Ensure defaults for the columns the repository now writes.
ALTER TABLE usage_logs ALTER COLUMN cost_usd SET DEFAULT 0;
ALTER TABLE usage_logs ALTER COLUMN latency_ms SET DEFAULT 0;

-- Ensure usage_daily has cost_usd with a proper default.
ALTER TABLE usage_daily ALTER COLUMN cost_usd SET DEFAULT 0;
