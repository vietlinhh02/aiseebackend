# Tổng quan kiến trúc hệ thống

## Hệ thống gồm 4 thành phần chính

AiSee là ứng dụng trợ lý AI cho người suy giảm thị lực tại Việt Nam. Người dùng
tương tác với AI bằng giọng nói, AI có thể nhìn qua camera khi cần, và thực hiện
các hành động (gọi điện, nhắn tin, nhắc thuốc, ...) thay cho người dùng.

Hệ thống gồm 4 component:

1. **Flutter App** -- ứng dụng di động, giao diện voice-first cho người dùng, giao
   diện tab-based cho người thân
2. **LiveKit Server** -- WebRTC SFU, quản lý tất cả kết nối audio/video/data
3. **Agent Worker (Python)** -- chạy Gemini Live API, định tuyến tool call
4. **Go Backend** -- toàn bộ business logic, database, push notification

## Sơ đồ kiến trúc tổng thể

```
+------------------------------------------------------------------+
|                     FLUTTER APP                                  |
|                  (1 app, 2 giao diện)                            |
|                                                                  |
|  +------------------------------------------------------------+  |
|  |  LiveKit Client SDK (livekit_client)                        |  |
|  |                                                            |  |
|  |  Một kết nối WebRTC duy nhất cho TẤT CẢ:                   |  |
|  |  - Hội thoại với AI (Gemini Live)                          |  |
|  |  - Camera frames --> AI vision (khi cần)                   |  |
|  |  - Gọi VoIP với người thân (cùng room)                     |  |
|  |  - Data channel (điều khiển app <--> agent)                |  |
|  +----------------------------+-------------------------------+  |
+-------------------------------+----------------------------------+
                                |
                                | WebRTC
                                v
+------------------------------------------------------------------+
|                    LIVEKIT SERVER (Go)                            |
|                   WebRTC SFU - quản lý rooms                     |
+--------------+-----------------------------------+---------------+
               |                                   |
               v                                   v
+--------------------------+       +-------------------------------+
|   AGENT WORKER           |       |   Người tham gia khác         |
|   (Python, ~200-500 LOC) |       |   (người thân, tình nguyện    |
|                          |       |    viên join cùng room)        |
|   Gemini Live            |       +-------------------------------+
|   (RealtimeModel)        |
|                          |
|   Tool Router -----------+--------> Go Backend API
|   Provider Tools:        |              (HTTP)
|   - GoogleSearch         |
|   - Mapbox               |
+--------------------------+
               |
               | HTTP (internal)
               v
+------------------------------------------------------------------+
|                      GO BACKEND API                              |
|                                                                  |
|   Auth | Users | Chat | Calls | Health | Emergency               |
|   Music | Volunteers | Dashboard | Audit                         |
|                                                                  |
|   +------------------+    +------------------+                   |
|   |   PostgreSQL     |    |     Redis        |                   |
|   |   (dữ liệu chính)|    |  (cache, queue)  |                   |
|   +------------------+    +------------------+                   |
+------------------------------------------------------------------+
```

## Tại sao chỉ dùng một kết nối LiveKit duy nhất?

Thay vì tạo nhiều kết nối riêng biệt cho AI voice, camera, và gọi điện, AiSee chỉ
dùng **một kết nối WebRTC duy nhất** qua LiveKit. Đây là quyết định kiến trúc quan
trọng nhất của dự án.

### Vấn đề nếu dùng nhiều kết nối

```
KHÔNG LÀM:                        LÀM:

App --WebRTC--> AI voice           App --WebRTC--> LiveKit Room
App --WebRTC--> Camera AI               |
App --WebRTC--> VoIP calls              +-- AI voice (audio track)
App --HTTP----> Chat                    +-- Camera (video track, on-demand)
                                        +-- VoIP call (người thân join room)
3 kết nối = 3x battery,                +-- Data channel (điều khiển)
3x phức tạp, 3x lỗi
                                   1 kết nối = tiết kiệm pin,
                                   đơn giản, ít lỗi
```

### Lợi ích cụ thể

| Lợi ích | Giải thích |
|---------|------------|
| Tiết kiệm pin | Một kết nối WebRTC thay vì 2-3. Quan trọng cho người dùng lâu |
| Đơn giản hóa code | Một LiveKit client, một room. Không cần quản lý nhiều connection |
| Chuyển đổi mượt | User đang nói chuyện với AI, bảo "gọi con gái" --> người thân join cùng room, không cần tạo kết nối mới |
| Camera linh hoạt | Video track chỉ publish khi cần, unpublish khi xong. Cùng kết nối |
| Data channel có sẵn | LiveKit cung cấp data channel để gửi lệnh (bật camera, phát nhạc, mở bản đồ) mà không cần HTTP riêng |

### Cách nó hoạt động

Mỗi người dùng có một **personal room** trên LiveKit:

