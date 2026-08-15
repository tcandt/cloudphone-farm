-- Revert Nullable Physical Telemetry & Keystore Security Metadata Authority

ALTER TABLE devices
    DROP COLUMN IF EXISTS key_protection;

ALTER TABLE device_agents
    DROP COLUMN IF EXISTS key_protection;

ALTER TABLE device_heartbeats
    ALTER COLUMN cpu_usage SET NOT NULL,
    ALTER COLUMN cpu_usage SET DEFAULT 0,
    ALTER COLUMN memory_usage SET NOT NULL,
    ALTER COLUMN memory_usage SET DEFAULT 0,
    ALTER COLUMN battery_level SET NOT NULL,
    ALTER COLUMN battery_level SET DEFAULT 100,
    ALTER COLUMN temperature_c SET NOT NULL,
    ALTER COLUMN temperature_c SET DEFAULT 0,
    ALTER COLUMN network_type SET NOT NULL,
    ALTER COLUMN network_type SET DEFAULT 'wifi';
