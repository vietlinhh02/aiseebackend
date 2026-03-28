# Bảo mật và quyền riêng tư

## 1. Luồng xác thực (Authentication Flow)

AiSee dùng Google OAuth2 để đăng nhập, sau đó cấp JWT token riêng cho hệ thống.
Không dùng Firebase Auth.

### Luồng đăng nhập

```
User                Flutter App           Google OAuth2          Go Backend
  |                     |                      |                     |
  | Bấm "Đăng nhập      |                      |                     |
  |  với Google"         |                      |                     |
  +-------------------->|                      |                     |
  |                     | google_sign_in ----->|                     |
  |                     |                      |                     |
  |    [Màn hình chọn   |                      |                     |
  |     tài khoản       |                      |                     |
  |     Google]         |                      |                     |
  |                     |                      |                     |
  | Chọn tài khoản      |                      |                     |
  +-------------------->| <-- Google ID Token  |                     |
  |                     |                      |                     |
  |                     | POST /api/v1/auth/google                   |
  |                     | { google_id_token }  |                     |
  |                     +---------------------------------------------->|
  |                     |                      |                     |
  |                     |                      |  Verify token với   |
  |                     |                      |  Google public keys |
  |                     |                      |<--------------------|
  |                     |                      |  Token hợp lệ       |
  |                     |                      |------------------->|
  |                     |                      |                     |
  |                     |                      |  Tạo/tìm user      |
  |                     |                      |  trong PostgreSQL   |
  |                     |                      |                     |
  |                     |                      |  Cấp JWT token      |
  |                     |                      |  (access + refresh) |
  |                     |                      |                     |
  |                     | <-- { access_token, refresh_token,         |
  |                     |       user } ---------|---------------------|
  |                     |                      |                     |
  |                     | Lưu token vào        |                     |
  |                     | flutter_secure_      |                     |
  |                     | storage (Keychain/   |                     |
  |                     | EncryptedSharedPrefs)|                     |
  |                     |                      |                     |
  |  [Đã đăng nhập]     |                      |                     |
  |<--------------------|                      |                     |
```

### JWT Token

- **Access token:** Thời hạn ngắn (vd: 1 giờ). Đính kèm vào mỗi HTTP request
  trong header `Authorization: Bearer <token>`
- **Refresh token:** Thời hạn dài (vd: 30 ngày). Dùng để lấy access token mới
  khi hết hạn. Lưu an toàn trên device
- **Token revoke:** Khi Google OAuth token bị thu hồi (user xóa quyền), refresh
  fail --> app hiện màn hình đăng nhập lại

### Bảo vệ token trên device

| Platform | Cách lưu | Bảo mật |
|----------|---------|---------|
| iOS | Keychain (qua flutter_secure_storage) | Mã hóa bởi Secure Enclave, xóa khi xóa app |
| Android | EncryptedSharedPreferences (qua flutter_secure_storage) | AES-256, key trong Android Keystore |

## 2. Bảo mật API nội bộ (Internal API Security)

Agent Worker gọi Go Backend qua HTTP nội bộ. Đây là điểm quan trọng vì Agent
có quyền thực hiện hành động thay người dùng.

### 2 lớp bảo vệ

```
+----------------------------------------------------+
|              Docker Network (bridge)               |
|                                                    |
|  Agent Worker ---HTTP---> Go Backend               |
|  (cùng network)          :8080 (internal port)     |
|                                                    |
|  Port 8080 KHÔNG expose ra ngoài Docker network    |
+----------------------------------------------------+
         |
         | Port expose ra ngoài (cho App):
         | chỉ các endpoint /api/v1/*
         v
    Internet (Flutter App kết nối)
```

**Lớp 1: Docker network isolation**
- Agent Worker và Go Backend chạy trong cùng Docker network
- Internal API port (`/api/internal/*`) không expose ra ngoài
- Flutter App chỉ truy cập được `/api/v1/*` (public endpoints)

**Lớp 2: Shared secret**
- Agent Worker gửi header `X-Internal-Key: <secret>` trong mỗi request
- Go Backend verify header này trước khi xử lý
- Secret lưu trong environment variable `INTERNAL_API_KEY`
- Middleware kiểm tra: không có header hoặc sai secret --> 403 Forbidden

