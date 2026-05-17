-- Migration: Add artifact_id column to messages table
-- Run this in your PostgreSQL database

ALTER TABLE messages 
ADD COLUMN IF NOT EXISTS artifact_id UUID REFERENCES artifacts(id) ON DELETE SET NULL;

-- Create index for faster lookups
CREATE INDEX IF NOT EXISTS idx_messages_artifact_id ON messages(artifact_id);
