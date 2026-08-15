-- Phase 1.2.1 Core Database Migration (19 Tables with Composite Tenant Constraints & Security Standards)

-- 1. Organizations (Tenant Boundary)
CREATE TABLE IF NOT EXISTS organizations (
    organization_id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(128) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. Users (Global Identity)
CREATE TABLE IF NOT EXISTS users (
    user_id VARCHAR(64) PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 3. Organization Memberships
CREATE TABLE IF NOT EXISTS organization_memberships (
    membership_id VARCHAR(64) PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(organization_id) ON DELETE CASCADE,
    user_id VARCHAR(64) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_org_user UNIQUE (organization_id, user_id)
);

-- 4. Roles (System roles have organization_id NULL)
CREATE TABLE IF NOT EXISTS roles (
    role_id VARCHAR(64) PRIMARY KEY,
    organization_id VARCHAR(64) REFERENCES organizations(organization_id) ON DELETE CASCADE,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    CONSTRAINT uk_org_role_code UNIQUE (organization_id, code)
);

-- 5. System Permissions (Immutable Permission Codes)
CREATE TABLE IF NOT EXISTS permissions (
    permission_code VARCHAR(64) PRIMARY KEY,
    category VARCHAR(64) NOT NULL,
    description TEXT NOT NULL
);

-- 6. Role Permissions Junction
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id VARCHAR(64) NOT NULL REFERENCES roles(role_id) ON DELETE CASCADE,
    permission_code VARCHAR(64) NOT NULL REFERENCES permissions(permission_code) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_code)
);

-- 7. User Roles (Membership <-> Role Mapping)
CREATE TABLE IF NOT EXISTS user_roles (
    membership_id VARCHAR(64) NOT NULL REFERENCES organization_memberships(membership_id) ON DELETE CASCADE,
    role_id VARCHAR(64) NOT NULL REFERENCES roles(role_id) ON DELETE CASCADE,
    PRIMARY KEY (membership_id, role_id)
);

-- 8. Devices (Physical / Cloud Android Device Registry)
CREATE TABLE IF NOT EXISTS devices (
    device_id VARCHAR(64) NOT NULL,
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(organization_id) ON DELETE CASCADE,
    group_id VARCHAR(64),
    name VARCHAR(255) NOT NULL,
    serial_number VARCHAR(128) NOT NULL,
    model VARCHAR(128) NOT NULL,
    platform_version VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'provisioning',
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (device_id),
    CONSTRAINT uk_org_device UNIQUE (organization_id, device_id)
);

-- 9. Device Agents
CREATE TABLE IF NOT EXISTS device_agents (
    agent_id VARCHAR(64) PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    device_id VARCHAR(64) NOT NULL,
    apk_version VARCHAR(32) NOT NULL,
    protocol_version VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    registered_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_agent_device FOREIGN KEY (organization_id, device_id) REFERENCES devices(organization_id, device_id) ON DELETE CASCADE
);

-- 10. Device Heartbeats
CREATE TABLE IF NOT EXISTS device_heartbeats (
    heartbeat_id BIGSERIAL PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    device_id VARCHAR(64) NOT NULL,
    cpu_usage REAL NOT NULL DEFAULT 0,
    memory_usage REAL NOT NULL DEFAULT 0,
    battery_level INT NOT NULL DEFAULT 100,
    temperature_c REAL NOT NULL DEFAULT 0,
    network_type VARCHAR(32) NOT NULL DEFAULT 'wifi',
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_heartbeat_device FOREIGN KEY (organization_id, device_id) REFERENCES devices(organization_id, device_id) ON DELETE CASCADE
);

-- 11. Control Leases (Historical Audit; Redis is Realtime Authority)
CREATE TABLE IF NOT EXISTS control_leases (
    control_lease_id VARCHAR(64) PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    device_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL REFERENCES users(user_id),
    fencing_token BIGINT NOT NULL DEFAULT 1,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CONSTRAINT fk_lease_device FOREIGN KEY (organization_id, device_id) REFERENCES devices(organization_id, device_id) ON DELETE CASCADE
);

