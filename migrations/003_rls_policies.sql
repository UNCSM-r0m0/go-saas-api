-- 003_rls_policies.sql — Row-Level Security for multi-tenancy

-- Enable RLS on all tables
ALTER TABLE tenants ENABLE ROW LEVEL FORCE; -- tenants readable by all (filter in app)
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE artifacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE memories ENABLE ROW LEVEL SECURITY;
ALTER TABLE tool_executions ENABLE ROW LEVEL SECURITY;

-- Helper: set tenant from application
CREATE OR REPLACE FUNCTION set_tenant(tenant_id TEXT) RETURNS void AS $$
BEGIN
    PERFORM set_config('app.current_tenant', tenant_id, false);
END;
$$ LANGUAGE plpgsql;

-- Helper: get current tenant
CREATE OR REPLACE FUNCTION current_tenant() RETURNS UUID AS $$
DECLARE
    tid TEXT;
BEGIN
    tid := current_setting('app.current_tenant', true);
    IF tid IS NULL OR tid = '' THEN
        RETURN NULL;
    END IF;
    RETURN tid::UUID;
END;
$$ LANGUAGE plpgsql;

-- Users: tenant-scoped
CREATE POLICY tenant_users_isolation ON users
    USING (tenant_id = current_tenant());

-- API Keys: tenant-scoped
CREATE POLICY tenant_api_keys_isolation ON api_keys
    USING (tenant_id = current_tenant());

-- Agents: tenant-scoped
CREATE POLICY tenant_agents_isolation ON agents
    USING (tenant_id = current_tenant());

-- Conversations: tenant-scoped
CREATE POLICY tenant_conversations_isolation ON conversations
    USING (tenant_id = current_tenant());

-- Messages: tenant-scoped (via conversation check is too expensive; trust tenant_id)
CREATE POLICY tenant_messages_isolation ON messages
    USING (tenant_id = current_tenant());

-- Artifacts: tenant-scoped
CREATE POLICY tenant_artifacts_isolation ON artifacts
    USING (tenant_id = current_tenant());

-- Memories: tenant-scoped
CREATE POLICY tenant_memories_isolation ON memories
    USING (tenant_id = current_tenant());

-- Tool executions: tenant-scoped
CREATE POLICY tenant_tool_executions_isolation ON tool_executions
    USING (tenant_id = current_tenant());
