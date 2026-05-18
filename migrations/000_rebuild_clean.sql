-- 000_rebuild_clean.sql
-- Clean rebuild: no tenants, no RLS, no pgvector dependencies.
-- Replaces migrations 001 through 012.

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT,
    name TEXT,
    role TEXT NOT NULL DEFAULT 'registered' CHECK (role IN ('registered', 'premium', 'admin')),
    is_admin BOOLEAN NOT NULL DEFAULT false,
    oauth_provider TEXT,
    oauth_subject TEXT,
    password_reset_token TEXT,
    password_reset_expires TIMESTAMPTZ,
    messages_used_this_month INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- API Keys
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    scopes TEXT[] NOT NULL DEFAULT ARRAY['chat'],
    rate_limit INT NOT NULL DEFAULT 60,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);

-- Agents
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'assistant' CHECK (role IN ('system', 'coder', 'copywriter', 'critic', 'researcher', 'assistant')),
    model TEXT NOT NULL DEFAULT 'qwen2.5-coder:3b',
    system_prompt TEXT,
    tools_enabled TEXT[] NOT NULL DEFAULT '{}',
    settings JSONB NOT NULL DEFAULT '{}',
    parent_agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Conversations
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT,
    agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived', 'deleted')),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Messages
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system', 'tool')),
    content TEXT NOT NULL,
    tool_calls JSONB,
    tool_call_id TEXT,
    tool_name TEXT,
    model TEXT,
    tokens_input INTEGER,
    tokens_output INTEGER,
    latency_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_messages_tool_call_id ON messages(tool_call_id) WHERE tool_call_id IS NOT NULL;

-- Artifacts
CREATE TABLE artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    language TEXT,
    content TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    is_deleted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Plans
CREATE TABLE plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    stripe_price_id TEXT,
    messages_per_month INT NOT NULL DEFAULT 10,
    features JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO plans (id, name, stripe_price_id, messages_per_month, features) VALUES
    (gen_random_uuid(), 'registered', NULL, 10, '{"agentic": false, "sandbox": false}'),
    (gen_random_uuid(), 'premium', NULL, 100, '{"agentic": true, "sandbox": true}');

-- Subscriptions
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES plans(id),
    stripe_customer_id TEXT,
    stripe_subscription_id TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'past_due', 'canceled', 'incomplete')),
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Usage Logs
CREATE TABLE usage_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id UUID REFERENCES conversations(id) ON DELETE SET NULL,
    model TEXT,
    tokens_input INT NOT NULL DEFAULT 0,
    tokens_output INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_usage_logs_user_id ON usage_logs(user_id);

-- Daily Usage
CREATE TABLE usage_daily (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    message_count INT NOT NULL DEFAULT 0,
    tokens_input INT NOT NULL DEFAULT 0,
    tokens_output INT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, date)
);

-- Files
CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    original_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    storage_path TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Message Files (junction table)
CREATE TABLE message_files (
    message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    file_id UUID REFERENCES files(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, file_id)
);

-- AI Providers
CREATE TABLE ai_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL CHECK (type IN ('ollama', 'openai', 'opencode', 'deepseek', 'gemini', 'kimi', 'kimi-anthropic', 'lmstudio')),
    base_url TEXT NOT NULL,
    api_key_encrypted TEXT,
    api_key_hash TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_public BOOLEAN NOT NULL DEFAULT true,
    priority INT NOT NULL DEFAULT 1,
    config JSONB NOT NULL DEFAULT '{}',
    encryption_key_version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- AI Models
CREATE TABLE ai_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES ai_providers(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_name TEXT,
    description TEXT,
    max_tokens INT NOT NULL DEFAULT 4096,
    context_window INT NOT NULL DEFAULT 8192,
    supports_streaming BOOLEAN NOT NULL DEFAULT true,
    supports_images BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_public BOOLEAN NOT NULL DEFAULT true,
    is_premium BOOLEAN NOT NULL DEFAULT false,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_provider_model_name UNIQUE (provider_id, name)
);

-- Seed default Ollama provider
INSERT INTO ai_providers (name, type, base_url, is_active, is_public, priority) VALUES
    ('Ollama Local', 'ollama', 'http://localhost:11434', true, true, 1);

-- Seed default models for Ollama
INSERT INTO ai_models (provider_id, name, display_name, max_tokens, context_window, is_public) VALUES
    ((SELECT id FROM ai_providers WHERE name = 'Ollama Local'), 'qwen2.5-coder:3b', 'Qwen 2.5 Coder 3B', 4096, 32768, true),
    ((SELECT id FROM ai_providers WHERE name = 'Ollama Local'), 'qwen2.5-coder:7b', 'Qwen 2.5 Coder 7B', 8192, 131072, true);
