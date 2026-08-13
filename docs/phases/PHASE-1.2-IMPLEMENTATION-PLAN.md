# Phase 1.2 Implementation Plan — Go Backend Foundation & Authoritative Core

This document defines the architecture, database schema, project structure, and acceptance criteria for **Phase 1.2: Go Backend Foundation & Authoritative Core**.

---

## 🎯 Key Architectural Rules

1. **OpenAPI 1.2.0 Contract Locking & `oapi-codegen`**:
   - Single authoritative REST specification: `api/openapi.yaml` (v1.2.0).
   - Strict adherence to endpoint path conventions: `GET /devices`, `GET /devices/{id}`, `POST /devices/{id}/control-leases`, `POST /devices/{id}/control-leases/renew`, `DELETE /devices/{id}/control-leases`, `POST /commands`, `POST /auth/login`, `POST /auth/logout`, `GET /auth/session`.
   - Code generation via `oapi-codegen` producing Go Chi server interfaces and request/response structs.

2. **Commercial Multi-Tenant Data Model (19 Tables + Transactional Outbox)**:
   - `users` is a global identity table (no direct `organization_id`).
   - `organization_memberships` manages multi-organization user participation.
   - Composite tenant foreign keys `(organization_id, device_id)` strictly enforce tenant isolation at database engine level.
   - `command_outbox` table guarantees transactional event dispatching from PostgreSQL to WebSocket agents.

3. **Opaque Cookie Session Security (`__Host-pcp_session`)**:
   - Session cookie `__Host-pcp_session` holds a high-entropy 256-bit random opaque token (`HttpOnly`, `Secure`, `Path=/`, `SameSite=Strict`).
   - Backend stores SHA-256 hash of token. **Redis is the primary authoritative session store**.
   - Automatic token rotation on login and role modification; CSRF / Origin header validation on state-changing requests.

4. **Atomic Redis Control Lease & Idempotency Lua Transactions**:
   - **Atomic Lease Acquire**: `SET key value NX PX` with `fencing_counter`.
   - **Atomic Lease Renew / Release**: Lua scripts checking lease ID, user ID, and fencing token.
   - **Atomic Idempotency**: Lua script scoped to `organization + actor + idempotencyKey` storing SHA-256 fingerprint, `in_progress` / `completed` status, and cached response with 10-minute TTL.

5. **Clean Production Migration Hygiene**:
   - Production migration `000002_seed_initial_rbac.up.sql` contains ONLY immutable system roles and permission codes.
   - Dev seed organization and test users are strictly isolated in `backend/db/devseed/` / `backend/cmd/devseed/` (executes ONLY when `APP_ENV=development`).
   - Enrollment tokens stored as hashes; proxy credentials encrypted at-rest (AES-256-GCM ciphertext, nonce, key_version).

6. **Platform Operational Readiness**:
   - Structured logging via Go standard `log/slog`.
   - Correlation IDs (`X-Request-ID`, `X-Audit-Correlation-ID`).
   - Health endpoints `/health/live` and `/health/ready` (checking PostgreSQL & Redis ping).
   - Graceful shutdown (`os.Interrupt`, `SIGTERM`).
   - Middleware limits (body size limits, timeouts, rate limits).

---

## 📐 Comprehensive Commercial Database Schema (19 Tables)

1. `organizations` — Tenant boundaries.
2. `users` — Global user identities (email, Argon2id password hash).
3. `organization_memberships` — User <-> Org membership mapping.
4. `roles` — Role definitions (system roles have `organization_id NULL`).
5. `permissions` — Immutable system permission codes.
6. `role_permissions` — Role <-> Permission junction.
7. `user_roles` — Membership <-> Role mapping.
8. `devices` — Physical / Cloud Android phone entity registry.
9. `device_agents` — Agent metadata, protocol capabilities, APK version.
10. `device_heartbeats` — Historical presence and telemetry heartbeats.
11. `control_leases` — Audit history of control leases (Redis is real-time authority).
12. `commands` — Command envelope records and parameters.
13. `command_events` — Execution lifecycle progress events.
14. `command_outbox` — Transactional outbox for guaranteed WS delivery.
15. `proxies` — Proxy profile configurations (encrypted ciphertext, nonce, key_version).
16. `proxy_assignments` — Device <-> Proxy mapping rules.
17. `enrollment_tokens` — Onboarding tokens (stored as SHA-256 `token_hash`).
18. `sessions` — Historical session audit records.
19. `audit_logs` — Immutable compliance audit trail.

---

## 📁 Commit Scope 1 (Phase 1.2.1) Deliverables

- Pinned Go version (`1.22.6`) in `backend/go.mod`, `Dockerfile.backend`, and CI workflow.
- Updated `api/openapi.yaml` to `v1.2.0` with `GET /devices/{id}`.
- OpenAPI code generation setup (`backend/internal/transport/http/generated.go`).
- 19-table PostgreSQL migration schema (`000001_create_core_tables.up.sql` / `.down.sql`).
- System RBAC seed migration (`000002_seed_initial_rbac.up.sql` / `.down.sql`).
- Dev seed script isolated in `backend/db/devseed/`.
- Bootstrap server in `backend/cmd/server/main.go` with `log/slog`, `config`, `/health/live`, `/health/ready`, and graceful shutdown.
- Docker integration for Go backend service (`infra/docker/Dockerfile.backend`, `compose.yaml`, `compose.production.yaml`).
