-- 010_encrypt_api_keys.sql — Ensure api_key_encrypted column exists and migrate plaintext keys

-- Add api_key_encrypted column if it doesn't exist (for databases created before migration 006)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ai_providers' AND column_name = 'api_key_encrypted'
    ) THEN
        ALTER TABLE ai_providers ADD COLUMN api_key_encrypted TEXT;
    END IF;
END $$;

-- Add api_key_hash column if it doesn't exist
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ai_providers' AND column_name = 'api_key_hash'
    ) THEN
        ALTER TABLE ai_providers ADD COLUMN api_key_hash TEXT;
    END IF;
END $$;

-- If there was an old 'api_key' plaintext column, migrate its data to api_key_encrypted
-- (the actual encryption will be performed by the application on startup)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'ai_providers' AND column_name = 'api_key'
    ) THEN
        UPDATE ai_providers
        SET api_key_encrypted = api_key
        WHERE api_key_encrypted IS NULL AND api_key IS NOT NULL AND api_key <> '';

        ALTER TABLE ai_providers DROP COLUMN api_key;
    END IF;
END $$;
