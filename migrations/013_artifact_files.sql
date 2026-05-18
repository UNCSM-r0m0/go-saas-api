-- 013_artifact_files.sql
-- Add multi-file support for artifacts

-- Add entry_file column to artifacts
ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS entry_file TEXT;

-- Make content nullable (for multi-file projects)
ALTER TABLE artifacts ALTER COLUMN content DROP NOT NULL;

-- Add is_complete flag for tracking generation status
ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS is_complete BOOLEAN NOT NULL DEFAULT true;

-- Create artifact_files table for multi-file projects
CREATE TABLE IF NOT EXISTS artifact_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    artifact_id UUID NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    language TEXT,
    content TEXT NOT NULL,
    file_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(artifact_id, path)
);

-- Index for faster lookups
CREATE INDEX IF NOT EXISTS idx_artifact_files_artifact_id ON artifact_files(artifact_id);

-- Update existing single-file artifacts to set entry_file
UPDATE artifacts SET entry_file = name WHERE entry_file IS NULL;
