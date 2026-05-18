-- 008_fix_opencode_constraint.sql — Add 'opencode' to ai_providers type CHECK constraint and seed OpenCode provider

-- Fix CHECK constraint to include 'opencode'
ALTER TABLE ai_providers DROP CONSTRAINT IF EXISTS ai_providers_type_check;
ALTER TABLE ai_providers ADD CONSTRAINT ai_providers_type_check 
    CHECK (type IN ('ollama', 'openai', 'gemini', 'deepseek', 'kimi', 'lmstudio', 'anthropic', 'custom', 'opencode'));

-- Seed OpenCode provider (insert only if not exists)
-- Note: The API key should be provided via environment variable OPENCODE_API_KEY
INSERT INTO ai_providers (tenant_id, name, type, base_url, api_key_encrypted, is_active, is_public, priority, config)
SELECT t.id, 'OpenCode Zen', 'opencode', 'https://opencode.ai/zen/go/v1', COALESCE(current_setting('app.opencode_api_key', true), ''), true, true, 90, '{}'
FROM tenants t
WHERE NOT EXISTS (SELECT 1 FROM ai_providers ap WHERE ap.tenant_id = t.id AND ap.type = 'opencode')
LIMIT 1;

-- Seed OpenCode models (insert only if not exists)
INSERT INTO ai_models (provider_id, name, display_name, max_tokens, context_window, supports_streaming, supports_images, is_active, is_public)
SELECT ap.id, 'k2p6', 'OpenCode K2.6', 262144, 262144, true, false, true, true
FROM ai_providers ap
WHERE ap.type = 'opencode'
  AND NOT EXISTS (SELECT 1 FROM ai_models am WHERE am.provider_id = ap.id AND am.name = 'k2p6');

INSERT INTO ai_models (provider_id, name, display_name, max_tokens, context_window, supports_streaming, supports_images, is_active, is_public)
SELECT ap.id, 'k2p6-coder', 'OpenCode K2.6 Coder', 262144, 262144, true, false, true, true
FROM ai_providers ap
WHERE ap.type = 'opencode'
  AND NOT EXISTS (SELECT 1 FROM ai_models am WHERE am.provider_id = ap.id AND am.name = 'k2p6-coder');
