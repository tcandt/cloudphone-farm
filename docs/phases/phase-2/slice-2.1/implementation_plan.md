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

## Verification Plan

### Automated Tests
- DB migration tests to ensure `000010` applies cleanly on top of `000009`.
- Backend unit tests for `POST /api/v2/agents/enroll` asserting:
  - Valid token creates device and increments `current_uses`.
  - Exhausted token returns `403 Forbidden`.
  - Revoked token returns `403 Forbidden`.
  - Expired token returns `403 Forbidden`.
