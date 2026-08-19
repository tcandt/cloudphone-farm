# Slice 2.3 Implementation Plan: Admin Token Key UI + Phase 2 Contract Completion

## Audit & Current State
The Phase 2 V2 Token Key architecture relies on persistent `AgentKey`s with maximum bounds and strict revocability. However, the current Admin interface still serves the Phase 1 paradigm:
- **Frontend UI (`src/pages/AgentsPage.tsx`)**: Displays a QR code with a 10-minute expiry counter and polls `getTokenReadiness`. It calls the legacy V1 token endpoints, while also listing registered APK agents.
- **Frontend Service (`src/services/enrollment-service.ts`)**: Operates against `POST /api/v1/enrollment-tokens` and includes a mock implementation for local dev (`MockEnrollmentService`).
- **Backend Contract**: `api/openapi.yaml` and `AgentKeyHandler` are missing the `GET /api/v2/agent-keys/{key_id}/devices` endpoint required by the architecture to list a key's bound agents. The `AgentKey` response is also missing the `active_bindings` count.
- **RBAC**: Existing APIs use the canonical `agent.enroll` permission, but the frontend UX needs strict alignment.

## 1. Lock Active Bindings Count Contract
The Admin list requires displaying `Active Bindings / Capacity`. To avoid N+1 queries from the frontend, an additive V2 response field must be introduced to the `AgentKey` schema:
```yaml
active_bindings:
  type: integer
  minimum: 0
```
- Both `GET /api/v2/agent-keys` (List) and `GET /api/v2/agent-keys/{key_id}` (Get) responses will expose this.
- **Backend implementation**: Must calculate `active_bindings` from `agent_key_bindings WHERE organization_id = $1 AND key_id = $2 AND released_at IS NULL`.
- **For LIST operations**: Use a single aggregate query (e.g., `LEFT JOIN` or a grouped batch). A one-query-per-key repository loop is strictly forbidden.
- **Capacity UI Format**:
  - If `max_bindings == null`: `"{active_bindings} / Unlimited"`
  - Otherwise: `"{active_bindings} / {max_bindings}"`
- **Quota Rule**: Reducing `max_bindings` below `active_bindings` does NOT revoke any existing agents.

## 2. Exact Bindings Endpoint Contract
**Endpoint**: `GET /api/v2/agent-keys/{key_id}/devices`
- **Middleware Chain**: `AuthMiddleware` → `TenantMiddleware` → `RequirePermission("agent.enroll")`. Key ownership must be verified first.
- **Error Codes**:
  - Unknown key: `404 Not Found`
  - Cross-tenant key: `404 Not Found`
  - Revoked key: `200 OK` (bindings remain readable).
  - No bindings: `200 OK` with payload `[]`.
- **Response**: Must return both active AND historical bindings. Stable order: `ORDER BY bound_at DESC, binding_id ASC`.
- **OpenAPI Schema (`AgentKeyBinding`)**:
  - `binding_id`
  - `device_id`
  - `agent_id`
  - `public_key_fingerprint`
  - `bound_at`
  - `released_at` (nullable)
  - `release_reason` (nullable)
- **Forbidden Returns**: Do NOT return `raw_secret`, `token_hash`, or public/private key bytes.
- **Frontend State Derivation**: The frontend derives status: `released_at == null => ACTIVE`, `released_at != null => HISTORICAL/RELEASED`.

## 3. Lock RBAC Authority
- The exact canonical permission is `agent.enroll`.
- **Production Chain**: `AuthMiddleware` → `TenantMiddleware` → `RequirePermission("agent.enroll")` → `AgentKeyHandler`.
- **Frontend Guard**: `PermissionGuard("agent.enroll")` is for UX only. The backend strictly remains the security authority.

## 4. Preserve Existing Agent Monitoring
- `AgentsPage.tsx` currently contains the registered APK Agents table. **Slice 2.3 replaces only the legacy V1 enrollment-token UX.**
- **DO NOT delete Agent monitoring functionality.** The Agent list itself remains available.
- **UI Composition**: The page will be restructured into clear sections, e.g., using tabs or a split layout:
  - `Token Keys` (Tab or Default Section)
  - `Registered Agents` (Tab or Secondary Section)
- The legacy V1 QR code, countdown timer, and readiness polling flow will disappear from this Admin Token Key surface.

## 5. Raw Secret Lifecycle
- `POST /api/v2/agent-keys` returns `raw_secret` exactly once.
- **Storage Rule**: Store only in ephemeral React memory (e.g., component `useState`).
- **Forbidden Storage**: Never store in `localStorage`, `sessionStorage`, `IndexedDB`, URL, application logs, `console.log`, analytics, toast payloads/history, or persisted React/query cache.
- **Closure Event**: On Close / Acknowledge, explicitly call `setRawSecret(null)` before/while closing the modal.
- **Cleanup**: On route/component cleanup, the raw-secret state must be cleared.
- After the modal closes or the page reloads, the application CANNOT retrieve `raw_secret` again.
- The Copy button requires an explicit user action.

