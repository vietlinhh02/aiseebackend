# Tech Stack

## Tổng quan lựa chọn công nghệ

AiSee dùng 4 ngôn ngữ/framework chính: Flutter (app), Go (backend), Python (agent),
và PostgreSQL/Redis (data). Mỗi lựa chọn đều có lý do kỹ thuật cụ thể.

## Flutter App

| Mục đích | Package | Tại sao |
|----------|---------|---------|
| Cross-platform | `flutter` | Một codebase cho iOS + Android. Đội nhỏ, không đủ nguồn lực làm 2 native app. Flutter có hệ sinh thái tốt cho accessibility |
| State management | `riverpod` | Type-safe, testable, không boilerplate. Phổ biến nhất trong Flutter ecosystem hiện tại |
| LiveKit client | `livekit_client` | SDK chính thức của LiveKit cho Flutter. Hỗ trợ WebRTC, data channel, video track |
| Native call UI | `flutter_callkit_incoming` | Hiển thị màn hình gọi đến như cuộc gọi bình thường (CallKit trên iOS, ConnectionService trên Android) |
| Local notification | `flutter_local_notifications` | Nhắc thuốc, lịch hẹn -- hoạt động offline, không cần server |
| Chat UI | `flutter_chat_ui` | UI chat có sẵn, tiết kiệm thời gian. Hỗ trợ voice message bubble |
| Local DB | `drift` + `sqlite3_flutter_libs` | SQLite ORM cho Flutter. Cache tin nhắn, thuốc, contacts để dùng offline |
| HTTP client | `dio` | HTTP client mạnh hơn `http` package. Interceptors (auth token), retry, logging |
| Audio player | `just_audio` | Phát nhạc, radio, podcast. Hỗ trợ streaming, queue, background playback |
| Camera | `camera` | Package chính thức của Flutter. Chụp frame gửi lên LiveKit video track |
| Contacts | `flutter_contacts` | Đọc danh bạ điện thoại (với quyền) để match với family link |
| Permissions | `permission_handler` | Quản lý quyền camera, mic, contacts, notification -- một API thống nhất |
| Maps | `mapbox_maps_flutter` | Mapbox thay Google Maps. Free tier 25k requests/tháng. Không phụ thuộc Google |
| Auth | `google_sign_in` | Google OAuth2 trực tiếp, không qua Firebase. Đơn giản, 1 bước đăng nhập |
| Secure storage | `flutter_secure_storage` | Lưu JWT token an toàn (Keychain trên iOS, EncryptedSharedPreferences trên Android) |
| Deep links | `url_launcher` | Mở URL, deep link cho invite family |

### Tại sao Flutter mà không phải native?

- Đội chỉ có 3-5 người, không đủ nguồn lực làm 2 app riêng
- Flutter có `Semantics` widget tốt cho accessibility (VoiceOver, TalkBack)
- LiveKit có SDK chính thức cho Flutter
- Performance đủ tốt cho app voice-first (không cần animation phức tạp)

## LiveKit Server

| Mục đích | Chi tiết | Tại sao |
|----------|----------|---------|
| WebRTC SFU | `livekit-server` (Go binary) | Open-source, self-hosted, miễn phí. Thay thế cho Twilio/Agora với chi phí $0 |
| Room management | Built-in | Mỗi user có personal room. Người thân join cùng room khi gọi điện |
| Agent dispatch | Built-in | Tự động gán Agent Worker vào room khi user join |
| Data channel | Built-in | Gửi lệnh giữa Agent và App (bật camera, phát nhạc, ...) |

### Tại sao LiveKit mà không phải Twilio/Agora?

- **Miễn phí** khi self-hosted (Twilio/Agora tính phí theo phút)
- **Open-source** -- đọc code, debug, không bị vendor lock-in
- **Agent framework** có sẵn -- Agent Worker tích hợp trực tiếp, không cần tạo pipe riêng
- **Go native** -- LiveKit viết bằng Go, backend cũng Go, SDK server tương thích tốt
- Nhược điểm: phải tự quản lý server (Docker), nhưng chấp nhận được cho MVP

## Agent Worker (Python)

| Mục đích | Package | Tại sao |
|----------|---------|---------|
| Agent framework | `livekit-agents` | SDK chính thức của LiveKit để viết agent. Chỉ có Python SDK hỗ trợ đầy đủ |
| Gemini Live | `livekit-plugins-google` | Plugin chính thức kết nối Gemini Live API qua LiveKit |
| Google Search | Built-in trong plugin | Gemini có thể tìm kiếm thông tin khi cần |
| HTTP client | `httpx` | Async HTTP client. Agent gọi Go Backend qua HTTP để thực hiện tool |

### Tại sao Python mà không phải Go?

- **LiveKit Agent SDK chỉ hỗ trợ đầy đủ trên Python.** Go SDK chưa hỗ trợ Realtime Model
- Agent Worker rất mỏng (~200-500 dòng code), chỉ làm 2 việc:
  1. Chạy Gemini Live session
  2. Chuyển tool call sang Go Backend
- Không có business logic trong Python -- tất cả logic nằm trong Go Backend
- Python process chỉ dùng ~200MB RAM, chạy cùng VPS với Go Backend

## Go Backend

