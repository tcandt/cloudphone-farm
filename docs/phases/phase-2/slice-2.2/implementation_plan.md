# Implementation Plan: Phase 2 Slice 2.2 - Agent Identity & Challenge Enrollment

This document specifies the exact architecture for durable Agent identity enrollment, ECDSA P-256 challenges, and atomic quota enforcement. It replaces all prior legacy agent matching strategies.

## Proposed Changes

### 1. Database Migrations & Domain

#### [NEW] `backend/db/migrations/000011_agent_key_bindings.up.sql`
Add the new schema correctly enforcing the composite uniqueness and binding limits. Do NOT touch migrations 000001 through 000010.

```sql
ALTER TABLE device_agents
ADD COLUMN client_instance_id VARCHAR(64);

CREATE UNIQUE INDEX uq_device_agents_org_client_instance
ON device_agents(organization_id, client_instance_id)
WHERE client_instance_id IS NOT NULL;

CREATE UNIQUE INDEX uq_device_agents_org_device_agent
ON device_agents(organization_id, device_id, agent_id);

CREATE TABLE agent_key_bindings (
    binding_id VARCHAR(64) PRIMARY KEY,

    organization_id VARCHAR(64) NOT NULL,
    key_id VARCHAR(64) NOT NULL,

    device_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,

    public_key_fingerprint VARCHAR(64) NOT NULL,

    bound_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    released_at TIMESTAMPTZ,
    release_reason VARCHAR(128),

    FOREIGN KEY (organization_id, key_id)
      REFERENCES agent_enrollment_keys(organization_id, key_id),

    FOREIGN KEY (organization_id, device_id, agent_id)
      REFERENCES device_agents(organization_id, device_id, agent_id),

    CHECK (
      public_key_fingerprint ~ '^[0-9a-f]{64}$'
    )
);

CREATE UNIQUE INDEX uq_agent_key_bindings_active_agent
ON agent_key_bindings(organization_id, agent_id)
WHERE released_at IS NULL;
```

#### [NEW] `backend/db/migrations/000011_agent_key_bindings.down.sql`
- Rollback `agent_key_bindings` table.
- Rollback `device_agents` indexes and columns.

#### [MODIFY] `backend/internal/domain/agent.go`
- Add `ClientInstanceID` to the `DeviceAgent` struct.

#### [NEW] `backend/internal/domain/agent_binding.go`
- Define `AgentKeyBinding` domain entity representing the new schema.

---

### 2. Service & Repository Layer

Create dedicated V2 enrollment boundaries rather than mixing with the admin-facing CRUD service:
- **[NEW]** `backend/internal/agentenrollment/repository.go` (`EnrollmentV2Repository`)
- **[NEW]** `backend/internal/agentenrollment/service.go` (`EnrollmentV2Service`)
- **[NEW]** `backend/internal/agentenrollment/challenge_store.go` (`ChallengeStore` via Redis)

#### Crypto Contract Details
- **Token lookup authority**: `SHA-256(raw enrollment token) → token_hash` (`token_prefix` is NEVER an authority).
- **Public Key format**: X.509 SubjectPublicKeyInfo (SPKI) DER → Base64 string.
- **Key parsing**: Must parse as ECDSA P-256 (`x509.ParsePKIXPublicKey` → `*ecdsa.PublicKey` → `elliptic.P256()`).
- **Fingerprint**: Lowercase hexadecimal `SHA-256(SPKI DER)`, exactly 64 characters.
- **Nonce**: 32 random bytes from CSPRNG.
- **Challenge ID**: >=128-bit CSPRNG.
- **Challenge Response**: base64url WITHOUT padding.
- **Agent Signing**: Agent decodes base64url challenge to raw 32 bytes and signs using `SHA256withECDSA`.
- **Signature Transport**: ASN.1 DER ECDSA signature → Base64.
- **Redis Storage**: `agent:enroll:challenge:<challenge_id>` (TTL 120s). Stores ONLY: `key_id`, `organization_id`, `client_instance_id`, `public_key_fingerprint`, `nonce`. NEVER store the raw token.

#### `GenerateChallenge`
1. Hashes raw `enrollment_token` to `token_hash`.
2. Resolves V2 enrollment key. Rejects if revoked or expired.
3. Parses and validates the P-256 public key.
4. Generates fingerprint and 32-byte CSPRNG nonce.
5. Saves challenge context to Redis and returns the `challenge_id` and base64url-encoded nonce to the client.

