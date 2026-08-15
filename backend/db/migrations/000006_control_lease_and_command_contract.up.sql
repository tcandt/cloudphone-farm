-- Migration 000006: Add device.control.acquire Permission & Persistent Fencing Counter Table

INSERT INTO permissions (permission_code, category, description)
VALUES ('device.control.acquire', 'device', 'Acquire exclusive realtime control lease on a device')
ON CONFLICT (permission_code) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_code)
VALUES 
    ('role_sys_owner', 'device.control.acquire'),
    ('role_sys_operator', 'device.control.acquire')
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS device_control_fences (
    organization_id VARCHAR(64) NOT NULL,
    device_id VARCHAR(64) NOT NULL,
    last_fencing_token BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (organization_id, device_id),
    FOREIGN KEY (organization_id, device_id) REFERENCES devices(organization_id, device_id) ON DELETE CASCADE
);

INSERT INTO pcp_schema_migrations (version, name, checksum) VALUES (6, '000006_control_lease_and_command_contract.up.sql', 'v6_fences') ON CONFLICT (version) DO UPDATE SET name = EXCLUDED.name;