```go
// Trong Go Backend middleware
func InternalAuth(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        key := r.Header.Get("X-Internal-Key")
        if key != os.Getenv("INTERNAL_API_KEY") {
            http.Error(w, "Forbidden", 403)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Endpoint phân loại

| Endpoint pattern | Ai truy cập | Xác thực |
|-----------------|------------|---------|
| `/api/v1/*` | Flutter App | JWT token (Bearer header) |
| `/api/internal/*` | Agent Worker | Shared secret (X-Internal-Key header) |
| `/healthz` | Monitoring | Không cần xác thực |

## 3. An toàn Agent (Agent Safety)

Agent Worker có quyền thực hiện hành động thay người dùng (gọi điện, gửi tin nhắn,
cảnh báo khẩn cấp). Cần các lớp bảo vệ để tránh lạm dụng.

### 3.1 Xác nhận trước hành động (Confirmation Flow)

```
+--------------------------------------------------+
|  NGUYÊN TẮC: Mọi hành động đều hỏi xác nhận      |
|  trước khi thực hiện                              |
+--------------------------------------------------+

User: "Gọi cho con gái"
                |
                v
Gemini: "Tôi sẽ gọi cho Lan nhé?"
                |
        +-------+--------+
        |                |
   "Ừ" / "Có"     "Không" / im lặng
        |                |
        v                v
   Thực hiện          Hủy bỏ
   tool call
```

**Ngoại lệ duy nhất:** `emergency_alert` không cần xác nhận khi phát hiện
user đang nguy hiểm (từ khóa: "cứu tôi", "đau quá", "ngã rồi", "không
thấy đường"). Gemini được train để nh���n biết tình huống khẩn cấp.

### 3.2 Contact Whitelist

```
+--------------------------------------------------+
|  CHỈ CÓ THỂ LIÊN LẠC VỚI NGƯỜI ĐÃ ĐĂNG KÝ        |
+--------------------------------------------------+

Tool call: make_voice_call(contact_name="Lan")
                |
                v
Go Backend kiểm tra:
  SELECT * FROM family_links
  WHERE user_id = :user_id
    AND deleted_at IS NULL
                |
        +-------+--------+
        |                |
   Tìm thấy "Lan"  Không tìm thấy
   trong family_links
        |                |
        v                v
   Tạo room,        Trả về lỗi:
   gửi push          "Không tìm thấy
                      liên hệ này"
```

- User chỉ có thể gọi/nhắn tin cho người có trong bảng `family_links`
- Không thể gọi số điện thoại bất kỳ
- Không thể gửi tin nhắn cho người lạ

### 3.3 Rate Limiting

Mỗi tool có giới hạn số lần gọi trong 1 ngày:

| Tool | Giới hạn mặc định | Lý do |
|------|-------------------|-------|
| `make_voice_call` | 10 lần/ngày | Tránh spam gọi điện |
| `send_chat_message` | 50 lần/ngày | Tránh spam tin nhắn |
| `emergency_alert` | 3 lần/ngày | Tránh báo động giả |
| `play_music` | 30 lần/ngày | Hợp lý cho nghe nhạc |
| Các tool khác | 50 lần/ngày | Mặc định |

Thực hiện: Redis counter với TTL 24 giờ

```
Key: ratelimit:{user_id}:{tool_name}
Value: số lần đã gọi
TTL: 86400 giây (24 giờ)

Khi tool call đến:
  INCR ratelimit:{user_id}:{tool_name}
  Nếu giá trị > giới hạn --> từ chối, Gemini nói "Bạn đã thực hiện quá nhiều
  lần hôm nay. Thử lại ngày mai nhé."
```

### 3.4 Audit Log

Mỗi tool call đều được ghi lại, không ngoại lệ.

```sql
INSERT INTO audit_logs (user_id, tool_name, params, result, created_at)
VALUES (:user_id, 'make_voice_call', '{"contact_name": "Lan"}', 'success', now());
```

- Người thân có thể xem audit log qua Family Dashboard
- Endpoint: `GET /api/v1/dashboard/:user_id/audit-log`
- Giúp người thân giám sát hoạt động và phát hiện bất thường
- Audit logs là append-only, không bao giờ xóa

### 3.5 Không truy cập native dialer

```
+--------------------------------------------------+
|  QUAN TRỌNG: App KHÔNG BAO GIỜ gọi ra số điện    |
|  thoại thực (113, 115, số bất kỳ)                |
+--------------------------------------------------+

Lý do:
- Ứng dụng voice-first --> người dùng dễ vô tình kích hoạt
- Gọi 113/115 nhầm gây hậu quả nghiêm trọng
- Người suy giảm thị lực khó kiểm soát

Thay thế:
- Tool call_emergency_service đã BỊ XÓA
- Emergency flow chỉ dùng IN-APP:
  push notification đến tất cả người thân + GPS
  + tạo LiveKit room để người thân gọi lại
```

## 4. Quyền riêng tư dữ liệu (Data Privacy)

### Dữ liệu lưu trữ trong hệ thống

| Dữ liệu | Nơi lưu | Bảo mật | Thời gian giữ |
|---------|---------|---------|---------------|
| Thông tin user (tên, email, avatar) | PostgreSQL | Mã hóa at-rest (nếu VPS hỗ trợ) | Vĩnh viễn (hoặc khi user xóa tài khoản) |
| Tin nhắn chat | PostgreSQL | Không mã hóa end-to-end (MVP) | Append-only, không xóa |
| Voice messages | Object storage (URL trong DB) | URL có thời hạn | Append-only |
| Lịch sử thuốc, triệu chứng, chỉ số sức khỏe | PostgreSQL | Chỉ user và family link xem được | Append-only |
| Audit log (tool call) | PostgreSQL | Chỉ user và family link xem được | Append-only |
| Vị trí GPS | Chỉ gửi khi emergency alert | Không lưu trữ lâu dài | Gửi trong push, không lưu DB |
| JWT token | Device (Keychain/EncryptedSharedPrefs) | Mã hóa | Refresh: 30 ngày |
| Session context | Redis | Không mã hóa | TTL tự động xóa |

### Dữ liệu gửi đến Google/Gemini

| Dữ liệu | Khi nào gửi | Google lưu không? |
|---------|------------|------------------|
| Giọng nói (audio stream) | Mỗi khi nói chuyện với AI | Theo policy Gemini API -- không lưu cho model miễn phí (*) |
| Hình ảnh camera (video frames) | Chỉ khi camera bật (on-demand) | Theo policy Gemini API |
| Nội dung tool call (tên liên hệ, thuốc, ...) | Khi Gemini gọi tool | Theo policy Gemini API |
| Google ID token | Khi đăng nhập | Google lưu (OAuth2 standard) |
| FCM token | Khi đăng ký push (Android) | Google lưu (FCM standard) |

(*) Cần verify với Gemini API Terms of Service. Với free tier, Google có thể
dùng dữ liệu để cải thiện model. Cần cân nhắc upgrade lên paid tier nếu cần
đảm bảo dữ liệu không bị dùng.

### Dữ liệu KHÔNG gửi ra ngoài

- Danh bạ điện thoại (chỉ đọc local để match tên)
- GPS vị trí (chỉ gửi khi emergency alert, không tracking liên tục)
- Lịch sử sử dụng app (không analytics/tracking)
- Nội dung tin nhắn giữa user và người thân (chỉ lưu DB riêng, không gửi cho Google)

### Quyền truy cập dữ liệu

```
+-------------------------------------------------------------------+
|                     Ai xem được gì?                               |
+-------------------------------------------------------------------+
|                                                                   |
|  User (người dùng):                                               |
|  - Tất cả dữ liệu của mình                                        |
|  - Tin nhắn với người thân (chỉ người đã link)                    |
|                                                                   |
|  Family member (người thân):                                      |
|  - Tin nhắn với user đã link                                      |
|  - Health data của user đã link (qua Dashboard)                   |
|  - Audit log của user đã link (qua Dashboard)                     |
|  - Vị trí cuối cùng (chỉ khi có emergency)                        |
|  - KHÔNG thể xem dữ liệu của user không link                      |
|                                                                   |
|  Volunteer (tình nguyện viên):                                    |
|  - KHÔNG xem được dữ liệu của user                                |
|  - Chỉ tham gia cuộc gọi khi được match                           |
|                                                                   |
|  Agent Worker:                                                    |
|  - Truy cập internal API (với shared secret)                      |
|  - Thực hiện tool call THAY user (có audit log)                   |
|  - Không lưu dữ liệu riêng -- mọi thứ qua Go Backend              |
|                                                                   |
+-------------------------------------------------------------------+
```

### Kiểm tra quyền truy cập Family Dashboard

```
GET /api/v1/dashboard/:user_id/health

Go Backend:
  1. Verify JWT --> lấy requester.id
  2. SELECT * FROM family_links
     WHERE family_id = requester.id
       AND user_id = :user_id
       AND deleted_at IS NULL
  3. Không tìm thấy --> 403 Forbidden
  4. Tìm thấy --> trả về dữ liệu
```

## 5. Bảo mật truyền tải (Transport Security)

| Kết nối | Giao thức | Mã hóa |
|---------|----------|--------|
| App --> Go Backend | HTTPS (TLS 1.2+) | Có |
| App --> LiveKit Server | WebRTC (DTLS + SRTP) | Có (built-in) |
| Agent --> Go Backend | HTTP (trong Docker network) | Không cần -- cùng máy, mạng nội bộ |
| Go Backend --> PostgreSQL | TCP (trong Docker network) | Không cần -- cùng máy |
| Go Backend --> Redis | TCP (trong Docker network) | Không cần -- cùng máy |
| Go Backend --> APNs | HTTPS (TLS) | Có |
| Go Backend --> FCM | HTTPS (TLS) | Có |

## 6. Tổng hợp các lớp bảo mật

```
Lớp 1: Transport
  HTTPS (TLS) cho mọi kết nối ra ngoài
  WebRTC (DTLS + SRTP) cho audio/video

Lớp 2: Authentication
  Google OAuth2 --> JWT token
  Mỗi request cần JWT hợp lệ

Lớp 3: Authorization
  Role-based (user / family)
  Family link check cho Dashboard
  Contact whitelist cho gọi điện/nhắn tin

Lớp 4: Internal Security
  Docker network isolation
  Shared secret cho internal API
  Port không expose ra ngoài

Lớp 5: Agent Safety
  Confirmation flow (hỏi trước khi làm)
  Rate limiting (giới hạn số lần/ngày)
  Audit log (ghi lại tất cả)
  Không native dialer access

Lớp 6: Data Protection
  Token lưu trong Keychain/EncryptedSharedPrefs
  Soft-delete (không xóa thật, có thể khôi phục)
  Append-only logs (không sửa đổi)
```
