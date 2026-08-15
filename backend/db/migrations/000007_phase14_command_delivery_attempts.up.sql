-- Phase 1.4: Command Delivery Attempts & Generation Fencing Tracking

CREATE TABLE IF NOT EXISTS command_delivery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id VARCHAR(64) NOT NULL,
    command_id VARCHAR(64) NOT NULL REFERENCES commands(command_id) ON DELETE CASCADE,
    device_id VARCHAR(64) NOT NULL,
    attempt_no INT NOT NULL DEFAULT 1,
    agent_id VARCHAR(64) NOT NULL,
    connection_id VARCHAR(64) NOT NULL,
    generation BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'dispatched',
    dispatched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    failed_at TIMESTAMPTZ,
    failure_reason TEXT,
    CONSTRAINT unique_command_attempt UNIQUE (command_id, attempt_no)
);

CREATE INDEX IF NOT EXISTS idx_command_delivery_attempts_cmd ON command_delivery_attempts(command_id, attempt_no DESC);

INSERT INTO schema_migrations (version, dirty) VALUES (7, false) ON CONFLICT (version) DO UPDATE SET dirty = false;
