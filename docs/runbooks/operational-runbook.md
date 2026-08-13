# Operational Runbook — Phone Control Platform

> **Document Status:** Production Baseline v1.0  
> **Target Environment:** Ubuntu Server + Docker Containerization  
> **Reference Blueprint:** `Phone-Control-Platform-Blueprint.md`

---

## 1. Incident Response Procedures

### 1.1 Device Offline Spike
- **Symptom:** Multiple devices transition to `offline` or `degraded` status simultaneously.
- **Diagnostic Step:** Check `/app/diagnostics` readiness endpoint `/health/ready` and WebSocket simulator logs.
- **Remediation:** Verify network proxy connectivity, inspect coturn TURN server relay logs, and verify Redis presence key expiration (`pcp:presence:*`).

### 1.2 WebRTC Transport Failure / Relay Fallback
- **Symptom:** Screen video stream tiles display `reconnecting` spinner or fail SDP handshake.
- **Remediation:** Verify coturn port 3478 (UDP/TCP) firewall rules. Ensure TURN authentication credentials have not expired.

### 1.3 Control Lease Conflicts
- **Symptom:** Operator receives `409 CONTROL_LEASE_CONFLICT` when attempting to control a device.
- **Remediation:** Verify active lease countdown. If emergency override is required, perform explicit admin lease release (`DELETE /control-leases/{id}`).

---

## 2. Pre-Production Launch Checklist

- [x] TypeScript strict compilation passes without warnings (`npm run typecheck`).
- [x] Unit and E2E integration test suites pass (`npx vitest run`).
- [x] Production web assets built and optimized (`npm run build`).
- [x] All device actions capability-gated against non-root/non-ADB APK constraints.
- [x] Support Access Grants enforced with explicit owner consent and ticket audit trail.
- [x] OpenAPI 3.1 specification validated (`api/openapi.yaml`).
- [x] Docker orchestration manifest verified (`docker-compose.yml`).
