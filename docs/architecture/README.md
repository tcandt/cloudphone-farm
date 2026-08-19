# CLOUDPHONERENTAL V2 — ARCHITECTURE DOCUMENTATION INDEX

> **Trạng thái:** MỤC LỤC CHUẨN MỰC TỐI CAO CỦA HỆ THỐNG KIẾN TRÚC V2 (CANONICAL ARCHITECTURE INDEX)  
> **Phiên bản:** 2.1.0  
> **Ngày cập nhật:** 2026-08-19

---

## 1. TÀI LIỆU CHUẨN MỰC HIỆN HÀNH (AUTHORITATIVE V2 DOCUMENTS)

Tất cả các quyết định kỹ thuật, quy trình phát triển và kiểm thử của **CloudPhoneRental V2** bắt buộc phải đối chiếu và tuân thủ danh mục tài liệu dưới đây:

| Tên Tài liệu | Đường dẫn File | Vai trò & Phạm vi Nội dung |
| :--- | :--- | :--- |
| **Product Constitution** | [`PRODUCT-CONSTITUTION.md`](file:///d:/phone-farm/docs/architecture/PRODUCT-CONSTITUTION.md) | **Hiến pháp Dự án:** Các nguyên tắc bất biến về Stock ROM, Non-root, Token Key 1-time, ECDSA P-256, và phân tách 4 vòng đời độc lập. |
| **System Architecture V2** | [`SYSTEM-ARCHITECTURE-V2.md`](file:///d:/phone-farm/docs/architecture/SYSTEM-ARCHITECTURE-V2.md) | **Kiến trúc Tổng thể:** Bảo toàn 10 trụ cột Go/PG/Redis/Fencing, Multi-node Cluster Router, mô hình dữ liệu V2 và xác thực M2M. |
| **Android Agent V2** | [`ANDROID-AGENT-V2.md`](file:///d:/phone-farm/docs/architecture/ANDROID-AGENT-V2.md) | **Kiến trúc Android Agent:** Foreground Service, Connection Supervisor FSM, Keystore ECDSA P-256, SQLite Journal, Remote IME và UI/UX. |
| **Android Capability Matrix** | [`ANDROID-CAPABILITY-MATRIX.md`](file:///d:/phone-farm/docs/architecture/ANDROID-CAPABILITY-MATRIX.md) | **Ma trận Tương thích OS:** Chi tiết tương thích Android 8 $\rightarrow$ 15+, phân định 5 trạng thái Readiness và ràng buộc MediaProjection khi Reboot. |
| **Web Architecture V2** | [`WEB-ARCHITECTURE-V2.md`](file:///d:/phone-farm/docs/architecture/WEB-ARCHITECTURE-V2.md) | **Kiến trúc Ứng dụng Web:** Shared Packages (`packages/brand`, `packages/ui`, `packages/device-control`), Client Web (4 mục), và Admin Console. |
| **Media Plane V2** | [`MEDIA-PLANE-V2.md`](file:///d:/phone-farm/docs/architecture/MEDIA-PLANE-V2.md) | **Mặt phẳng Truyền thông WebRTC:** Multi-layer Simulcast SFU, 3 profile thích ứng, định lượng chỉ số SLA cho Bức tường 30–50 máy. |
| **Automation V2** | [`AUTOMATION-V2.md`](file:///d:/phone-farm/docs/architecture/AUTOMATION-V2.md) | **Tự động hóa Native:** UI Selector Engine qua AccessibilityNodeInfo (không phụ thuộc Appium ở runtime), mô hình Selector và Workflow. |
| **Master CodeGraph V2** | [`../codegraph/CODEGRAPH-V2.md`](file:///d:/phone-farm/docs/codegraph/CODEGRAPH-V2.md) | **Bản đồ CodeGraph:** Quy trình 10 bước kiểm tra trước khi code, Inbound/Outbound Call Paths và Ma trận Điểm chạm kỹ thuật. |
| **Phase Roadmap V2** | [`../phases/PHASE-ROADMAP-V2.md`](file:///d:/phone-farm/docs/phases/PHASE-ROADMAP-V2.md) | **Lộ trình 16 Giai đoạn (Phase 0 $\rightarrow$ 15):** Định nghĩa ranh giới Baseline SHA, nội dung từng Slice và tiêu chí nghiệm thu Owner Gates. |

---

## 2. TÀI LIỆU LỊCH SỬ ĐÃ HẾT HIỆU LỰC (SUPERSEDED / HISTORICAL DOCUMENTS)

Các tài liệu dưới đây thuộc giai đoạn thử nghiệm PCP 1.x cũ hoặc tài liệu nháp tạm thời, **CHỈ DÙNG ĐỂ THAM KHẢO LỊCH SỬ**, không có giá trị chi phối kiến trúc V2 hiện tại:

| Tài liệu Cũ | Đường dẫn File | Trạng thái | Lý do Hết hiệu lực |
| :--- | :--- | :--- | :--- |
| **PCP Master Blueprint** | `Phone-Control-Platform-Blueprint.md` | **SUPERSEDED** | Đã được thay thế toàn diện bởi Master Blueprint V2 trong `docs/architecture/*`. |
| **Phase 1.2 Implementation Plan** | `docs/phases/PHASE-1.2-IMPLEMENTATION-PLAN.md` | **HISTORICAL** | Thuộc kế hoạch giai đoạn cũ đã nghiệm thu trước mốc Rebaseline. |
| **Phase 1.7 Implementation Plan** | `docs/phases/PHASE-1.7-IMPLEMENTATION-PLAN.md` | **HISTORICAL** | Thuộc kế hoạch giai đoạn cũ đã nghiệm thu trước mốc Rebaseline. |
| **Sign-Off Document** | `docs/SIGN-OFF.md` | **HISTORICAL** | Ký duyệt các giai đoạn thử nghiệm trước ngày 18/8/2026. |
| **Root Mapping Markdown** | `mapping.md` / `maping.md` | **DELETED** | Đã được chuẩn hóa và hợp nhất vào `docs/codegraph/CODEGRAPH-V2.md`. |
