# CLOUDPHONERENTAL V2 — MEDIA PLANE ARCHITECTURE V2

> **Tài liệu:** Bản thiết kế Kiến trúc Mặt phẳng Truyền dẫn Hình ảnh Thời gian thực V2 (Media Plane Blueprint)  
> **Công nghệ lõi:** Android MediaProjection, Hardware H.264 MediaCodec, Google WebRTC Native, WebRTC SFU, CoTURN  
> **Mục tiêu năng lực:** Vận hành mượt mà Bức tường 30–50 thiết bị hiển thị đồng thời trên một trình duyệt duy nhất

---

## 1. TỔNG QUAN KIẾN TRÚC MEDIA PLANE

Khác biệt hoàn toàn với các giải pháp cũ (như `socmtool` chụp ảnh Bitmap $\rightarrow$ nén JPEG $\rightarrow$ mã hóa Base64 $\rightarrow$ gửi qua JSON WebSocket gây quá tải CPU và lag nghẽn), **CloudPhoneRental Media Plane V2** được xây dựng 100% trên nền tảng **WebRTC Native Video Streaming** chuẩn công nghiệp kết hợp máy chủ **WebRTC SFU (Selective Forwarding Unit)** hỗ trợ Multi-layer Simulcast.

```mermaid
flowchart LR
    subgraph ANDROID_PIPELINE ["Android Hardware Media Pipeline"]
        Screen["Màn hình Android (Display)"]
        Proj["MediaProjection Service (FGS)"]
        VDisplay["VirtualDisplay Surface"]
        Codec["MediaCodec Hardware H.264 Encoder"]
        RTC_Agent["WebRtcPublisher (Native PeerConnection)"]
        
        Screen --> Proj --> VDisplay --> Codec --> RTC_Agent
    end

    subgraph MEDIA_GATEWAY ["Media Gateway & Distribution"]
        TURN["CoTURN (STUN/TURN Server)"]
        SFU["WebRTC SFU (Selective Forwarding Unit)<br/>- Multi-layer Simulcast Routing<br/>- Dynamic Layer Switching<br/>- Offscreen Track Pause"]
    end

    subgraph BROWSER_PIPELINE ["Browser Client Pipeline"]
        WallMgr["Wall Media Session Manager"]
        Obs["IntersectionObserver (UI Viewport Controller)"]
        VideoEl["HTML5 <video> Element (Hardware Decoded)"]
        
        WallMgr --> Obs --> VideoEl
    end

    RTC_Agent <===>|SRTP / RTP H.264 Track (Simulcast)| SFU
    SFU <===>|WebRTC Adaptive Quality Layer (Preview/Focus/Control)| WallMgr
    RTC_Agent -.-> TURN
    WallMgr -.-> TURN
```

---

## 2. NGUYÊN TẮC CÁCH LY MEDIA PLANE VỚI CONTROL PLANE

$$\text{MEDIA PLANE} \perp \text{CONTROL PLANE}$$

1. **Không can thiệp trạng thái Agent:** Việc khởi tạo WebRTC session, đàm phán lại SDP (Renegotiation) hoặc lỗi luồng hình ảnh tuyệt đối không được phép làm đứt socket điều khiển WSS (`/agent/v1/connect`) hay đổi trạng thái thiết bị thành Offline.
2. **Kênh Tín hiệu (Signaling Channel):**
   - Agent nhận yêu cầu mở stream và gửi SDP Offer qua WSEnvelope trên kênh WSS sẵn có.
   - Trình duyệt trao đổi SDP/ICE thông qua Endpoint chuyên biệt: `/api/v1/devices/{id}/media/ws`.
   - Backend đóng vai trò Gateway chuyển tiếp tín hiệu trong suốt (Signaling Relay).

---

## 3. CÁC HẠNG MỤC CHẤT LƯỢNG HÌNH ẢNH THÍCH ỨNG (QUALITY PROFILES)

Để đảm bảo trình duyệt máy tính của khách hàng không bị tràn bộ nhớ RAM hay sập card đồ họa khi mở cùng lúc 50 màn hình điện thoại, luồng truyền tải được chia làm 3 cấu hình chuẩn:

| Cấu hình | Độ phân giải | Tốc độ khung hình (FPS) | Băng thông (Bitrate) | Mục đích sử dụng |
| :--- | :--- | :--- | :--- | :--- |
| **1. PREVIEW** | $240 \times 480$ (hoặc 180p) | $2 - 5\text{ FPS}$ | $50 - 100\text{ Kbps}$ | Hiển thị thumbnail trên lưới 30–50 máy (Tổng wall: $\sim 3-5\text{ Mbps}$) |
| **2. FOCUS** | $360 \times 720$ (hoặc 480p) | $10 - 15\text{ FPS}$ | $250 - 400\text{ Kbps}$ | Theo dõi nhóm 4–8 máy đang chạy kịch bản tự động |
| **3. CONTROL** | $720 \times 1280$ (HD 720p) | $20 - 30\text{ FPS}$ | $800 - 1500\text{ Kbps}$ | Tương tác tay trực tiếp với độ trễ $< 100\text{ms}$ |

