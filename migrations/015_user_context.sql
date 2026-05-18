-- Migration: add user_context table for persistent cross-session memory
-- Created: 2026-05-17

CREATE TABLE IF NOT EXISTS user_context (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value JSONB NOT NULL,
    source TEXT CHECK (source IN ('explicit', 'inferred')),
    confidence FLOAT CHECK (confidence BETWEEN 0 AND 1),
    updated_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, key)
);

CREATE INDEX idx_user_context_user_id ON user_context(user_id);
