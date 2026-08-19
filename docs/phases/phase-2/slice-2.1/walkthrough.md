# Walkthrough: Phase 2 - Slice 2.1 (Agent Enrollment Keys)

## Goal

Implement Slice 2.1 of the Phase 2 Agent Identity Enrollment architecture, establishing the robust `AgentEnrollmentKeys` foundation with strict security invariants.

## Changes Made

### 1. Database Migrations
- Added `000010_agent_enrollment_keys` up and down SQL scripts.
- Implemented `agent_enrollment_keys` table with strict constraints on `organization_id` and `key_id`.
- Used partial unique indexes to manage active vs revoked states.

### 2. Domain & Interfaces
- Defined `AgentKey` struct inside `backend/internal/domain/agentkey.go` with explicit handling of `TokenHash` (never exposed over HTTP) and nullable `RevokedAt` / `ExpiresAt`.
- Added `AgentKeyRepository` interface in `backend/internal/repository/`.

### 3. Service Layer (`agentkey.AgentKeyService`)
- Added core business logic: `CreateKey`, `GetKey`, `ListKeys`, `UpdateKey`, `RevokeKey`.
- Enforced 256-bit CSPRNG generation for the raw secret, hashing it with SHA-256 before persisting, and exposing a safe `TokenPrefix` for UI display.
- Implemented irreversible revocation by utilizing `revoked_at = NOW()` instead of row deletion.
- Added extensive security tests verifying tenant isolation, prefix safety, and correct hash mismatch with the raw secret.

### 4. HTTP Handlers & OpenAPI Contracts
- Updated `api/openapi.yaml` to define `/api/v2/agent-keys` routes and JSON schema mapping.
- Re-ran `oapi-codegen` to synchronize generated structs and the Server interface.
- Wrote `AgentKeyHandler` translating REST constraints, ensuring the raw secret is returned *exactly once* during a 201 Created response.
- Wired dependencies into `backend/cmd/server/main.go` under the `/api/v1` group (`/agent-keys`).

## Verification Performed

- **Unit tests:** Passed for `TestAgentKeyService_CreateKey`, `TestAgentKeyService_TenantIsolation`, and `TestAgentKeyService_Revoke`.
- **Database Migrations:** Checked checksum drift integration via `go test ./cmd/migrate`.
- **Compilation:** Successfully compiled the server `go build -o /dev/null ./cmd/server/main.go`
- **End-to-End Tests:** All current repository tests complete with zero regressions.

> [!TIP]
> The Agent Key domain is now fully ready for the next slice (Slice 2.2). We can proceed with `agent_key_bindings` when you are ready to implement the physical device-level agent identity mapping.