---

## 4. BẢN THIẾT KẾ BỨC TƯỜNG 30–50 MÁY & ĐỊNH LƯỢNG CHỈ SỐ (QUANTITATIVE SLA BENCHMARKS)

Khi hiển thị 30–50 thiết bị trên màn hình Dashboard:

```text
┌──────────────┬──────────────┬──────────────┬──────────────┬──────────────┐
│ Tile 01      │ Tile 02      │ Tile 03      │ Tile 04      │ Tile 05      │
│ [PREVIEW 3fps│ [PREVIEW 3fps│ [PREVIEW 3fps│ [PREVIEW 3fps│ [PREVIEW 3fps│
├──────────────┼──────────────┼──────────────┼──────────────┼──────────────┤
│ Tile 06      │ Tile 07      │ Tile 08      │ Tile 09      │ Tile 10      │
│ [PREVIEW 3fps│ [PREVIEW 3fps│ [PREVIEW 3fps│ [PREVIEW 3fps│ [PREVIEW 3fps│
├──────────────┼──────────────┼──────────────┼──────────────┼──────────────┤
│ ...                                                                        │
└────────────────────────────────────────────────────────────────────────────┘
```

### 4.1. Cơ chế Điều phối SFU & UI Viewport Controller:
1. **IntersectionObserver UI Controller:**
   - Hoạt động như một bộ điều khiển tín hiệu tại tầng Client (UI Optimization).
   - Khi Tile nằm trong Viewport $\rightarrow$ Kích hoạt Subscription nhận layer **PREVIEW** từ SFU.
   - Khi Tile cuộn ra ngoài Viewport (Offscreen) $\rightarrow$ Gửi tín hiệu `PAUSE_TRACK / UNSUBSCRIBE` lên SFU để SFU ngừng forward RTP packet, giúp giải phóng hoàn toàn băng thông mạng và năng lực giải mã của GPU.
2. **On-Demand Dynamic Layer Switching:**
   - Khi Operator click chuột vào bất kỳ Tile nào để mở Modal điều khiển $\rightarrow$ SFU chuyển đổi tức thì luồng của máy đó lên layer **CONTROL (720p / 30 FPS)**.
   - Khi đóng Modal $\rightarrow$ SFU chuyển trở lại layer **PREVIEW**.
3. **Chống bão Rerender trong React:**
   - Mỗi thẻ `<video>` được quản lý độc lập bên trong Component tự quản (Self-contained VideoSurface).
   - Dữ liệu WebRTC Stats (FPS, Bitrate, RTT) được cập nhật qua Direct DOM mutation hoặc Ref, không đưa vào React Root State gây giật lag giao diện.

### 4.2. Bộ Chỉ số Định lượng Nghiệm thu Bức tường 50 Thiết bị (Acceptance Benchmarks):
- **Tải CPU Trình duyệt (Browser CPU Load):** $< 30\%$ trên vi xử lý Desktop 8-core tiêu chuẩn (Intel Core i7 / AMD Ryzen 7 / Apple M-series).
- **Bộ nhớ Trình duyệt (Browser RAM Heap):** $< 600\text{ MB}$ sau 1 giờ chạy liên tục (Zero Memory Leak).
- **Tổng Băng thông Mạng Inbound (Total Inbound Bitrate):** $< 6\text{ Mbps}$ cho toàn bộ 50 máy ở chế độ PREVIEW.
- **Tỷ lệ Khung hình Rớt (Dropped Frames):** $< 2\%$ trên tổng số frame nhận được.
- **Thời gian Thực thi Tác vụ Dài (Long Tasks > 50ms):** $< 0.1\%$ trên tổng số khung hình trình duyệt.
- **Tải CPU Phần cứng Điện thoại (Android Hardware Encoder Load):** $< 15\%$ CPU trên từng máy vật lý nhờ sử dụng chip MediaCodec H.264 phần cứng.
- **Độ trễ Điều khiển Trực tiếp (Glass-to-Glass Latency):** $< 100\text{ms}$ khi ở chế độ CONTROL.

---

## 5. RÀNG BUỘC BẢO MẬT MEDIAPROJECTION KHI REBOOT

- **Nguyên tắc:** Trên ROM gốc Android 14+ (API 34) và 15+ (API 35), hệ điều hành Android **CẤM** khởi chạy Foreground Service loại `mediaProjection` từ các background receivers (như `BOOT_COMPLETED`) mà không có User Consent Token hợp lệ từ người dùng tại thời điểm khởi chạy.
- **Cam kết Khôi phục:**
  - Sau khi Reboot, hệ thống **KHÔNG CAM KẾT tự động khôi phục luồng stream (No Unattended Stream Guarantee)** trên ROM gốc unmanaged.
  - Tuy nhiên, kết nối điều khiển WSS (`AGENT_ONLINE`) và dịch vụ Trợ năng (`CONTROL_READY`) sẽ khôi phục 100% tự động, sẵn sàng nhận lệnh mở hộp thoại cấp quyền ghi hình khi kỹ thuật viên / operator yêu cầu.
