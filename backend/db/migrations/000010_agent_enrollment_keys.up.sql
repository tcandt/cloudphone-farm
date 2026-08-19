-- Migration 000010: Agent Enrollment Keys (V2)

CREATE TABLE agent_enrollment_keys (
    key_id              VARCHAR(64) PRIMARY KEY,
    
    organization_id     VARCHAR(64) NOT NULL 
        REFERENCES organizations(organization_id) ON DELETE CASCADE,
    created_by          VARCHAR(64) NOT NULL 
        REFERENCES users(user_id),

    name                VARCHAR(128) NOT NULL,

    token_hash          VARCHAR(64) NOT NULL UNIQUE,
    token_prefix        VARCHAR(32) NOT NULL,

    max_bindings        INT NULL,
    expires_at          TIMESTAMPTZ NULL,
    revoked_at          TIMESTAMPTZ NULL,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at        TIMESTAMPTZ,

    CONSTRAINT chk_agent_enrollment_keys_max_bindings CHECK (max_bindings IS NULL OR max_bindings > 0)
);

-- Unique authority index to allow composite FK referencing from agent_key_bindings in Slice 2.2
CREATE UNIQUE INDEX uq_agent_enrollment_keys_org_key 
ON agent_enrollment_keys(organization_id, key_id);
