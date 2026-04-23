-- 006_ai_providers.sql â€” AI providers and models (configurable in DB)

CREATE TABLE IF NOT EXISTS ai_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('ollama', 'openai', 'gemini', 'deepseek', 'kimi', 'anthropic', 'custom')),
    base_url TEXT NOT NULL,
    api_key_encrypted TEXT,
    api_key_hash TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_public BOOLEAN NOT NULL DEFAULT true,
    priority INTEGER NOT NULL DEFAULT 0,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ai_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES ai_providers(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT,
    max_tokens INTEGER NOT NULL DEFAULT 4096,
    context_window INTEGER NOT NULL DEFAULT 8192,
    supports_streaming BOOLEAN NOT NULL DEFAULT true,
    supports_images BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_public BOOLEAN NOT NULL DEFAULT true,
    is_premium BOOLEAN NOT NULL DEFAULT false,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Indexes
CREATE INDEX idx_ai_providers_tenant_id ON ai_providers(tenant_id);
CREATE INDEX idx_ai_providers_type ON ai_providers(type);
CREATE INDEX idx_ai_providers_active ON ai_providers(is_active);
CREATE INDEX idx_ai_models_provider_id ON ai_models(provider_id);
CREATE INDEX idx_ai_models_active ON ai_models(is_active);
CREATE INDEX idx_ai_models_public ON ai_models(is_public);

-- Seed default providers (public, tenant-aware via RLS)
INSERT INTO ai_providers (tenant_id, name, type, base_url, is_active, is_public, priority, config)
SELECT t.id, 'Ollama Local', 'ollama', 'http://host.docker.internal:11434', true, true, 50, '{}'
FROM tenants t
WHERE NOT EXISTS (SELECT 1 FROM ai_providers ap WHERE ap.tenant_id = t.id AND ap.type = 'ollama')
LIMIT 1;

INSERT INTO ai_models (provider_id, name, display_name, max_tokens, context_window, supports_streaming, is_active, is_public)
SELECT ap.id, 'qwen2.5-coder:7b', 'Qwen 2.5 Coder 7B', 4096, 8192, true, true, true
FROM ai_providers ap
WHERE ap.type = 'ollama'
  AND NOT EXISTS (SELECT 1 FROM ai_models am WHERE am.provider_id = ap.id AND am.name = 'qwen2.5-coder:7b');

INSERT INTO ai_models (provider_id, name, display_name, max_tokens, context_window, supports_streaming, is_active, is_public)
SELECT ap.id, 'deepseek-r1:7b', 'DeepSeek R1 7B', 4096, 8192, true, true, true
FROM ai_providers ap
WHERE ap.type = 'ollama'
  AND NOT EXISTS (SELECT 1 FROM ai_models am WHERE am.provider_id = ap.id AND am.name = 'deepseek-r1:7b');

-- RLS
ALTER TABLE ai_providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_models ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_ai_providers_isolation ON ai_providers
    USING (tenant_id = current_tenant());

CREATE POLICY tenant_ai_models_isolation ON ai_models
    USING (provider_id IN (SELECT id FROM ai_providers WHERE tenant_id = current_tenant()));