## 6. Gap Analysis & Exact Files to Change

### Backend Contract & Logic
- **`api/openapi.yaml`**: Add `GET /api/v2/agent-keys/{key_id}/devices` endpoint and explicit `AgentKeyBinding` schema. Add `active_bindings` to `AgentKey`.
- **`backend/internal/transport/http/openapi_types.gen.go`** (Auto-generated): Updated types from OpenAPI spec.
- **`backend/internal/domain/agent_binding.go`**: Verify struct tags match exact OpenAPI contract.
- **`backend/internal/repository/agentkey_repository.go` & `postgres/agentkey_repository.go`**: 
  - Add `GetBindings(ctx, orgID, keyID)`. 
  - Modify `List` and `GetByID` to query/JOIN and return `active_bindings`.
- **`backend/internal/agentkey/service.go`**: Expose `GetBindings`, pass `active_bindings` through standard structures.
- **`backend/internal/transport/http/agentkey_handler.go`**: Wire `r.Get("/{keyId}/devices", h.GetBindings)` protected by `agent.enroll`.

### Frontend & Services
- **`src/services/agent-key-service.ts`** [NEW]: HTTP service wrapping `/api/v2/agent-keys`.
- **`src/types/openapi.d.ts`** (Auto-generated/Updated): Update frontend types matching the V2 OpenAPI spec.
- **`src/pages/AgentsPage.tsx`**: Add tabs to preserve the registered Agents table while rendering the new V2 Token Keys interface in the primary tab.
- **`src/components/agent-keys/CreateTokenKeyModal.tsx`** [NEW]: Form for Name, Max Bindings, Expiration.
- **`src/components/agent-keys/RawSecretRevealModal.tsx`** [NEW]: One-time reveal modal strictly following the Secret Lifecycle.
- **`src/components/agent-keys/EditTokenKeyModal.tsx`** [NEW]: PATCH modal for name, max_bindings, expires_at.
- **`src/components/agent-keys/BindingsDrawer.tsx`** [NEW]: Drawer calling the new `/devices` endpoint to render active/historical bindings.
- **`src/components/agent-keys/RevokeConfirmDialog.tsx`** [NEW]: Confirmation modal outlining the irreversible revocation semantics.

### Tests
- **Frontend Tests**: Update or add component tests in `tests/frontend/...`.
- **Backend Tests**: Add tests to `backend/internal/transport/http/agentkey_handler_test.go` and `backend/internal/agentkey/service_test.go`.
- **E2E Tests**: Update/Add Playwright tests in `tests/e2e/...`.

## 7. Complete Test Matrix Explicitly

### Frontend / Service Tests
- GET V2 key list.
- No V1 `enrollment-token` request from Token Key UI.
- Displays `ACTIVE` / `EXPIRED` / `REVOKED`.
- Renders `active_bindings/capacity` accurately (including `Unlimited` and `Forever`).
- Validates create payload.
- One-time raw secret reveal works.
- Raw secret state explicitly cleared on close (`setRawSecret(null)`).
- Refresh/reload cannot recover raw secret.
- PATCH allows only name, max_bindings, expires_at.
- Revoke confirmation copy is explicit.
- No unrevoke control exists.
- Bindings drawer opens and displays active + historical rendering derived from `released_at`.
- Loading state, empty state, validation error, 401/403, 404, 409, 5xx/network error + retry all handled gracefully.
- `agent.enroll` permission UX enforced.

### Backend Tests
- `active_bindings` calculation verified for both list and get.
- Aggregate list completely avoids tenant leakage.
- Bindings success endpoint returns proper data.
- Returns empty `[]` when no bindings exist.
- Returns active + historical bindings.
- Deterministic ordering (`bound_at DESC, binding_id ASC`).
- Unknown key returns `404`.
- Cross-tenant access returns `404`.
- Revoked key bindings remain readable.
- Raw secret absolutely absent from all get/list responses.
- Permission enforcement (`agent.enroll`) verified on handler.

### E2E with Real Backend (Playwright)
- `Create` → raw secret visible → `acknowledge/close` → `reload` → raw secret unavailable.
- `Edit` functionality succeeds.
- `View bindings` displays correct table data.
- `Revoke` → key remains listed as `REVOKED` → bindings remain viewable → no `Unrevoke` action available.
- Verify NO V1 enrollment-token network requests occur.

## 8. Explicit Out-of-Scope
- No migrations `000001–000011` modifications.
- No Android source modifications.
- No WSS/reconnect/media changes.
- Device groups or Bulk Control.
- Binding release/decommission workflow.
- Key rotation/recovery.

## 9. Correct Governance
- **Slice 2.3 completion** → **Phase 2 Final Gate** → **STOP**.
- **Phase 3 Android/APK requires separate Owner Authorization.**
