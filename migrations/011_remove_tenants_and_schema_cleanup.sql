-- 011_remove_tenants_and_schema_cleanup.sql
-- Remove multi-tenancy infrastructure and apply schema improvements

-- Phase 1: Drop RLS policies FIRST (must happen before column removal)
DROP POLICY IF EXISTS tenant_users_isolation ON users;
DROP POLICY IF EXISTS tenant_api_keys_isolation ON api_keys;
DROP POLICY IF EXISTS tenant_agents_isolation ON agents;
DROP POLICY IF EXISTS tenant_conversations_isolation ON conversations;
DROP POLICY IF EXISTS tenant_messages_isolation ON messages;
DROP POLICY IF EXISTS tenant_artifacts_isolation ON artifacts;
DROP POLICY IF EXISTS tenant_memories_isolation ON memories;
DROP POLICY IF EXISTS tenant_tool_executions_isolation ON tool_executions;

-- Disable RLS on all tables
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys DISABLE ROW LEVEL SECURITY;
ALTER TABLE agents DISABLE ROW LEVEL SECURITY;
ALTER TABLE conversations DISABLE ROW LEVEL SECURITY;
ALTER TABLE messages DISABLE ROW LEVEL SECURITY;
ALTER TABLE artifacts DISABLE ROW LEVEL SECURITY;
ALTER TABLE memories DISABLE ROW LEVEL SECURITY;
ALTER TABLE tool_executions DISABLE ROW LEVEL SECURITY;

-- Phase 2: Drop tenant_id indexes
DROP INDEX IF EXISTS idx_users_tenant_id;
DROP INDEX IF EXISTS idx_api_keys_tenant_id;
DROP INDEX IF EXISTS idx_agents_tenant_id;
DROP INDEX IF EXISTS idx_conversations_tenant_id;
DROP INDEX IF EXISTS idx_messages_tenant_id;
DROP INDEX IF EXISTS idx_artifacts_tenant_id;
DROP INDEX IF EXISTS idx_memories_tenant_id;
DROP INDEX IF EXISTS idx_files_tenant_id;
DROP INDEX IF EXISTS idx_subscriptions_tenant_id;
DROP INDEX IF EXISTS idx_usage_logs_tenant_id;
DROP INDEX IF EXISTS idx_usage_daily_tenant_id;
DROP INDEX IF EXISTS idx_ai_providers_tenant_id;

-- Phase 3: Drop tenant_id columns from all tables
ALTER TABLE users DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE agents DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE conversations DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE messages DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE artifacts DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE memories DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE tool_executions DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE usage_logs DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE usage_daily DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE files DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE ai_providers DROP COLUMN IF EXISTS tenant_id;

-- Phase 4: Drop helper functions
DROP FUNCTION IF EXISTS set_tenant(TEXT);
DROP FUNCTION IF EXISTS current_tenant();

-- Phase 5: Drop tenants table (cascades to FK constraints)
DROP TABLE IF EXISTS tenants CASCADE;

-- Phase 6: Update users unique constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);

-- Phase 7: Create message_files junction table
CREATE TABLE IF NOT EXISTS message_files (
    message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, file_id)
);

-- Migrate existing messages.file_ids[] data into message_files
INSERT INTO message_files (message_id, file_id)
SELECT id, unnest(file_ids)
FROM messages
WHERE file_ids IS NOT NULL
  AND array_length(file_ids, 1) > 0
  AND EXISTS (SELECT 1 FROM files WHERE files.id = unnest(file_ids));

-- Alternative safer migration with WHERE EXISTS guard per spec
-- The above uses unnest in SELECT which PostgreSQL handles correctly
-- For extra safety, we use a guarded version:
INSERT INTO message_files (message_id, file_id)
SELECT m.id, fid
FROM messages m,
     LATERAL unnest(m.file_ids) AS fid
WHERE m.file_ids IS NOT NULL
  AND array_length(m.file_ids, 1) > 0
  AND EXISTS (SELECT 1 FROM files f WHERE f.id = fid)
ON CONFLICT (message_id, file_id) DO NOTHING;

-- Drop messages.file_ids column
ALTER TABLE messages DROP COLUMN IF EXISTS file_ids;

-- Phase 8: Drop memories.embedding column
ALTER TABLE memories DROP COLUMN IF EXISTS embedding;

-- Phase 9: Rename plans.messages_per_day to messages_per_month
ALTER TABLE plans RENAME COLUMN messages_per_day TO messages_per_month;

-- Phase 10: Fix usage_daily primary key
ALTER TABLE usage_daily DROP CONSTRAINT IF EXISTS usage_daily_pkey;
ALTER TABLE usage_daily ADD PRIMARY KEY (user_id, date);

-- Phase 11: Add new columns to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS messages_used_this_month INT NOT NULL DEFAULT 0;

-- Phase 12: Drop tool_executions table
DROP TABLE IF EXISTS tool_executions CASCADE;

-- Phase 13: Add encryption_key_version to ai_providers
ALTER TABLE ai_providers ADD COLUMN IF NOT EXISTS encryption_key_version INT NOT NULL DEFAULT 1;
