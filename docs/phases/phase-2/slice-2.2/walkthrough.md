# Slice 2.2 Walkthrough: Agent Binding & Challenges

I have completed the implementation of **Phase 2 Slice 2.2** ensuring the architecture strictly adheres to your required invariants.

## What Was Accomplished

1. **Database Schema Update (`000011_agent_key_bindings`)**:
   - `client_instance_id` added to `device_agents` with a conditional `UNIQUE` index.
   - Refactored `device_agents` constraint to allow composite reference via `(organization_id, device_id, agent_id)`.
   - `agent_key_bindings` table created with correct composite foreign keys enforcing domain integrity.

2. **Domain Layer**:
   - `AgentKeyBinding` struct added.
   - `ClientInstanceID` pointer added to `DeviceAgent`.

3. **OpenAPI V2 Definition & Go Models Generated**:
   - Added `POST /api/v2/agents/enroll/challenge` to request challenges.
   - Added `POST /api/v2/agents/enroll` to finalize enrollment using cryptographic proofs.
   - Removed legacy `POST /api/v1/agents/enroll` block entirely to prevent V1 leakage.
   - Re-generated models correctly map to new endpoints.

4. **Dedicated Enrollment Contracts & Repositories**:
   - **`ChallengeStore` (Redis)**: Uses `GETDEL` internally on finalize. Nonce is safely stored and validated.
   - **`EnrollmentV2Repository` (Postgres)**: Implemented using explicit transactions with `SELECT FOR UPDATE` to avoid race conditions and over-allocation.

5. **EnrollmentV2Service & AgentEnrollmentHandlerV2**:
   - Strict logic flow enforcing the "Idempotency before Quota" invariant.
   - Validates ECDSA (P-256) signatures via cryptographic proofs over base64url encoded nonces.
   - Distinguishes creation vs. idempotent existence to correctly map to HTTP 201 (Created) and HTTP 200 (OK).

6. **Wiring & Integration**:
   - Wired new routes safely *outside* of the `authMiddleware` inside `main.go`, preserving the required endpoint independence.

## Invariant Audit Checklist & Gate 2.2 Status

- [x] **2 Concurrent requests same identity**: Returns exactly one `201 Created` and one `200 OK`. `CheckIdempotency` safely returns existing binding properties when identity matches the challenge.
- [x] **2 Different identities / 1 slot**: Only the first to complete the transaction will succeed. The second evaluates `MaxBindings`, hits the cap, and returns `ErrQuotaExhausted` mapping to HTTP `409 Conflict`.
- [x] **Offline/Idempotency**: Offline devices re-trying after reconnecting with the same `client_instance_id` correctly re-establish connection state without consuming more quota.
- [x] **Cryptographic Consistency**: Validates lowercase Hex SHA-256 and P-256 ECDSA DER.
- [x] **Strict GETDEL Validation**: The `ChallengeStore.ConsumeChallenge` effectively uses Redis `GETDEL`, eliminating any replay attacks from hijacked nonces.

## Hardening & Production Polish (Owner Gate 2.2 Requisites)

- **Cryptographic Storage**: Now strictly stores the raw `public_key` (SPKI DER as `[]byte`) and validates its length/content alongside the 64-char lowercase `public_key_fingerprint`.
- **Strict Parsing**: Uses `dec.DisallowUnknownFields()` and length validation (e.g. `client_instance_id` <= 64) to reject payloads with trailing data or unmapped properties.
- **Typed Contracts**: Eliminated `map[string]interface{}` in favor of the strongly typed `AgentDeviceInfo`, explicitly validating the presence of Manufacturer, Model, AndroidVersion, SerialNumber, AgentVersion, and ProtocolVersion.
- **Security Check**: `ChallengeID` now generated via 32-byte `crypto/rand` encoded to base64url without padding, NOT hex.
- **Integration Test Matrix**: We created tests fully covering the concurrency matrix (same identity, different identity) natively interacting with the PostgreSQL transaction limits and isolation levels.
- **Schema Validation**: Tests added validating the CHECK constraint `chk_akb_fp` and the `client_instance_id` unique composite keys directly against Postgres 16.

All integration, unit, and frontend tests pass (`go test`, `npm run typecheck`, `npm run lint -- --max-warnings=0`, `npm run test -- --run`, `npm run build`), and the implementation follows your hardening plan exactly. 

> [!TIP]
> The infrastructure and security hardening for V2 Enrollment is now complete. The repository state matches the documented architecture goals. You can safely lock **Gate 2.2**.
