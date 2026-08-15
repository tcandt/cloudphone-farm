-- Migration 000005: Harden Command Schema for Runtime Persistence & Sequence Tracking

ALTER TABLE commands
ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS executed_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS error_message TEXT,
ADD COLUMN IF NOT EXISTS last_status_sequence BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_commands_status ON commands(status);

INSERT INTO pcp_schema_migrations (version, name, checksum) VALUES (5, '000005_harden_command_runtime.up.sql', 'v5_runtime') ON CONFLICT (version) DO UPDATE SET name = EXCLUDED.name;
