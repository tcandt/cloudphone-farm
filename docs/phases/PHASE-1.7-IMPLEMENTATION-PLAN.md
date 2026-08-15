# Phase 1.7 — Physical Device Fleet & Real WebRTC End-to-End Acceptance Implementation Plan

This document details the authoritative technical architecture, workstream execution plan, and 20 mandatory directives for **Phase 1.7 — Physical Device Fleet & Real WebRTC End-to-End Acceptance**.

---

## 1. Locked Baseline & Governance Rules

- **Authoritative Software Baseline**: Commit `e3a8618ebcf44c57ba56d72bb76a1acd531eab95` (GitHub Actions Run #130 / `31882343478`).
- **Working Branch**: `feature/phase-1.7-physical-webrtc`.
- **Macro-Phase Policy**: Phase 1.7 is a single, comprehensive phase containing Workstreams A–L and Gates A–G. No micro-phases (e.g. 1.7.1, 1.7.2) will be created. Bug fixes and adjustments are commits within Phase 1.7.
- **Physical Hardware Requirement**: All evidence for Gate A–G must be generated using physical Android devices attached to the cluster. Synthetic agents cannot substitute for physical hardware. If hardware is unavailable, the gate result is strictly `NOT_RUN`. Zero manufactured evidence.

---

## 2. Mandatory Architecture Directives (20 Directives)

1. **Stack Reuse**: Reuse existing `DeviceControlService`, `CommandProcessor`, `CommandJournal`, `FencingStore`, `NormalizedCoordinateMapper`, `video-geometry.ts`, `WebRtcMediaClient`, browser media signaling, and Redis session/viewer authority.
2. **No Parallel Executors**: Do NOT create a parallel `NewInputEngine`, `NewCommandExecutor`, `ADBExecutor`, or `uinputExecutor`.
3. **Agent Identity Keystore Wording & Logic**: Machine identity is Tink Ed25519; the 32-byte Ed25519 seed is encrypted at rest using AES-256-GCM with the AES key stored/generated in `AndroidKeyStore`. Record actual hardware security level (`KeyInfo.isInsideSecureHardware()` / `KeyProperties.SECURITY_LEVEL_*`). Never claim hardware-backed unless verified.
4. **Single Media Owner**: Eliminate dual `MediaProjection` capture ownership (`MediaCaptureService` vs `WebRtcPeerConnectionManager`). `WebRTC`-native capture is the single capture owner:
   `MediaProjection consent → mediaProjection FGS → ScreenCapturerAndroid → WebRTC VideoSource → H.264 → VideoTrack → PeerConnection → P2P / TURN`. Refactor `MediaCaptureService` to act strictly as a foreground service lifecycle manager for `MediaProjection`.
5. **Android 14+ TargetSdk 34 FGS Order**: User consent prompt MUST precede `mediaProjection` FGS start, and `mediaProjection` FGS MUST be running before the `MediaProjection` token is consumed by `ScreenCapturerAndroid`.
6. **Boot Lifecycle Invariant**: `BOOT_COMPLETED` starts ONLY core `AgentService` connectivity. Never auto-start `MediaProjection` on boot.
7. **Zero Invented Telemetry**: Replace hardcoded heartbeat telemetry (`battery = 85`, `cpu_usage = 15.0`, etc.) with real hardware measurement APIs (`BatteryManager`, `/proc/stat`, `ActivityManager`, `ThermalManager`). If a metric is unsupported/unavailable, send `null` or omit the field.
8. **Single Connection Supervisor**: Refactor `AgentWebSocketClient` into a single connection supervisor owned by `AgentService` with exponential backoff + jitter. Eliminate `Thread.sleep()` blocking calls in WS callbacks. No re-instantiations of `CommandProcessor` or EGL/WebRTC managers on every reconnect attempt.
9. **Coordinate Authority (`normalized_display_v1`)**: Browser pointer → actual `<video>` content rectangle (removing letterbox/pillarbox via `video-geometry.ts`) → normalized $x/y \in [0..1]$ → `normalized_display_v1` → `CommandProcessor` → physical display geometry → physical pixels → `AccessibilityService`. Canvas (if present) is strictly a visual overlay.
10. **Endpoint Distinction**: Local/CI = `wss://127.0.0.1:8443`. Production = TLS/WSS on public domain `:443`.
11. **Infrastructure Terminology**: Term the current Redis instance "Redis Shared Distributed Authority" or "Redis 7.2 persistent/authenticated authority", NOT "Redis Cluster".
12. **Extended WebRTC Evidence**: Collect `selectedCandidatePairId`, `localCandidateId`, `remoteCandidateId`, `bytesReceived`, `packetsReceived`, `packetsLost`, `framesReceived`, `framesDecoded`, `framesDropped`, `framesPerSecond`, `frameWidth`, `frameHeight`, `codecId`, `codecMimeType`, `codecPayloadType`, `jitter`, and `currentRoundTripTime`.
13. **Gate C H.264 Verification**: Browser stats must prove `codec.mimeType == "video/H264"`, `framesDecoded > 0`, and `bytesReceived(t2) > bytesReceived(t1)`.
14. **Gate D TURN Verification**: Selected candidate pair must prove `candidateType = relay` with active decoded video progression.
15. **Physical Gesture PASS Criteria**: `command_id` → `accepted` → `dispatched` → `ACK` → `executing` → physical screen gesture → `succeeded` → browser event + physical screenshot evidence.
16. **Fleet Isolation Scale**: Require $\ge 3$ physical Android devices connected simultaneously for Gate F with zero cross-device command, stream, presence, or lease leakage.
17. **Hardware Absence Handling**: Mark result as `NOT_RUN` when hardware is unavailable. Never manufacture evidence.
18. **Evidence Package Schema**: Bundle `git_sha`, `timestamp`, `device_id`, `agent_id`, `connection_id`, `generation`, `command_id` / `media_session_id`, raw logs, screenshots/video, and browser `getStats` JSON.
19. **ADB Boundary**: ADB is permitted ONLY for APK installation, logcat collection, and device metadata harvesting. Runtime control and media stream MUST NOT depend on ADB.
20. **Gate Verification Closure**: Phase 1.7 closes only when Gate A–G all have machine-verifiable + physical evidence.

---

## 3. Workstream Execution Sequence

```
Foundation Hardening (Workstream A & I Core)
  ├─ Single Media Owner Refactoring & FGS Order
  ├─ Real Hardware Telemetry Harvester
  └─ Supervised WS Reconnect Manager
         │
         ▼
Physical Input & Geometry (Workstream B & C)
  ├─ Harden DeviceControlService & CommandProcessor on real hardware
  └─ Validate normalized_display_v1 coordinate mapping across Portrait/Landscape
         │
         ▼
Real MediaProjection & H.264 WebRTC Stream (Workstream D, E, F, G, H)
  ├─ Consent-Safe MediaSession flow (start -> consent -> FGS -> ScreenCapturerAndroid -> 101)
  ├─ WebRTC getStats telemetry & H.264 codec verification
  └─ Forced Coturn TURN Relay & Single Viewer Quota (Max 1)
         │
         ▼
Resilience, Isolation & Security (Workstream I, J, K)
  ├─ Wi-Fi loss/reconnect, backend node kill failover, browser refresh lease cleanup
  ├─ FLAG_SECURE black-stream physical verification
  └─ Simultaneous 3-Device Fleet Isolation (Device A, B, C)
         │
         ▼
Physical Evidence Collector & Report (Workstream L & Gates A-G)
  ├─ Create scripts/phase17-physical-gate/ evidence collector
  └─ Machine-verify artifacts and generate PHASE-1.7-ACCEPTANCE.md
```

---

## 4. Phase 1.7 Acceptance Gates (A–G)

| Gate ID | Gate Description | Acceptance Criteria |
| ------- | ---------------- | ------------------- |
| **Gate A** | Physical Fleet Lifecycle | $\ge 3$ physical Android devices enrolled, Tink Ed25519 key encrypted via AndroidKeyStore AES-256-GCM, BOOT_COMPLETED auto-reconnect without auto-stream, generation $N+1$ reconnect, 10s heartbeat / 30s TTL. |
| **Gate B** | Physical Control & ACK | Physical screen execution of `tap`, `swipe`, `HOME`, `BACK`, `APP_SWITCH` with physical screen evidence & ACK telemetry. |
| **Gate C** | Real H.264 Screen Capture | `MediaProjection` → `ScreenCapturerAndroid` → H.264 → WebRTC → browser `<video>` first frame rendered with `codec.mimeType == "video/H264"`, `framesDecoded > 0`, `bytesReceived` increasing. |
| **Gate D** | Networking & TURN Relay | Verified Direct ICE P2P and forced Coturn TURN relay (`candidateType = relay`). |
| **Gate E** | Security & Consent | OS `MediaProjection` consent prompt, `FLAG_SECURE` black-stream protection, revoked/stale agent rejection. |
| **Gate F** | Scale & Isolation | Concurrent 3-device operation with `MAX_VIEWERS_PER_DEVICE = 1` and strict zero cross-device leakage. |
| **Gate G** | Automated Evidence Package | Complete artifact bundle containing raw logs, `getStats` JSON, screenshots/videos, and `PHASE-1.7-ACCEPTANCE.md`. |
