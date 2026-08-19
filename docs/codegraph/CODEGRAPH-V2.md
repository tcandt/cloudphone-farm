# CLOUDPHONERENTAL V2 — CODEGRAPH V2

> **Tài liệu:** Bản đồ CodeGraph Tổng thể & Ma trận Điểm chạm Toàn hệ thống V2 (Authoritative Master CodeGraph & Dependency Matrix)  
> **Trạng thái:** TÀI LIỆU DUY NHẤT VÀ CHUẨN MỰC TỐI CAO CỦA HỆ THỐNG CODEGRAPH (CANONICAL)  
> **Phiên bản:** 2.1.0 (Audit Resolution V2)

---

## 1. NGUYÊN TẮC 10 BƯỚC CODEGRAPH BẮT BUỘC TRƯỚC MỌI TASK

Mọi lập trình viên / Agent trước khi sửa đổi hoặc viết thêm bất kỳ dòng code nào đều bắt buộc phải đi qua quy trình 10 bước:

```text
[1. Search CodeGraph] ──> [2. Identify Existing Code] ──> [3. Map Inbound Calls] ──> [4. Map Outbound Calls]
                                                                                            │
[8. Map Web Impact] <── [7. Map Android Impact] <── [6. Check API Contract] <── [5. Map DB Touchpoints]
        │
        └──> [9. Define Test Plan & Evidence] ──> [10. THEN WRITE CODE]
```

---

## 2. BẢN ĐỒ TỔNG THỂ CÁC LUỒNG GỌI HÀM (MASTER CALL GRAPHS)

### 2.1. Luồng 1: Khởi tạo Token Key & Đăng ký Thiết bị (Enrollment Key V2)

```mermaid
flowchart TD
    subgraph WEB_ADMIN ["Admin Console"]
        CreateKeyDialog["CreateTokenKeyDialog.tsx"] --> ApiClientEnroll["api-client: /api/v1/agent-keys"]
    end

    subgraph BACKEND_ENROLL ["Backend Core"]
        ApiClientEnroll --> HTTP_KeyHandler["transport/http: KeyHandler.CreateKey"]
        HTTP_KeyHandler --> Svc_Enroll["enrollment: EnrollmentService.CreateKey"]
        Svc_Enroll --> Repo_Key["repository/postgres: EnrollmentKeyRepo.InsertKey"]
        
        HTTP_AgentEnroll["transport/http: AgentHandler.EnrollV2"] --> Svc_EnrollV2["enrollment: EnrollmentService.EnrollV2"]
        Svc_EnrollV2 --> Repo_KeyLock["repository/postgres: EnrollmentKeyRepo.LockAndValidateQuota"]
        Svc_EnrollV2 --> Repo_Dev["repository/postgres: DeviceRepo.CreateDevice"]
        Svc_EnrollV2 --> Repo_Agent["repository/postgres: AgentRepo.CreateAgent"]
        Svc_EnrollV2 --> Repo_Binding["repository/postgres: KeyBindingRepo.BindDevice"]
    end

    subgraph ANDROID_AGENT ["Android Agent"]
        ConnectUI["ui/ConnectActivity.kt"] --> EnrollMgr["enrollment/EnrollmentManager.kt"]
        EnrollMgr --> KeyStoreAgent["security/AgentKeyStore.kt: getOrCreateKeyPair(ECDSA P-256)"]
        EnrollMgr --> EnrollApiAgent["enrollment/EnrollmentApi.kt: POST /api/v2/agents/enroll"]
        EnrollApiAgent --> HTTP_AgentEnroll
        EnrollMgr --> CredStore["security/CredentialStore.kt: saveIdentity(agentId, deviceId)"]
        EnrollMgr --> ConnSvc["connection/AgentConnectionService.kt: startService()"]
    end
```

---

### 2.2. Luồng 2: Kết nối WebSocket & Xác thực Mật mã Challenge-Response (ECDSA P-256)

```mermaid
flowchart TD
    subgraph ANDROID_CONN ["Android Connection Pipeline"]
        Supervisor["connection/ConnectionSupervisor.kt"] --> WS["connection/AgentWebSocket.kt: connect()"]
        WS --> Signer["security/ChallengeSigner.kt: signChallenge(nonce)"]
        Signer --> KeyStore["security/AgentKeyStore.kt: sign(SHA256withECDSA)"]
    end

    subgraph BACKEND_WS ["Backend WSS Gateway"]
        WS -->|WSS /agent/v1/connect| WSHandler["transport/ws: AgentWSHandler.Connect"]
        WSHandler --> MW_AgentAuth["middleware: AgentAuthMiddleware (ecdsa.VerifyASN1)"]
        WSHandler --> Hub["agentws: Hub.RegisterConnection"]
        Hub --> Repo_Presence["repository/redis: PresenceRepo.SetOnline"]
        Hub --> Repo_Conn["repository/redis: AgentConnRepo.SetConnection"]
        Hub --> ClusterRouter["cluster: ClusterRouter.BroadcastPresence"]
    end
```

---

### 2.3. Luồng 3: Điều khiển Cử chỉ Đơn lẻ & Fencing Token

