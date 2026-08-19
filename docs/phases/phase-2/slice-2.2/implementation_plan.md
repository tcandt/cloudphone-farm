# Phase 2 Slice 2.1: Enrollment / Token Key V2 Architecture

This plan establishes the technical foundation for Phase 2: Agent Enrollment. It defines the schema migration, key lifecycle, binding semantics, and API contracts required before any code implementation begins.

## Open Questions

> [!IMPORTANT]
> 1. **Unlimited Uses:** Should `max_uses` be allowed to be `NULL` or `-1` to indicate unlimited uses? (This is useful for bulk factory provisioning).
> 2. **Authentication Flow:** Should `POST /api/v2/agents/enroll` directly accept the `public_key` and bind it, or should it involve a multi-step cryptographic challenge?
> 3. **Token Naming:** Should we add a `name` or `label` column to `enrollment_tokens` so Admins can easily identify them in the UI?

## Proposed Changes

### 1. Database Schema Migration (000010_token_key_v2)

We will expand the existing `enrollment_tokens` table from a strictly single-use model to a multi-use, quota-based model, and link devices to their origin token.

#### [NEW] `backend/db/migrations/000010_token_key_v2.up.sql`
```sql
-- 1. Add Token Metadata and Quota
ALTER TABLE enrollment_tokens
  ADD COLUMN IF NOT EXISTS name VARCHAR(128),
  ADD COLUMN IF NOT EXISTS max_uses INT DEFAULT 1,
  ADD COLUMN IF NOT EXISTS current_uses INT DEFAULT 0;

-- 2. Track Token Provenance on Devices
ALTER TABLE device_agents
  ADD COLUMN IF NOT EXISTS enrolled_via_token_id VARCHAR(64) REFERENCES enrollment_tokens(token_id) ON DELETE SET NULL;
```

### 2. Token Lifecycle & Semantics

The `Token Key` will follow a strict state machine:
- **Active**: `expires_at` is in the future, `revoked_at` is NULL, and `current_uses < max_uses`.
- **Exhausted**: `current_uses >= max_uses`. The token is automatically rejected for new enrollments.
- **Expired**: `expires_at` has passed. Automatically rejected.
- **Revoked**: Admin manually sets `revoked_at = NOW()`.

**Binding & Revocation Semantics:**
- **Persistent Binding:** The Token Key is strictly for *initial enrollment*. Once enrolled, the device generates a local KeyPair and submits the `public_key`. All future communication (WebSockets, commands) is authenticated via cryptographic signatures against this public key, NOT the Token Key.
- **Decoupled Revocation:** Revoking a Token Key prevents *new* devices from enrolling. It **does not** disconnect or revoke devices that already successfully enrolled using that token. To disconnect a device, the Admin must revoke the device's specific `device_agents` identity.

### 3. API Contract

#### `POST /api/v2/agents/enroll`
This endpoint is called by the physical Android Agent on first boot.

**Request Payload:**
```json
{
  "enrollment_token": "raw-plaintext-token-string",
  "public_key": "base64-encoded-public-key",
  "device_info": {
    "model": "SM-G998B",
    "android_version": "13",
    "mac_address": "02:00:00:00:00:00",
    "serial_number": "R5CXXXXXXX"
  }
}
```

**Response Payload (200 OK):**
```json
{
  "device_id": "dev_xxxxx",
  "organization_id": "org_xxxxx",
  "assigned_group_id": "group_xxxxx",
  "status": "enrolled"
}
```

*Server Behavior:*
1. Hashes the `enrollment_token` and looks it up in the database.
2. Validates Lifecycle (Not expired, not revoked, `current_uses < max_uses`).
3. Creates a new `devices` row (or updates an existing one if a deterministic serial matching strategy is used).
4. Creates a `device_agents` row with the provided `public_key` and `enrolled_via_token_id`.
5. Increments `current_uses` on the `enrollment_tokens` row.

# Implementation Plan: Phase 2 Slice 2.2 - Agent Identity & Challenge Enrollment

This plan covers the durable Agent identity enrollment using the V2 `agent_key_bindings` mechanism, ECDSA P-256 challenges, and atomic quota enforcement.

## Proposed Changes

### 1. Database Migrations & Domain
#### [NEW] `backend/db/migrations/000011_agent_key_bindings.up.sql`
- Add `client_instance_id VARCHAR(128)` to `device_agents`.
- Create unique constraint on `device_agents(organization_id, client_instance_id)` where `client_instance_id IS NOT NULL`.
- Create composite unique constraint `uk_device_agents_composite` on `device_agents(organization_id, device_id, agent_id)`.
- Create `agent_key_bindings` table with `binding_id`, `key_id`, `organization_id`, `device_id`, `agent_id`, `public_key_fingerprint`, `bound_at`, `released_at`, `release_reason`.
- Add FKs and the `idx_agent_key_bindings_active_agent` unique constraint.

