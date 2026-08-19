-- Revert Migration 000011: Agent Key Bindings

DROP INDEX IF EXISTS uq_agent_key_bindings_active_agent;

DROP TABLE IF EXISTS agent_key_bindings;

ALTER TABLE device_agents
DROP CONSTRAINT IF EXISTS uq_device_agents_org_device_agent;

DROP INDEX IF EXISTS uq_device_agents_org_client_instance;

ALTER TABLE device_agents
DROP COLUMN IF EXISTS client_instance_id;
