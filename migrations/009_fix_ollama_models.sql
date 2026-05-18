-- 009_fix_ollama_models.sql — Fix Ollama models to match local installation

-- Deactivate deepseek-r1:7b (not available locally)
UPDATE ai_models
SET is_active = false, updated_at = NOW()
WHERE name = 'deepseek-r1:7b'
  AND provider_id IN (SELECT id FROM ai_providers WHERE type = 'ollama');

-- Update qwen2.5-coder:7b to qwen2.5-coder:3b (correct local model)
UPDATE ai_models
SET name = 'qwen2.5-coder:3b',
    display_name = 'Qwen 2.5 Coder 3B',
    updated_at = NOW()
WHERE name = 'qwen2.5-coder:7b'
  AND provider_id IN (SELECT id FROM ai_providers WHERE type = 'ollama');

-- If the correct model doesn't exist yet, insert it
INSERT INTO ai_models (provider_id, name, display_name, max_tokens, context_window, supports_streaming, is_active, is_public)
SELECT ap.id, 'qwen2.5-coder:3b', 'Qwen 2.5 Coder 3B', 4096, 8192, true, true, true
FROM ai_providers ap
WHERE ap.type = 'ollama'
  AND NOT EXISTS (SELECT 1 FROM ai_models am WHERE am.provider_id = ap.id AND am.name = 'qwen2.5-coder:3b');
