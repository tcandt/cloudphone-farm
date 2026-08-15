-- Phase 1.7 Nullable Physical Telemetry & Keystore Security Metadata Authority

ALTER TABLE device_heartbeats
    ALTER COLUMN cpu_usage DROP NOT NULL,
    ALTER COLUMN cpu_usage DROP DEFAULT,
    ALTER COLUMN memory_usage DROP NOT NULL,
    ALTER COLUMN memory_usage DROP DEFAULT,
    ALTER COLUMN battery_level DROP NOT NULL,
    ALTER COLUMN battery_level DROP DEFAULT,
    ALTER COLUMN temperature_c DROP NOT NULL,
    ALTER COLUMN temperature_c DROP DEFAULT,
    ALTER COLUMN network_type DROP NOT NULL,
    ALTER COLUMN network_type DROP DEFAULT;

ALTER TABLE device_agents
    ADD COLUMN IF NOT EXISTS key_protection JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS key_protection JSONB NOT NULL DEFAULT '{}'::jsonb;