#### `EnrollAgent` (Transaction Order is MANDATORY)
1. Atomically `GETDEL` the challenge from Redis. (Rejects immediately if missing).
2. Verifies the ECDSA P-256 signature over the nonce.
3. Opens a PostgreSQL transaction (`BEGIN TX`).
4. `SELECT FOR UPDATE` on the `agent_enrollment_keys`.
5. **Revalidate**: Rejects if key is revoked or expired.
6. **Idempotency Check BEFORE Quota**:
   - Lookup existing `device_agents` by `organization_id` + `client_instance_id`.
   - If found AND `public_key_fingerprint` matches: return `200 OK` (idempotent success). Do NOT consume extra quota, do NOT create new bindings.
   - If found AND `public_key_fingerprint` differs: return `409 IDENTITY_CONFLICT`.
7. **Only if NEW identity**:
   - Count active bindings (`COUNT bindings WHERE released_at IS NULL`).
   - Enforce `max_bindings` quota. Rejects with `409 ENROLLMENT_QUOTA_EXHAUSTED` if exhausted.
   - Server generates fresh `device_id` and `agent_id` (NEVER derive from serial or fingerprint).
   - Create `devices`, `device_agents`, and `agent_key_bindings`.
   - Update `last_used_at` on the key.
8. Commit transaction and return `201 Created`.

---

### 3. Transport & API Layer

#### [NEW] `backend/internal/transport/http/agent_enrollment_handler.go`
Create `AgentEnrollmentHandlerV2` to completely separate from `AgentKeyHandler` (which remains Admin-only).
- Expose `POST /api/v2/agents/enroll/challenge`
- Expose `POST /api/v2/agents/enroll`
- Mount these endpoints OUTSIDE `authMiddleware`, `TenantMiddleware`, and `RequirePermission` in `main.go`. Apply global rate-limiting.

#### [MODIFY] `api/openapi.yaml`
- Add schemas for `POST /api/v2/agents/enroll/challenge` and `POST /api/v2/agents/enroll`.
- Set `security: []` on both operations since they rely entirely on the token/challenge rather than browser sessions.

**Final Enroll Contract**:
```json
{
  "enrollment_token": "cpk_xxx",
  "challenge_id": "chl_xxx",
  "client_instance_id": "uuid",
  "public_key": "base64-spki",
  "signature": "base64-der-signature",
  "device_info": {
    "manufacturer": "Samsung",
    "model": "SM-G930F",
    "android_version": "8.0",
    "sdk_int": 26,
    "serial_number": "...",
    "agent_version": "2.0.0",
    "protocol_version": "2"
  }
}
```
*Note: `serial_number` is strictly metadata, never used for matching.*

#### Error Contract
- **Generic 401/403** for invalid/revoked/expired tokens, invalid challenges, or signatures. (Do not expose token-oracle details).
- **400** for malformed request payloads.
- **409** for `IDENTITY_CONFLICT` and `ENROLLMENT_QUOTA_EXHAUSTED`.

---

## Verification Plan

### Automated Tests
Run real PostgreSQL and Redis integration tests via `go test -race ./...`. Tests must cover:
- **Migration Tests**: `000011` up/down/re-up, FK constraints, fingerprint `CHECK` regex, and partial unique binding constraint.
- **Crypto & Error Paths**: Invalid/revoked/expired token, invalid P-256 key, invalid signature, expired challenge, challenge replay, challenge/request mismatch.
- **Idempotency**: Same identity with fresh challenge yields `200 OK` (only one active binding exists). Same client ID with different key yields `409`.
- **Quota Enforcement**: Unlimited quota succeeds. Exhausted quota rejects.
- **Concurrency**: 2 concurrent requests with SAME identity: one `201`, one `200`, only one binding. 2 DIFFERENT identities with 1 remaining slot: exactly one `201`, exactly one quota rejection.
- **State Validation**: Revoking a Token Key leaves existing Agents/bindings unchanged. Offline device does not release binding.
- **Secret Hygiene**: Raw enrollment token is NEVER saved/logged in Redis or DB.

### Manual Verification
- Execute `go test -race ./...`
- Execute `go build ./cmd/server`
- Ensure frontend checks remain green.