#### [NEW] `backend/db/migrations/000011_agent_key_bindings.down.sql`
- Revert the changes applied above.

#### [MODIFY] `backend/internal/domain/agent.go`
- Add `ClientInstanceID` to the `DeviceAgent` struct.

#### [NEW] `backend/internal/domain/agent_binding.go`
- Define `AgentKeyBinding` domain entity representing the new binding schema.

---

### 2. Service & Repository Layer
#### [MODIFY] `backend/internal/agentkey/repository.go`
- Add `CreateBindingTx(ctx, tx, binding, device, agent)` logic that handles the multi-table insert inside a transaction.
- Add methods for finding active bindings and verifying idempotency.

#### [NEW] `backend/internal/agentkey/enrollment_service.go` (or extending `service.go`)
- **`GenerateChallenge(ctx, req)`**:
  - Resolves `agent_enrollment_keys` by token prefix/hash.
  - Checks if key is revoked or expired.
  - Validates the ECDSA P-256 public key (X.509 SPKI DER -> Base64).
  - Computes `public_key_fingerprint`.
  - Generates a 32-byte CSPRNG nonce.
  - Stores `(key_id, organization_id, client_instance_id, fingerprint, nonce)` in Redis with a 120s TTL.
- **`EnrollAgent(ctx, req)`**:
  - Atomically `GETDEL` the challenge from Redis.
  - Verifies the ECDSA P-256 signature against the nonce.
  - Opens a PostgreSQL transaction (`tx`).
  - `SELECT FOR UPDATE` on the `agent_enrollment_keys`.
  - Checks active quota (`COUNT` bindings where `released_at IS NULL`) and enforces `max_bindings`.
  - Checks for idempotent replays (returns existing device/agent without consuming quota).
  - Handles `409 IDENTITY_CONFLICT` if `client_instance_id` matches but `public_key_fingerprint` differs.
  - Creates the new `device`, `device_agents`, and `agent_key_bindings` records.
  - Updates `last_used_at` on the `agent_enrollment_keys` record.

---

### 3. Transport & API Layer
#### [MODIFY] `backend/internal/transport/http/agentkey_handler.go` (or new handler)
- Expose `POST /api/v2/agents/enroll/challenge`.
- Expose `POST /api/v2/agents/enroll`.
- Wire these endpoints in `main.go` under the V2 routes without requiring authentication (they use the enrollment token for auth). Rate-limiting will be applied.

#### [MODIFY] `api/openapi.yaml`
- Add `/api/v2/agents/enroll/challenge` and `/api/v2/agents/enroll` schemas and endpoints.
- Update `oapi-codegen` generated models.

---

## Verification Plan

### Automated Tests
- **Migration Tests**: `TestMigration000011_AgentKeyBindings` in `agentkey_migration_test.go` to test constraints (FKs, unique bindings, down/up rollback).
- **Service/Handler Tests**:
  - Generate Challenge: Invalid token, expired/revoked key, invalid P-256 key.
  - Enroll: Invalid signature, expired challenge, challenge replay rejection.
  - Idempotency: Retry with fresh challenge yields `200 OK`, differing fingerprint yields `409 IDENTITY_CONFLICT`.
  - Quota: Exhaust quota, concurrent enrollment tests (`for update` lock ensures exactly 1 succeeds).
  - Validate that raw enrollment tokens are never saved to Redis.

### Manual Verification
- Execute `go test -race ./...`
- Execute `go build ./cmd/server`
- Confirm `databases` and Redis integrations pass without skipping.

> [!IMPORTANT]
> **Open Question:** Should the new V2 enrollment endpoints be handled inside the existing `AgentKeyHandler` (which currently manages keys via Admin Auth), or inside a new `AgentEnrollmentHandlerV2` to cleanly separate the Admin API from the Agent API (which is unauthenticated and relies on the enrollment token/challenge)? I plan to use a new `AgentEnrollmentHandlerV2` mounted under `/api/v2/agents/enroll`.

## Verification Plan

### Automated Tests
- DB migration tests to ensure `000010` applies cleanly on top of `000009`.
- Backend unit tests for `POST /api/v2/agents/enroll` asserting:
  - Valid token creates device and increments `current_uses`.
  - Exhausted token returns `403 Forbidden`.
  - Revoked token returns `403 Forbidden`.
  - Expired token returns `403 Forbidden`.
