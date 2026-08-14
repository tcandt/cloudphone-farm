-- Rollback Migration 000005

DROP INDEX IF EXISTS idx_commands_status;

ALTER TABLE commands
DROP COLUMN IF EXISTS expires_at,
DROP COLUMN IF EXISTS executed_at,
DROP COLUMN IF EXISTS error_message,
DROP COLUMN IF EXISTS last_status_sequence;
