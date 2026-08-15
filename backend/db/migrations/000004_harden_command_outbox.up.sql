-- Migration 000004: Harden Command Outbox Table for Reliable Delivery & Retries

ALTER TABLE command_outbox
ADD COLUMN IF NOT EXISTS attempt_count INT NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS last_attempt_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS last_error TEXT,
ADD COLUMN IF NOT EXISTS dispatched_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS locked_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS locked_by VARCHAR(128);

CREATE INDEX IF NOT EXISTS idx_command_outbox_pending
ON command_outbox(status, next_attempt_at, created_at)
WHERE status = 'pending';
