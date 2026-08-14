-- Rollback Migration 000004

DROP INDEX IF EXISTS idx_command_outbox_pending;

ALTER TABLE command_outbox
DROP COLUMN IF EXISTS attempt_count,
DROP COLUMN IF EXISTS next_attempt_at,
DROP COLUMN IF EXISTS last_attempt_at,
DROP COLUMN IF EXISTS last_error,
DROP COLUMN IF EXISTS dispatched_at,
DROP COLUMN IF EXISTS locked_at,
DROP COLUMN IF EXISTS locked_by;
