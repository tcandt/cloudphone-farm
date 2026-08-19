# Slice 2.3 Implementation Plan: Admin Token Key UI + Phase 2 Contract Completion

## Audit & Current State
The Phase 2 V2 Token Key architecture relies on persistent `AgentKey`s with maximum bounds and strict revocability. However, the current Admin interface still serves the Phase 1 paradigm:
- **Frontend UI (`src/pages/AgentsPage.tsx`)**: Displays a QR code with a 10-minute expiry counter and polls `getTokenReadiness`. It calls the legacy V1 token endpoints.
- **Frontend Service (`src/services/enrollment-service.ts`)**: Operates against `POST /api/v1/enrollment-tokens` and includes a mock implementation for local dev (`MockEnrollmentService`).
- **Backend Contract**: `api/openapi.yaml` and `AgentKeyHandler` are missing the `GET /api/v2/agent-keys/{key_id}/devices` endpoint required by the architecture to list a key's bound agents. The repository `AgentKeyRepository` also lacks this method.
- **RBAC**: Existing APIs use standard permission guards (e.g. `agent.enroll`).

## Gap Analysis & Files to Change

### 1. Backend Contract Completion (Bindings Read-Only)
- **`api/openapi.yaml`**: Add `GET /api/v2/agent-keys/{key_id}/devices` endpoint, returning an array of device bindings. Regenerate OpenAPI Go models.
- **`backend/internal/domain/agent_binding.go`**: Verify struct serialization fields (already standard `json` tags).
- **`backend/internal/repository/agentkey_repository.go` & `postgres/agentkey_repository.go`**: Add `GetBindings(ctx context.Context, orgID, keyID string) ([]*domain.AgentKeyBinding, error)` method querying `agent_key_bindings`. Return active and historical bindings. Tenant isolation applies (WHERE organization_id = $1 AND key_id = $2).
- **`backend/internal/agentkey/service.go`**: Expose `GetBindings` through the service interface. Return `ErrNotFound` if the key doesn't belong to the org.
- **`backend/internal/transport/http/agentkey_handler.go`**: Implement `GetBindings` and wire it to `r.Get("/{keyId}/devices", h.GetBindings)`. Ensure 404 on missing/cross-tenant keys. Ensure raw secrets are NEVER returned. Add unit/integration tests for RBAC and cross-tenant access.

### 2. Frontend Admin Token Key UI
- **`src/services/agent-key-service.ts`** [NEW]: Implement an HTTP service wrapper around the new V2 endpoints (`/api/v2/agent-keys`). No mock implementation.
- **`src/pages/AgentsPage.tsx`**: Completely overhaul to list V2 Token Keys.
  - Remove QR code modal, countdown timer, and polling.
  - Display operational states: Name, Prefix, Status (`ACTIVE`, `EXPIRED`, `REVOKED`), Active Bindings/Capacity, Expiration, Last Used, Created Time.
  - Show "Unlimited" and "Forever" for null properties.
- **`src/components/agent-keys/CreateTokenKeyModal.tsx`** [NEW]: Modal to submit Name, Max Bindings, and Expiration.
- **`src/components/agent-keys/RawSecretRevealModal.tsx`** [NEW]: Dedicated one-time reveal modal for the `raw_secret` string returned on 201 Created. Ensures raw_secret is cleared on unmount. Explicit copy and warning.
- **`src/components/agent-keys/EditTokenKeyModal.tsx`** [NEW]: PATCH modal for name, max_bindings, expires_at.
- **`src/components/agent-keys/BindingsDrawer.tsx`** [NEW]: Sidebar or modal showing the results of `GET /api/v2/agent-keys/{key_id}/devices`.
- **`src/components/agent-keys/RevokeConfirmDialog.tsx`** [NEW]: Destructive modal emphasizing irreversibility.

## API & UI Flow
1. **List**: Load `GET /api/v2/agent-keys`. Render table.
2. **Create**: Submit `POST /api/v2/agent-keys`. Backend returns `AgentKeyCreatedResponse` containing `raw_secret`. The UI mounts `RawSecretRevealModal` with the secret. Once closed, the secret is irretrievable. The list is refreshed.
3. **Edit**: Open PATCH modal. Send delta to `PATCH /api/v2/agent-keys/{key_id}`.
4. **Revoke**: Open confirmation modal. Send `DELETE /api/v2/agent-keys/{key_id}`. Refresh list (status becomes REVOKED). No "Unrevoke".
5. **View Bindings**: User clicks a key. UI opens a drawer. Calls `GET /api/v2/agent-keys/{key_id}/devices`. Renders list of Device ID, Agent ID, PK fingerprint, bound_at, released_at.

## Raw-Secret Lifecycle & Strict Secret Rules
- Returned only once in `POST /api/v2/agent-keys`.
- UI holds `rawSecret` strictly in React local component state (e.g., `useState`).
- Destroyed explicitly on unmount.
- NOT stored in localStorage, URL, logs, or cache.
- `GET`, `PATCH`, `DELETE` responses inherently do not contain this field on the backend.

## RBAC & Tenant Handling
- The new `AgentKeyHandler` routes are protected by existing middleware (`auth.GetPrincipal`).
- Backend remains the authority on `organization_id` (derived from token).
- Frontend uses existing `PermissionGuard` wrapping Create/Edit/Revoke buttons, using existing permissions (e.g. `agent.enroll`).

## Test Matrix
**Frontend**:
- Component tests for `AgentsPage.tsx` listing varying states (Active, Expired, Revoked, Unlimited/Forever).
- Modal tests verifying one-time reveal behavior and secret destruction.
- Revoke irreversible UX.
- Loading/Error boundary tests.
**E2E (Playwright)**:
- Full V2 lifecycle: Create key -> verify secret shown once -> refresh page (secret gone) -> Edit -> View Bindings -> Revoke.
**Backend**:
- `TestAgentKey_GetBindings_CrossTenant`: Ensure 404 when querying another org's key.
- `TestAgentKey_GetBindings_Success`: Returns valid historical & active binding slice.
- `TestAgentKey_GetBindings_RawSecretAbsence`: Asserts the struct serialization hides `raw_secret`.
- Ensure standard `go test -race` passes and existing cluster gates remain green.

## Explicit Out-of-Scope
- Android Agent APK V2 UI modifications.
- AndroidKeyStore or client WSS credentials.
- Reconnect FSM / media work.
- Device groups or Bulk Control.
- Binding release/decommission workflow.
- Key rotation/recovery.
- Migrations 000012+.

## Phase 2 Final Gate Evidence Requirements
- Code conforms strictly to this plan.
- All 11 assertions described in "Test Requirements" from the Owner Authorization are explicitly fulfilled.
- CI pipeline completes 100% green (`go test`, Vitest, Playwright, production build).
- Post-merge, the Admin app natively uses V2 credentials to establish the baseline for Slice 2.4 (Android implementation).
