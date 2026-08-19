# Phase 2: Enrollment & Token Key V2 Architecture

This document defines the architectural blueprint for Phase 2. It explicitly separates V2 Token Key semantics from the legacy system by introducing new schemas, enforcing a multi-step cryptographic enrollment flow, and establishing clear idempotency and quota boundaries.

## 1. Database Schema Design (V2 Separate from V1)

We will **not** modify the legacy `enrollment_tokens` table. Instead, we introduce two new tables to cleanly encapsulate V2 semantics, quotas, and provenance tracking.

### `000010_agent_enrollment_keys.up.sql`
Defines the multi-use Token Keys.
```sql
CREATE TABLE agent_enrollment_keys (
    key_id              VARCHAR(64) PRIMARY KEY,
    organization_id     VARCHAR(64) NOT NULL,
    created_by          VARCHAR(64) NOT NULL,

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
Serves as the authoritative source of truth for active quota usage and device provenance.
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
    release_reason         VARCHAR(128) NULL
);
```
**Quota Enforcement**: Active bindings are calculated as `COUNT(*) WHERE released_at IS NULL`.
- To prevent race conditions, the enrollment transaction must lock the key row (`SELECT ... FOR UPDATE`) before counting and inserting.
- Revoking the Token Key does **not** release bindings.
- Devices going "offline" do **not** release bindings.

## 2. Authentication Flow & Cryptographic Challenge

We will not blindly accept a public key sent alongside a token. The Agent must prove possession of the corresponding private key via a multi-step challenge.

**Crypto Contract:**
- **KeyPair**: ECDSA P-256 generated in AndroidKeyStore.
- **Signature**: SHA256withECDSA.
- **Transport**: X.509 SubjectPublicKeyInfo DER → Base64.
- **Fingerprint**: SHA-256(SPKI DER).

**Flow:**
1. **Agent generates KeyPair** locally.
2. `POST /api/v2/agents/enroll/challenge` 
   - Server returns a short-lived, one-time cryptographic challenge (stored in Redis, does NOT consume quota).
3. **Agent signs the challenge** with its private key.
4. `POST /api/v2/agents/enroll` 
   - Sends the raw token, `public_key`, and the signed challenge.
5. **Server validates** the signature, verifies the token hash, runs a transactional quota check, and creates the device + binding.

## 3. API Contracts

### Admin Token Key Management APIs
These endpoints manage the lifecycle of Token Keys.
- `POST /api/v2/agent-keys`: Creates a new key. **Returns the raw secret Token Key one time only.**
- `GET /api/v2/agent-keys`: Lists keys. Never returns the raw secret.
- `GET /api/v2/agent-keys/{key_id}`: Retrieves key details.
- `PATCH /api/v2/agent-keys/{key_id}`: Updates properties (e.g., name).
- `DELETE /api/v2/agent-keys/{key_id}`: Semantic revocation (`revoked_at = NOW()`). Does not hard-delete, does not sever existing bindings.
- `GET /api/v2/agent-keys/{key_id}/devices`: Lists all active/historical bindings.

### Agent Enrollment Endpoint (`POST /api/v2/agents/enroll`)
Android serial numbers and MAC addresses are explicitly ignored for identity matching due to instability/randomization. Identity is established by the `client_instance_id` and the `public_key_fingerprint`.

**Request Payload:**
```json
{
  "enrollment_token": "raw-plaintext-token-string",
  "client_instance_id": "uuid-generated-locally-on-agent",
  "public_key": "base64-encoded-public-key",
  "challenge_id": "xxx",
  "signature": "base64-encoded-signature",
  "device_info": {
    "manufacturer": "Samsung",
    "model": "SM-G930F",
    "android_version": "8.0",
    "sdk_int": 26,
    "agent_version": "1.0.0",
    "protocol_version": "1.0"
  }
}
```

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

## 4. Idempotency & Conflict Rules
Enrollment operations must be strictly idempotent to handle network drops.
- **Idempotent Replay**: If an Agent retries enrollment with the *exact same* `client_instance_id` and *same* `public_key`, the server returns the existing `device_id` and `agent_id` (200 OK) without consuming an additional quota binding.
- **Identity Conflict**: If an Agent attempts to enroll with the *same* `client_instance_id` but a *different* `public_key`, the server rejects it (`409 IDENTITY_CONFLICT`). Silent identity rotation is forbidden.

## 5. Mandatory Verification Tests
The following specific scenarios must be verified in integration tests:
- **Concurrency**: 2 concurrent enrollments for a token with 1 remaining slot -> Exactly 1 succeeds.
- **Idempotency**: Retrying the same identity -> No duplicate device, no extra quota consumption.
- **Revocation Decoupling**: Revoking an enrollment key -> Rejects new enrollments, existing bound Agents remain valid.
- **Quota Retention**: Devices marked offline -> Quota remains occupied.
- **Null Handling**: `expires_at IS NULL` -> Never expires. `max_bindings IS NULL` -> Unlimited uses.
- **Crypto Validation**: Invalid challenge signature -> Rejected. Replayed challenge -> Rejected.
- **Identity Enforcement**: Same `client_instance_id` + different `public_key` -> 409 Conflict.
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
