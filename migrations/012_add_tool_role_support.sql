-- 012_add_tool_role_support.sql
-- Add columns for agentic loop: tool_call_id and tool_name on messages

ALTER TABLE messages ADD COLUMN IF NOT EXISTS tool_call_id TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS tool_name TEXT;

CREATE INDEX IF NOT EXISTS idx_messages_tool_call_id ON messages(tool_call_id) WHERE tool_call_id IS NOT NULL;