```mermaid
flowchart TD
    subgraph WEB_CONTROL ["Web Client (packages/device-control)"]
        Overlay["viewer/ViewerOverlay.tsx"] --> GestureEngine["gesture/PointerGestureEngine.ts: onPointerDown/Up"]
        GestureEngine --> Normalizer["gesture/CoordinateNormalizer.ts: toNormalizedV1()"]
        Normalizer --> CmdClient["commands/CommandClient.ts: dispatchTouch()"]
    end

    subgraph BACKEND_CMD ["Backend Command Pipeline"]
        CmdClient -->|POST /api/v1/commands| CmdHandler["transport/http: CommandHandler.Dispatch"]
        CmdHandler --> LeaseSvc["device: LeaseService.ValidateLeaseAndFence"]
        CmdHandler --> CmdSvc["command: CommandService.CreateCommandTx"]
        CmdSvc --> Repo_Cmd["repository/postgres: CommandRepo.Insert"]
        CmdSvc --> Repo_Outbox["repository/postgres: OutboxRepo.Insert"]
        
        OutboxWorker["command: OutboxDispatcher.PollLoop"] --> Repo_Outbox
        OutboxWorker --> Hub_Or_Cluster{"Is Local Node?"}
        Hub_Or_Cluster -->|Yes| Hub_Local["agentws: Hub.DispatchToConnection"]
        Hub_Or_Cluster -->|No| Cluster_Route["cluster: ClusterRouter.RouteCrossNode"]
    end

    subgraph ANDROID_EXEC ["Android Execution Pipeline"]
        Hub_Local -->|WSEnvelope 'command.dispatch'| AgentWS["connection/AgentWebSocket.kt"]
        AgentWS --> CmdProc["command/CommandProcessor.kt: enqueue()"]
        CmdProc --> FenceStore["command/FencingStore.kt: validateToken()"]
        CmdProc --> Journal["command/CommandJournal.kt: recordExecuting()"]
        CmdProc --> CoordMapper["control/CoordinateMapper.kt: toPhysicalPixel()"]
        CoordMapper --> DevControl["control/DeviceControlService.kt: performGesture()"]
        DevControl --> A11yNode["android.accessibilityservice.AccessibilityService"]
        CmdProc --> JournalDone["command/CommandJournal.kt: recordSucceeded()"]
        CmdProc -->|WSEnvelope 'command.status'| AgentWS
    end
```

---

### 2.4. Luồng 4: Điều phối Lệnh Hàng loạt (Bulk Command Fan-Out)

```mermaid
flowchart TD
    subgraph WEB_BULK ["Web Batch Selection"]
        BatchBar["components/DeviceBatchToolbar.tsx"] --> BatchClient["packages/device-control: BatchCommandClient.sendBatch()"]
    end

    subgraph BACKEND_BULK ["Backend Bulk Coordinator"]
        BatchClient -->|POST /api/v1/commands/batch| BulkHandler["transport/http: BulkHandler.DispatchBatch"]
        BulkHandler --> BulkSvc["bulkcontrol: BulkCommandService.FanOutCommands"]
        BulkSvc --> BulkSession["bulkcontrol: BulkSessionManager.TrackProgress"]
        BulkSvc -->|Loop N Devices| CmdSvc["command: CommandService.CreateCommandTx"]
        CmdSvc --> Repo_Outbox["repository/postgres: OutboxRepo (N rows)"]
    end

    subgraph AGENTS_BULK ["N Android Devices"]
        Repo_Outbox --> OutboxWorker["command: OutboxDispatcher"]
        OutboxWorker --> Device1["Agent Phone 1"]
        OutboxWorker --> Device2["Agent Phone 2"]
        OutboxWorker --> Device50["Agent Phone 50"]
    end
```

---

## 3. MA TRẬN ĐIỂM CHẠM THAY ĐỔI THEO TỪNG TÍNH NĂNG (TOUCHPOINT MATRIX)

| Tính năng yêu cầu | API Contract (`api/`) | Backend Domain / Services (`backend/internal/`) | Database Migrations (`backend/db/`) | Android Agent (`android-agent/`) | Web Applications (`src/`, `packages/`) | Tests Bắt buộc |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **Thêm loại Lệnh Mới** | `openapi.yaml` | `domain/command.go`<br/>`agentws/protocol.go`<br/>`command/command_service.go` | Không bắt buộc | `command/CommandProcessor.kt`<br/>`control/DeviceControlService.kt` | `packages/contracts/`<br/>`packages/device-control/commands/` | Unit test CommandProcessor<br/>E2E Dispatch test |
| **Nâng cấp Enrollment Key V2** | `openapi.yaml` | `enrollment/service.go`<br/>`transport/http/agent_handler.go` | `000010_agent_enrollment_keys.up.sql`<br/>`000011_agent_key_bindings.up.sql` | `enrollment/EnrollmentManager.kt`<br/>`ui/ConnectActivity.kt` | `pages/admin/TokenKeysPage.tsx`<br/>`packages/api-client/` | Concurrency enrollment test<br/>Quota exhaustion test |
| **Thêm Native UI Selector** | `openapi.yaml` | `domain/automation.go`<br/>`agentws/protocol.go`<br/>`workflow/service.go` | `000014_workflow_engine.up.sql` | `automation/UiSelectorEngine.kt`<br/>`automation/AutomationExecutor.kt` | `pages/admin/WorkflowsPage.tsx`<br/>`packages/device-control/` | Hierarchy snapshot test<br/>Selector resolution test |
| **Nâng cấp Media Adaptive SFU** | `openapi.yaml` | `media/session_service.go`<br/>`transport/ws/media_handler.go` | `000015_media_wall_sessions.up.sql` | `media/ScreenCapturer.kt`<br/>`media/WebRtcPublisher.kt` | `packages/device-control/media/`<br/>`pages/client/DeviceManagementPage.tsx` | 2-hour soak test<br/>Wall 30-tile memory test |
| **Thêm Gói Thuê & Nạp tiền Ví** | `openapi.yaml` | `rental/service.go`<br/>`billing/service.go`<br/>`transport/http/billing_handler.go` | `000017_rental_and_billing.up.sql` | Không chạm | `pages/client/RentalStorePage.tsx`<br/>`pages/client/WalletPage.tsx` | Transaction balance test<br/>Auto-expiry test |
