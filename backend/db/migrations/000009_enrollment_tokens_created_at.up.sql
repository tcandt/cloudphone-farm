-- Migration 000009: Add created_at column to enrollment_tokens
ALTER TABLE enrollment_tokens
ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;
