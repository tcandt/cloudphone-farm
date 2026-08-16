-- Migration 000009: Rollback created_at column on enrollment_tokens
ALTER TABLE enrollment_tokens
DROP COLUMN IF EXISTS created_at;
