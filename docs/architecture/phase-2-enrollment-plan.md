# Phase 2: Enrollment & Token Key V2 Architecture

This document defines the architectural blueprint for Phase 2. It explicitly separates V2 Token Key semantics from the legacy system by introducing new schemas, enforcing a multi-step cryptographic enrollment flow, and establishing clear idempotency and quota boundaries.

## 1. Database Schema Design (V2 Separate from V1)

We will **not** modify the legacy `enrollment_tokens` table. Instead, we introduce two new tables to cleanly encapsulate V2 semantics, quotas, and provenance tracking, while extending `device_agents` to track the local agent instance identity.

### `000010_agent_enrollment_keys.up.sql`
Defines the multi-use Token Keys with tenant-safe foreign keys.
```sql
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

    CHECK (max_bindings IS NULL OR max_bindings > 0)
);
```
*Note: `max_bindings IS NULL` means unlimited. `expires_at IS NULL` means it never expires. No sentinel values like `-1` or `9999` are used.*

### `000011_agent_key_bindings.up.sql`
Establishes the durable local identity in `device_agents` (without touching `000003`) and serves as the authoritative source of truth for active quota usage and device provenance.

**1. Persist `client_instance_id` and Unique Composite Identity:**
```sql
ALTER TABLE device_agents
ADD COLUMN client_instance_id VARCHAR(64);

-- Authority for idempotent replay & conflict detection
CREATE UNIQUE INDEX uq_device_agents_org_client_instance
ON device_agents (organization_id, client_instance_id)
WHERE client_instance_id IS NOT NULL;

-- Authority for safe foreign key bindings
CREATE UNIQUE INDEX uq_device_agents_org_device_agent
ON device_agents (organization_id, device_id, agent_id);
```

**2. Key Bindings Table:**
```sql
CREATE TABLE agent_key_bindings (
    binding_id             VARCHAR(64) PRIMARY KEY,
    key_id                 VARCHAR(64) NOT NULL REFERENCES agent_enrollment_keys(key_id),
    
    organization_id        VARCHAR(64) NOT NULL,
    device_id              VARCHAR(64) NOT NULL,
    agent_id               VARCHAR(64) NOT NULL,
    public_key_fingerprint VARCHAR(128) NOT NULL,
    
    bound_at               TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    released_at            TIMESTAMPTZ NULL,
    release_reason         VARCHAR(128) NULL,
    
    -- Tenant and composite identity safety
    FOREIGN KEY (organization_id, device_id, agent_id)
        REFERENCES device_agents(organization_id, device_id, agent_id)
);

-- Active-binding invariant: One Agent may consume at most ONE active binding
CREATE UNIQUE INDEX uq_agent_key_bindings_active_agent
ON agent_key_bindings (organization_id, agent_id)
WHERE released_at IS NULL;
```
**Quota Enforcement**: Active bindings are calculated as `COUNT(*) WHERE released_at IS NULL`.
- To prevent race conditions, the enrollment transaction must lock the key row (`SELECT ... FOR UPDATE`) before counting and inserting.
- Revoking the Token Key does **not** release bindings.
- Devices going "offline" do **not** release bindings.

## 2. Authentication Flow & Cryptographic Challenge

We will not blindly accept a public key sent alongside a token. The Agent must prove possession of the corresponding private key via a multi-step challenge. Both endpoints will be rate-limited.

**Cryptographic Encoding:**
- **Token Secret**: >=256 bits CSPRNG entropy.
- **KeyPair**: ECDSA P-256 generated in AndroidKeyStore.
- **Signature**: SHA256withECDSA.
- **Public Key**: X.509 SubjectPublicKeyInfo (SPKI) DER → Base64.
- **Fingerprint**: lowercase hex SHA-256(SPKI DER) -> strictly 64 characters.

**Flow:**
1. **Agent generates KeyPair** locally.
2. `POST /api/v2/agents/enroll/challenge` 
   - **Request Payload**: `{ enrollment_token, client_instance_id, public_key }`
   - Server validates the key lifecycle, creates a 32-byte CSPRNG nonce and a >=128-bit `challenge_id`.
   - **Consumes NO quota.**
   - Stores in Redis (TTL = 120s): `{ key_id, organization_id, client_instance_id, public_key_fingerprint, nonce }`. Raw enrollment secrets are NEVER stored in Redis.
   - **Response**: `{ challenge_id, challenge, expires_in }`