| Mục đích | Package/Tool | Tại sao |
|----------|-------------|---------|
| HTTP framework | `chi` router | Nhẹ, nhanh, idiomatic Go. Không cần framework nặng như Gin/Echo |
| WebSocket | `nhooyr.io/websocket` | WebSocket cho chat real-time. API đơn giản, hỗ trợ context cancellation |
| Database | `pgx` + `sqlc` | `pgx` là PostgreSQL driver nhanh nhất cho Go. `sqlc` generate type-safe Go code từ SQL -- không cần ORM |
| Migrations | `golang-migrate` | SQL migration files, chạy từ command line hoặc trong code |
| Redis | `go-redis/redis` | Client Redis phổ biến nhất cho Go. Connection pool, pipeline |
| LiveKit SDK | `livekit/server-sdk-go` | Tạo room, cấp token, quản lý participant từ backend |
| Push iOS | `sideshow/apns2` | APNs HTTP/2 client. Gửi push trực tiếp, không qua Firebase |
| Push Android | FCM HTTP v1 API (`net/http`) | Gửi FCM qua HTTP API, không cần Firebase SDK |
| Auth | Google OAuth2 verify + `golang-jwt/jwt` | Verify Google token trực tiếp, cấp JWT cho app |
| Music | `yt-dlp` subprocess | Tìm và lấy audio URL từ YouTube/radio. Chạy như subprocess |
| Config | env vars | Đơn giản, tương thích Docker. Không cần config file phức tạp |
| Logging | `slog` (stdlib) | Structured logging có sẵn trong Go stdlib từ 1.21. JSON output cho Docker logs |

### Tại sao Go?

- **LiveKit viết bằng Go** -- SDK server native, tương thích tốt
- **Nhẹ** -- binary nhỏ (~15MB), khởi động nhanh (<1s), ít RAM (~50MB)
- **Concurrency** -- goroutines cho WebSocket hub, push notification, volunteer matching
- **Deploy đơn giản** -- 1 binary, không cần runtime
- **Đội đã quen** -- (giả sử đội có kinh nghiệm Go)

## Database

### PostgreSQL

| Mục đích | Tại sao |
|----------|---------|
| Dữ liệu chính | Users, messages, health records, audit logs, family links |
| JSONB | Lưu settings linh hoạt (user preferences) |
| UUID | Primary key cho tất cả bảng -- không lo trùng khi scale |
| Indexes | Tối ưu query cho các use case chính (unread messages, medication by user, audit log) |

### Redis

| Mục đích | Key pattern | Tại sao |
|----------|------------|---------|
| Music queue | `music:queue:{user_id}` | Queue tạm thời, không cần lưu vào DB |
| Rate limiting | `ratelimit:{user_id}:{tool_name}` | Counter với TTL, tự động reset |
| Room metadata | `room:{room_id}` | Thông tin phòng, TTL=20 phút |
| Session context | `session:{user_id}` | Lưu lịch sử hội thoại (10 turn cuối) cho reconnect |

Redis là **optional** -- nếu Redis chết, app vẫn chạy. Music queue dùng in-memory,
rate limit bỏ qua, session context mất khi reconnect.

## Infrastructure

```
+--------------------------------------------------+
|              Docker Compose                      |
|                                                  |
|  +-----------+  +-----------+  +------------+   |
|  | Go Backend|  | LiveKit   |  | Agent      |   |
|  | :8080     |  | Server    |  | Worker     |   |
|  |           |  | :7880     |  | (Python)   |   |
|  +-----------+  +-----------+  +------------+   |
|        |                                         |
|  +-----------+  +-----------+                    |
|  | PostgreSQL|  |   Redis   |                    |
|  | :5432     |  |   :6379   |                    |
|  +-----------+  +-----------+                    |
+--------------------------------------------------+
```

| Component | Cấu hình MVP | Chi phí/tháng |
|-----------|-------------|---------------|
| VPS chính | 2-4 vCPU, 4-8GB RAM | $30-80 |
| LiveKit VPS | 2-4 vCPU, 4GB RAM (có thể cùng VPS) | $30-50 |
| Mapbox | Free tier 25k req/tháng | $0 |
| Gemini Live API | Free tier | $0 |
| UptimeRobot | Free tier (monitoring) | $0 |
| **Tổng** | | **$60-130/tháng** |

## Những thứ KHÔNG dùng và tại sao

| Công nghệ | Thay thế bằng | Lý do |
|-----------|-------------|-------|
| Firebase SDK | Google OAuth2 trực tiếp + APNs/FCM HTTP | Giảm phụ thuộc Google. Firebase SDK nặng, thêm phức tạp. Push gửi trực tiếp từ backend |
| Google Maps | Mapbox | Free tier tốt hơn (25k req/tháng). Không thêm dependency Google |
| Google ML Kit | Gemini Vision (qua LiveKit) | Gemini đã có khả năng nhìn qua camera. Không cần SDK riêng |
| Firebase Auth | Google OAuth2 + JWT tự cấp | Đơn giản hơn. Không cần Firebase console |
| Firestore | PostgreSQL | SQL quen thuộc hơn, query mạnh hơn, miễn phí (self-hosted) |
| ORM (GORM, Ent) | `sqlc` (SQL --> Go code) | Không có "magic". Viết SQL trực tiếp, sqlc generate type-safe code |
| gRPC (Agent --> Backend) | HTTP REST | Agent Worker chỉ gọi ~21 endpoint. HTTP đơn giản, dễ debug (curl), không cần protobuf |
| Kubernetes | Docker Compose | MVP chỉ cần 1-2 VPS. Kubernetes quá phức tạp cho quy mô này |
