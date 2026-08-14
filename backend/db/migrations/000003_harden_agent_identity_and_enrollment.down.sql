-- Rollback Migration 000003

DROP INDEX IF EXISTS idx_device_agents_pk_fp;

ALTER TABLE device_agents
DROP COLUMN IF EXISTS public_key,
DROP COLUMN IF EXISTS public_key_fingerprint,
DROP COLUMN IF EXISTS credential_version,
DROP COLUMN IF EXISTS revoked_at,
DROP COLUMN IF EXISTS last_authenticated_at;

ALTER TABLE enrollment_tokens
DROP COLUMN IF EXISTS revoked_at,
DROP COLUMN IF EXISTS consumed_at;