```
LiveKit Room: "user_{user_id}"
  |
  +-- Participant: User (Flutter App)
  |     Audio: luôn bật (mic)
  |     Video: tắt mặc định, bật khi cần
  |
  +-- Participant: Agent Worker
  |     Audio: Gemini Live output
  |     Tools: 20 tools
  |
  +-- Participant: Người thân (khi có cuộc gọi)
        Audio: giọng người thân
        Tham gia tạm thời, rời room khi kết thúc
```

## Audio-only mặc định, camera khi cần

Đây là quyết định thiết kế quan trọng thứ hai.

### Tại sao?

| Yếu tố | Audio-only | Audio + Video |
|--------|-----------|---------------|
| Thời hạn session Gemini | **15 phút** | 2 phút |
| Pin tiêu hao | Thấp | Cao (camera + xử lý hình) |
| Token sử dụng | Ít | Nhiều (xử lý frame video) |
| Bandwidth | ~100 kbps | ~1-2 Mbps |

Hầu hết tương tác chỉ cần giọng nói: hỏi đáp, nhắc thuốc, gọi điện, phát nhạc.
Camera chỉ cần cho: đọc nhãn thuốc, mô tả vật thể, nhận diện đường đi.

### Luồng bật/tắt camera

```
Trạng thái bình thường:
  [Audio ON] [Video OFF] --> Session 15 phút

User: "Đọc cái nhãn thuốc này"
  |
  v
Gemini: "Bạn muốn tôi xem qua camera không?"
User: "Có"
  |
  v
  [Audio ON] [Video ON] --> Gemini nhận frame (1 FPS)
  |
  Gemini: "Đây là thuốc Amlodipine 5mg, uống 1 viên mỗi ngày..."
  |
  v
  [Audio ON] [Video OFF] --> Quay lại audio-only

* Lưu ý: Khi video bật, session limit giảm xuống 2 phút.
  Agent Worker theo dõi và tắt camera sớm nếu cần.
```

### Cách bật camera kỹ thuật

```
1. Gemini gọi tool: activate_camera
2. Agent gửi data channel message: { "action": "enable_camera" }
3. App nhận --> bật camera --> publish video track lên LiveKit room
4. Gemini nhận video frames --> xử lý + trả lời
5. Gemini gọi tool: deactivate_camera
6. Agent gửi: { "action": "disable_camera" }
7. App tắt camera --> unpublish video track
```

## Phân tách trách nhiệm

```
+-------------------+-------------------------------------------+
| Component         | Trách nhiệm                               |
+-------------------+-------------------------------------------+
| Flutter App       | - Giao diện (2 UI: user + family)         |
|                   | - LiveKit client (audio/video/data)       |
|                   | - Local notification (nhắc thuốc)          |
|                   | - Local DB cache (tin nhắn, thuốc)         |
|                   | - Audio player (nhạc, radio, podcast)     |
|                   | - Camera capture + publish                |
+-------------------+-------------------------------------------+
| LiveKit Server    | - WebRTC SFU (chuyển tiếp media)          |
|                   | - Room management                         |
|                   | - Auto-dispatch Agent Worker vào room     |
|                   | - Data channel relay                      |
+-------------------+-------------------------------------------+
| Agent Worker      | - Chạy Gemini Live (RealtimeModel)        |
| (Python)          | - Định nghĩa 20 tools                     |
|                   | - Chuyển tool call sang Go Backend (HTTP) |
|                   | - Gửi lệnh đến App qua data channel       |
|                   | - Quản lý session (reconnect, context)    |
|                   | - KHÔNG có business logic                 |
+-------------------+-------------------------------------------+
| Go Backend        | - Auth (Google OAuth2 --> JWT)             |
|                   | - User management, family links           |
|                   | - Chat (messages + WebSocket)             |
|                   | - Call management (LiveKit room create)   |
|                   | - Health (thuốc, triệu chứng, chỉ số)     |
|                   | - Emergency (alert broadcast)             |
|                   | - Music (yt-dlp, radio, queue)            |
|                   | - Volunteer matching                      |
|                   | - Push notification (APNs + FCM)          |
|                   | - Audit log                               |
|                   | - TẤT CẢ business logic ở đây             |
+-------------------+-------------------------------------------+
| PostgreSQL        | - Dữ liệu chính (users, messages, health) |
| Redis             | - Cache, rate limit, music queue, session |
+-------------------+-------------------------------------------+
```

## Hạ tầng và chi phí

| Component | Cấu hình (MVP) | Chi phí/tháng |
|-----------|---------------|---------------|
| VPS (Go Backend + PostgreSQL + Redis) | 2-4 vCPU, 4-8GB RAM | $30-80 |
| LiveKit Server (cùng hoặc riêng VPS) | 2-4 vCPU, 4GB RAM | $30-50 |
| Agent Worker (cùng VPS) | Python process, ~200MB RAM | Đã bao gồm |
| Mapbox | Free tier 25k requests/tháng | $0 |
| Gemini Live API | Free tier | $0 |
| **Tổng** | | **$60-130/tháng** |

Tất cả chạy trên Docker Compose. Không cần Kubernetes cho MVP.