3. **Agent signs the challenge** (`base64url-nonce`) with its private key.
4. `POST /api/v2/agents/enroll` 
   - Final enroll must **atomically consume** the challenge using Redis `GETDEL` (or equivalent one-time atomic operation).
   - Server must verify against the challenge data:
     - `challenge.key_id == resolved token key`
     - `challenge.client_instance_id == request.client_instance_id`
     - `challenge.public_key_fingerprint == fingerprint(request.public_key)`
   - Server verifies the ECDSA signature.
5. Server validates quota transactionally and creates/returns device and binding.

## 3. Idempotency & Conflict Rules
V2 enrollment requires `client_instance_id` for durable authority handling.
- **Idempotent Replay**: If an Agent retries enrollment with the *exact same* `client_instance_id` and *same* `public_key_fingerprint`, the server returns the existing `device_id` and `agent_id` (200 OK) without consuming an additional quota binding.
- **Identity Conflict**: If an Agent attempts to enroll with the *same* `client_instance_id` but a *different* `public_key_fingerprint`, the server rejects it (`409 IDENTITY_CONFLICT`). Silent identity rotation is forbidden.

## 4. API Contracts

### Admin Token Key Management APIs
These endpoints manage the lifecycle of Token Keys.
- `POST /api/v2/agent-keys`: Creates a new key. **Returns the raw secret Token Key one time only.** The raw secret is never logged, stored plaintext, or returned by GET.
- `GET /api/v2/agent-keys`: Lists keys.
- `GET /api/v2/agent-keys/{key_id}`: Retrieves key details.
- `PATCH /api/v2/agent-keys/{key_id}`: May update **ONLY** `name`, `max_bindings`, and `expires_at`. `token_hash`, `organization_id`, `created_by`, and `revoked_at` are immutable.
  - *Note*: Reducing `max_bindings` below current active binding count does NOT revoke existing Agents; it simply prevents new enrollment until capacity exists.
- `DELETE /api/v2/agent-keys/{key_id}`: **Irreversible semantic revoke** in Phase 2. Sets `revoked_at = NOW()`. Does not hard-delete, does not sever existing bindings. `PATCH` cannot set `revoked_at = NULL`.
- `GET /api/v2/agent-keys/{key_id}/devices`: Lists all active/historical bindings.

### Agent Enrollment Endpoint (`POST /api/v2/agents/enroll`)
**Response Payload (201 Created or 200 OK):**
```json
{
  "device_id": "dev_xxx",
  "agent_id": "agt_xxx",
  "organization_id": "org_xxx",
  "credential_version": 1,
  "status": "enrolled"
}
```
*Note: `assigned_group_id` is excluded from Phase 2 until group binding semantics are implemented.*

## 5. Mandatory Verification Tests
The following specific scenarios must be verified in integration tests:
- **Concurrency**: 2 concurrent enrollments for a token with 1 remaining slot -> Exactly 1 succeeds.
- **Idempotency**: Retrying the same identity -> No duplicate device, no extra quota consumption.
- **Identity Enforcement**: Same `client_instance_id` + different `public_key` -> 409 Conflict.
- **Revocation Decoupling**: Revoking an enrollment key -> Rejects new enrollments, existing bound Agents remain valid.
- **Quota Retention**: Devices marked offline -> Quota remains occupied.
- **Null Handling**: `expires_at IS NULL` -> Never expires. `max_bindings IS NULL` -> Unlimited uses.
- **Crypto Validation**: Invalid challenge signature -> Rejected. Replayed challenge -> Rejected.
- **Secret Security**: Raw enrollment secret is never returned from GET or stored/logged.

## 6. Implementation Slices
To ensure strict verification boundaries, Phase 2 will be implemented in three slices:

**Slice 2.1: Schema + Admin API**
- Implement `000010_agent_enrollment_keys` migration.
- Implement Admin `agent-keys` CRUD, repository, and service.
- *Owner Gate Check*

**Slice 2.2: Enrollment Domain & API**
- Implement `000011_agent_key_bindings` migration.
- Implement Challenge generation/verification and `POST /api/v2/agents/enroll`.
- Implement atomic quota constraints and idempotency validation.
- *Owner Gate Check*

**Slice 2.3: Admin UI**
- Connect the Admin Token Keys page to the real API.
- Implement Key creation, revocation, quota displays, and bindings view.
- *Phase 2 Final Gate Check*
