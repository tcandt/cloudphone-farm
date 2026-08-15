-- Migration 000003: Harden Agent Cryptographic Identity and Enrollment Tokens

ALTER TABLE device_agents
ADD COLUMN IF NOT EXISTS public_key BYTEA,
ADD COLUMN IF NOT EXISTS public_key_fingerprint VARCHAR(128),
ADD COLUMN IF NOT EXISTS credential_version INT NOT NULL DEFAULT 1,
ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS last_authenticated_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_device_agents_pk_fp ON device_agents(public_key_fingerprint) WHERE public_key_fingerprint IS NOT NULL;

ALTER TABLE enrollment_tokens
ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS consumed_at TIMESTAMPTZ;

INSERT INTO pcp_schema_migrations (version, name, checksum) VALUES (3, '000003_harden_agent_identity_and_enrollment.up.sql', 'v3_identity') ON CONFLICT (version) DO UPDATE SET name = EXCLUDED.name;
