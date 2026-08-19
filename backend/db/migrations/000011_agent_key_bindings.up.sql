-- Migration 000011: Agent Key Bindings

ALTER TABLE device_agents
ADD COLUMN IF NOT EXISTS client_instance_id VARCHAR(64);

CREATE UNIQUE INDEX IF NOT EXISTS uq_device_agents_org_client_instance
ON device_agents(organization_id, client_instance_id)
WHERE client_instance_id IS NOT NULL;

-- Ensure uniqueness on (organization_id, device_id, agent_id) so it can be referenced
ALTER TABLE device_agents
DROP CONSTRAINT IF EXISTS uq_device_agents_org_device_agent;

ALTER TABLE device_agents
ADD CONSTRAINT uq_device_agents_org_device_agent UNIQUE (organization_id, device_id, agent_id);


CREATE TABLE IF NOT EXISTS agent_key_bindings (
    binding_id VARCHAR(64) PRIMARY KEY,
    organization_id VARCHAR(64) NOT NULL,
    key_id VARCHAR(64) NOT NULL,
    device_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    public_key_fingerprint VARCHAR(64) NOT NULL,
    bound_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    released_at TIMESTAMPTZ,
    release_reason VARCHAR(128),

    CONSTRAINT fk_akb_key FOREIGN KEY (organization_id, key_id)
      REFERENCES agent_enrollment_keys(organization_id, key_id),

    CONSTRAINT fk_akb_agent FOREIGN KEY (organization_id, device_id, agent_id)
      REFERENCES device_agents(organization_id, device_id, agent_id),

    CONSTRAINT chk_akb_fp CHECK (
      public_key_fingerprint ~ '^[0-9a-f]{64}$'
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_key_bindings_active_agent
ON agent_key_bindings(organization_id, agent_id)
WHERE released_at IS NULL;
