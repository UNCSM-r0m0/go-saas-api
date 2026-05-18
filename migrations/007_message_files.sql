-- 007_message_files.sql — Add file_ids to messages

ALTER TABLE messages ADD COLUMN IF NOT EXISTS file_ids UUID[] DEFAULT '{}';