-- 12. Commands
CREATE TABLE IF NOT EXISTS commands (
    command_id VARCHAR(64) PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    device_id VARCHAR(64) NOT NULL,
    actor_id VARCHAR(64) NOT NULL REFERENCES users(user_id),
    command_type VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    idempotency_key VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_command_device FOREIGN KEY (organization_id, device_id) REFERENCES devices(organization_id, device_id) ON DELETE CASCADE,
    CONSTRAINT uk_org_actor_idempotency UNIQUE (organization_id, actor_id, idempotency_key)
);

-- 13. Command Events
CREATE TABLE IF NOT EXISTS command_events (
    event_id BIGSERIAL PRIMARY KEY,
    command_id VARCHAR(64) NOT NULL REFERENCES commands(command_id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 14. Command Outbox (Transactional Outbox Pattern for WS Delivery)
CREATE TABLE IF NOT EXISTS command_outbox (
    outbox_id BIGSERIAL PRIMARY KEY,
    command_id VARCHAR(64) NOT NULL REFERENCES commands(command_id) ON DELETE CASCADE,
    organization_id VARCHAR(64) NOT NULL,
    device_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMPTZ
);

-- 15. Proxies (Encrypted Credentials at Rest)
CREATE TABLE IF NOT EXISTS proxies (
    proxy_id VARCHAR(64) PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(organization_id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'http',
    host VARCHAR(255) NOT NULL,
    port INT NOT NULL,
    ciphertext BYTEA,
    nonce BYTEA,
    key_version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 16. Proxy Assignments
CREATE TABLE IF NOT EXISTS proxy_assignments (
    assignment_id VARCHAR(64) PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    device_id VARCHAR(64) NOT NULL,
    proxy_id VARCHAR(64) NOT NULL REFERENCES proxies(proxy_id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_proxy_device FOREIGN KEY (organization_id, device_id) REFERENCES devices(organization_id, device_id) ON DELETE CASCADE,
    CONSTRAINT uk_device_proxy UNIQUE (organization_id, device_id)
);

-- 17. Enrollment Tokens (Stored as SHA-256 Token Hashes)
CREATE TABLE IF NOT EXISTS enrollment_tokens (
    token_id VARCHAR(64) PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(organization_id) ON DELETE CASCADE,
    bound_group_id VARCHAR(64),
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    created_by VARCHAR(64) NOT NULL REFERENCES users(user_id),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ
);

-- 18. Historical Sessions
CREATE TABLE IF NOT EXISTS sessions (
    session_id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

-- 19. Audit Logs
CREATE TABLE IF NOT EXISTS audit_logs (
    audit_id BIGSERIAL PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL REFERENCES organizations(organization_id) ON DELETE CASCADE,
    actor_id VARCHAR(64) NOT NULL REFERENCES users(user_id),
    correlation_id VARCHAR(128) NOT NULL,
    action VARCHAR(128) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(64) NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Essential Performance & Tenant Isolation Indexes
CREATE INDEX IF NOT EXISTS idx_devices_org_status ON devices(organization_id, status);
CREATE INDEX IF NOT EXISTS idx_heartbeats_device_received ON device_heartbeats(device_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_commands_device_created ON commands(device_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_outbox_status_created ON command_outbox(status, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_org_created ON audit_logs(organization_id, created_at DESC);

-- 20. Migration Authority Tracking Table
CREATE TABLE IF NOT EXISTS pcp_schema_migrations (
    version BIGINT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    checksum VARCHAR(64) NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO pcp_schema_migrations (version, name, checksum) VALUES (1, '000001_create_core_tables.up.sql', 'v1_core') ON CONFLICT (version) DO UPDATE SET name = EXCLUDED.name;
