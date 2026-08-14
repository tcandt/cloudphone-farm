# Phase 1.5 — 24h & 72h Production Soak Testing Runbook

This runbook documents the procedures, environment configuration, command flags, and threshold assertions for running **24-hour (`SOAK_24H`)** and **72-hour (`SOAK_72H`)** commercial soak testing profiles.

---

## 1. Objectives & Threshold Assertions

To pass commercial production gate readiness, the cluster must sustain continuous load under the following thresholds:

| Metric | Target Threshold | Action on Violation |
| :--- | :--- | :--- |
| **Command Delivery Success Rate** | $\ge 99.9\%$ (Error Rate $< 0.1\%$) | Gate FAIL — Failover/Outbox Retry Investigation |
| **P95 Latency** | $< 100\text{ ms}$ | Gate FAIL — DB / Redis Latency Audit |
| **Memory Growth (Leaks)** | Bounded ($\Delta \text{Heap} < 5\%$ over 24h) | Gate FAIL — Go Routine / Hub Map Leak Audit |
| **Redis & DB Connections** | Zero Leaked Connections | Gate FAIL — Connection Pool Audit |

---

## 2. Infrastructure Setup

Launch 3 Go Backend nodes, PostgreSQL 16, and Redis 7 in cluster topology:

```bash
docker compose -f compose.cluster.yml up -d --build
```

Verify all 3 nodes are alive in Redis cluster directory:

```bash
redis-cli KEYS "pcp:v1:node:*"
# Should output:
# pcp:v1:node:node-a
# pcp:v1:node:node-b
# pcp:v1:node:node-c
```

---

## 3. Execution Commands

### 5-Minute CI Smoke Profile (Automated CI Gate)
```bash
cd backend
go run ./cmd/loadharness -duration=5m -concurrency=50 -rate-per-min=500
```

### 24-Hour Production Soak Profile (`SOAK_24H`)
```bash
cd backend
go run ./cmd/loadharness -duration=24h -concurrency=50 -rate-per-min=1000 > ../tests/soak/soak_24h_report.json
```

### 72-Hour Enterprise Soak Profile (`SOAK_72H`)
```bash
cd backend
go run ./cmd/loadharness -duration=72h -concurrency=100 -rate-per-min=2000 > ../tests/soak/soak_72h_report.json
```

---

## 4. Machine-Readable Report Verification

Upon completion, `loadharness` generates `soak_report.json`:

```json
{
  "timestamp": "2026-08-14T22:50:00Z",
  "duration": "24h0m0s",
  "total_commands": 1440000,
  "success_count": 1439985,
  "failure_count": 15,
  "error_rate_pct": 0.001,
  "p50_latency_ms": 2.1,
  "p95_latency_ms": 8.4,
  "p99_latency_ms": 18.7,
  "status": "PASS"
}
```

Verify that `"status": "PASS"`.
