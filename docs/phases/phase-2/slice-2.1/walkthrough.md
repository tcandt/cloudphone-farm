# Walkthrough: Phase 2 - Slice 2.1 (Agent Enrollment Keys)

## Goal

Implement Slice 2.1 of the Phase 2 Agent Identity Enrollment architecture, establishing the robust `AgentEnrollmentKeys` foundation with strict security invariants.

## Changes Made

### 1. Database Migrations
- Added `000010_agent_enrollment_keys` up and down SQL scripts.
- Implemented `agent_enrollment_keys` table with strict constraints on `organization_id` and `key_id`.
- Removed mentions of non-existent partial unique indexes; enforced unique constraint on `(organization_id, key_id)` directly.

### 2. Domain & Interfaces
- Defined `AgentKey` struct inside `backend/internal/domain/agentkey.go` with explicit handling of `TokenHash` (never exposed over HTTP) and nullable `RevokedAt` / `ExpiresAt`.
- Added `AgentKeyRepository` interface in `backend/internal/repository/`.

### 3. Service Layer (`agentkey.AgentKeyService`)
- Added core business logic: `CreateKey`, `GetKey`, `ListKeys`, `UpdateKey`, `RevokeKey`.
- Enforced 256-bit CSPRNG generation for the raw secret, hashing it with SHA-256 before persisting, and exposing a safe `TokenPrefix` for UI display.
- Implemented irreversible revocation by utilizing `revoked_at = NOW()` instead of row deletion.
- Enforced strict validations (name trimming/length, quota limits > 0).

### 4. HTTP Handlers & OpenAPI Contracts
- Updated `api/openapi.yaml` to define `/api/v2/agent-keys` routes and JSON schema mapping via an endpoint-level server scope.
- Re-ran `oapi-codegen` to synchronize generated structs and the Server interface using pinned version `v2.4.1`.
- Wrote `AgentKeyHandler` translating REST constraints, ensuring the raw secret is returned *exactly once* during a 201 Created response.
- Wired dependencies into `backend/cmd/server/main.go` under an isolated `/api/v2` route strictly separated from V1.
- Ensured strict tri-state JSON decoding to prevent type coercions and block unknown immutable fields on PATCH.

### 5. Repository Layer
- Addressed `UPDATE` lost-update anomalies by utilizing a single atomic dynamic SQL statement for `PATCH`, eliminating the need for `SELECT`-before-`UPDATE`.

## Verification Performed

- **Unit tests:** Passed for `TestAgentKeyService_CreateKey`, `TestAgentKeyService_TenantIsolation`, and `TestAgentKeyService_Revoke` (`go test ./internal/agentkey -count=1`).
- **Handler tests:** Executed `go test ./internal/transport/http -run TestAgentKeyHandler -count=1` to verify tri-state PATCH decoding, malformed type rejections, tenant isolation, CORS OPTIONS, and `/api/v2` routing.
- **Database Migrations:** Executed real Postgres tests via `go test ./internal/repository/postgres -run TestMigration000010_AgentEnrollmentKeys` proving apply/rollback, constraints, and FK behaviors without skipping.
- **Compilation:** Successfully compiled the server `go build ./cmd/server`
- **Frontend Verification:** `npm run typecheck`, `npm run lint -- --max-warnings=0`, `npm run test -- --run`, and `npm run build` completed with zero regressions.

> [!TIP]
> The Agent Key domain is now fully ready for the next slice (Slice 2.2). We can proceed with `agent_key_bindings` when you are ready to implement the physical device-level agent identity mapping.
