# API Reference

## Tổng quan

Base URL: `http://localhost:8080`

Tất cả response dung JSON. Tất cả endpoints (tru healthcheck và auth) yêu cầu JWT trong header:

```
Authorization: Bearer <jwt-token>
```

Internal endpoints (Agent Worker -> Backend) dung shared secret:

```
X-Internal-Key: <INTERNAL_API_KEY>
```

### Response format chuan

**Thành công:**

```json
{
  "data": { ... }
}
```

**Loi:**

```json
{
  "error": {
    "code": "INVALID_TOKEN",
    "message": "Token has expired"
  }
}
```

---

## Auth

### POST /api/v1/auth/google

Xac thuc bang Google ID token, trả về JWT.

| | |
|---|---|
| Auth | Khong can |
| Request body | `{ "id_token": "string", "role": "user" \| "family" }` |
| Response 200 | `{ "data": { "access_token": "jwt", "refresh_token": "jwt", "user": { "id", "email", "display_name", "role" } } }` |
| Response 401 | `{ "error": { "code": "INVALID_GOOGLE_TOKEN", "message": "..." } }` |

`role` chi can truyen lan dau đăng ký. Lan đăng nhập sau, role lấy tu database.

### POST /api/v1/auth/refresh

Refresh JWT khi access token hết hạn.

| | |
|---|---|
| Auth | Khong can (refresh token trong body) |
| Request body | `{ "refresh_token": "string" }` |
| Response 200 | `{ "data": { "access_token": "jwt", "refresh_token": "jwt" } }` |
| Response 401 | `{ "error": { "code": "INVALID_REFRESH_TOKEN", "message": "..." } }` |

---

## Users

### GET /api/v1/users/me

Lấy thông tin user hiện tại.

| | |
|---|---|
| Auth | JWT required |
| Response 200 | `{ "data": { "id", "email", "display_name", "role", "is_volunteer", "avatar_url", "settings", "created_at" } }` |

### PUT /api/v1/users/me

Cap nhat thông tin user.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "display_name?": "string", "avatar_url?": "string", "settings?": {}, "fcm_token?": "string", "apns_token?": "string", "apns_voip_token?": "string" }` |
| Response 200 | `{ "data": { ...updated user } }` |

### POST /api/v1/users/family-link

Tạo invite code de mỗi người thân.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "relationship": "string" }` |
| Response 200 | `{ "data": { "invite_code": "ABC123", "expires_at": "timestamp" } }` |

### POST /api/v1/users/join-family

Tham gia family bang invite code.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "invite_code": "string" }` |
| Response 200 | `{ "data": { "family_link": { "id", "user_id", "family_id", "relationship" } } }` |
| Response 404 | `{ "error": { "code": "INVALID_INVITE_CODE", "message": "..." } }` |

### GET /api/v1/users/me/family

Lấy danh sách family members của user hiện tại.

| | |
|---|---|
| Auth | JWT required |
| Response 200 | `{ "data": { "family": [{ "id", "display_name", "relationship", "is_primary_contact" }] } }` |

### DELETE /api/v1/users/family-link/:id

Xóa family link (soft-delete).

| | |
|---|---|
| Auth | JWT required |
| Params | `id` — family_link ID |
| Response 200 | `{ "data": { "deleted": true } }` |
| Response 403 | Nếu user không so huu family link này |

---

## Chat

### GET /api/v1/messages/:contact_id

Lấy lịch sử tin nhắn với mot contact.

| | |
|---|---|
| Auth | JWT required |
| Params | `contact_id` — UUID của contact |
| Query | `limit` (default 50, max 100), `before` (cursor UUID cho pagination) |
| Response 200 | `{ "data": { "messages": [{ "id", "sender_id", "receiver_id", "content_text", "voice_url", "is_read", "created_at" }] } }` |
| Response 403 | Nếu không có family_link với contact này |

### POST /api/v1/messages/send

Gửi tin nhắn.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "receiver_id": "uuid", "content_text?": "string", "voice_url?": "string" }` |
| Response 200 | `{ "data": { "message": { "id", "sender_id", "receiver_id", "content_text", "voice_url", "created_at" } } }` |

Gửi push notification toi nguoi nhận. Ít nhất mot trong `content_text` hoặc `voice_url` phải có giá trị.

### PUT /api/v1/messages/:id/read

Danh dau tin nhắn đã đọc.

| | |
|---|---|
| Auth | JWT required |
| Params | `id` — message ID |
| Response 200 | `{ "data": { "updated": true } }` |

### GET /api/v1/messages/unread-count

Đếm tin nhắn chưa đọc.

| | |
|---|---|
| Auth | JWT required |
| Response 200 | `{ "data": { "count": 5 } }` |

### WS /api/v1/ws/chat

WebSocket cho real-time message delivery.

| | |
|---|---|
| Auth | JWT trong query param `?token=<jwt>` |
| Protocol | JSON messages qua WebSocket |

Server push message mỗi toi client khi co tin nhắn đến:

```json
{
  "type": "new_message",
  "message": { "id", "sender_id", "content_text", "voice_url", "created_at" }
}
```

---

## Calls (LiveKit)

### POST /api/v1/calls/token

Lấy LiveKit room token cho user hiện tại.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "room_name?": "string" }` |
| Response 200 | `{ "data": { "token": "livekit-jwt", "url": "ws://livekit:7880" } }` |

Nếu không truyen `room_name`, tạo personal room `user:<user_id>`.

### POST /api/v1/calls/create-room

Tao LiveKit room và gửi push notification cho nguoi nhận.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "callee_id": "uuid", "call_type": "voice" \| "video" }` |
| Response 200 | `{ "data": { "room_name": "string", "token": "livekit-jwt" } }` |

Backend sẽ:
1. Tao LiveKit room
2. Gửi VoIP Push (iOS) hoặc FCM high-priority (Android) toi callee
3. Tra room token cho caller

### POST /api/v1/calls/end-room

Ket thuc LiveKit room.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "room_name": "string" }` |
| Response 200 | `{ "data": { "ended": true } }` |

---

## Emergency

### POST /api/v1/emergency/alert

Gửi SOS toi tất cả family members.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "location?": { "lat": number, "lng": number } }` |
| Response 200 | `{ "data": { "alerted_count": 3, "room_name": "emergency-<uuid>" } }` |

Backend sẽ:
1. Gửi push notification (co GPS) toi tất cả family members
2. Tao LiveKit room de family có thể goi lai ngay

### POST /api/v1/emergency/escalate

Tim tình nguyện viên và tao room.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "reason?": "string" }` |
| Response 200 | `{ "data": { "match_id": "uuid", "room_id": "uuid" } }` |

Trả về ngay lap tuc. Matching dien ra async (xem [Volunteer Matching](#post-apiv1volunteersmatch)).

---

## Health

### CRUD /api/v1/health/medications

| Method | Path | Mo ta |
|--------|------|-------|
| GET | `/api/v1/health/medications` | Lấy danh sách thuoc (active) |
| POST | `/api/v1/health/medications` | Thêm thuoc mỗi |
| PUT | `/api/v1/health/medications/:id` | Cap nhat thuoc |
| DELETE | `/api/v1/health/medications/:id` | Soft-delete thuoc |

**POST request body:**

```json
{
  "name": "Amlodipine",
  "dosage": "5mg",
  "times": ["08:00", "20:00"],
  "frequency": "daily"
}
```

### POST /api/v1/health/medications/:id/taken

Danh dau da uống thuốc.

| | |
|---|---|
| Auth | JWT required |
| Response 200 | `{ "data": { "logged": true, "taken_at": "timestamp" } }` |

### POST /api/v1/health/medications/:id/skipped

Danh dau bỏ qua thuoc.

| | |
|---|---|
| Auth | JWT required |
| Response 200 | `{ "data": { "logged": true, "skipped": true } }` |

### CRUD /api/v1/health/symptoms

| Method | Path | Mo ta |
|--------|------|-------|
| GET | `/api/v1/health/symptoms` | Lấy lịch sử triệu chứng |
| POST | `/api/v1/health/symptoms` | Ghi triệu chứng mỗi |

**POST request body:**

```json
{
  "symptom": "dau dau",
  "severity": "vua",
  "notes": "sau khi an trua"
}
```

### CRUD /api/v1/health/metrics

| Method | Path | Mo ta |
|--------|------|-------|
| GET | `/api/v1/health/metrics` | Lấy lịch sử chỉ số |
| POST | `/api/v1/health/metrics` | Ghi chỉ số mỗi |

**POST request body:**

```json
{
  "metric_type": "blood_pressure",
  "value": "120/80",
  "unit": "mmHg"
}
```

### CRUD /api/v1/health/appointments

| Method | Path | Mo ta |
|--------|------|-------|
| GET | `/api/v1/health/appointments` | Lấy lịch hẹn |
| POST | `/api/v1/health/appointments` | Thêm lịch hẹn |
| PUT | `/api/v1/health/appointments/:id` | Cap nhat lịch hẹn |
| DELETE | `/api/v1/health/appointments/:id` | Soft-delete lịch hẹn |

**POST request body:**

```json
{
  "title": "Kham mat dinh ky",
  "location": "Benh vien Mat TP.HCM",
  "datetime": "2026-04-15T09:00:00+07:00",
  "notes": "Mang theo so kham benh"
}
```

### GET /api/v1/health/summary

Tổng hợp sức khỏe trong khoang thời gian.

| | |
|---|---|
| Auth | JWT required |
| Query | `user_id` (required cho family dashboard), `period` ("week" \| "month") |
| Response 200 | `{ "data": { "medications_taken": 14, "medications_missed": 2, "symptoms": [...], "metrics": [...] } }` |

---

## Music

### GET /api/v1/music/radio-stations

Lấy danh sách dai radio được cấu hình san.

| | |
|---|---|
| Auth | JWT required |
| Response 200 | `{ "data": { "stations": [{ "id", "name", "url", "description" }] } }` |

Ví dụ stations: VOV1, VOV3, VOH.

### GET /api/v1/music/podcasts

Lấy danh sách podcast feeds.

| | |
|---|---|
| Auth | JWT required |
| Response 200 | `{ "data": { "podcasts": [{ "id", "title", "feed_url", "episodes": [...] }] } }` |

### POST /api/v1/music/queue/add

Thêm bai hat vao queue của user.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "query": "string", "source?": "youtube" \| "radio" \| "podcast" }` |
| Response 200 | `{ "data": { "track": { "title", "url", "duration" } } }` |

### GET /api/v1/music/queue/:user_id

Lấy queue hiện tại của user.

| | |
|---|---|
| Auth | JWT required |
| Response 200 | `{ "data": { "queue": [{ "title", "url", "duration" }] } }` |

### DELETE /api/v1/music/queue/:user_id

Xóa toan bo queue.

| | |
|---|---|
| Auth | JWT required |
| Response 200 | `{ "data": { "cleared": true } }` |

---

## Volunteers

### POST /api/v1/volunteers/register

Dang ky lam tình nguyện viên.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "languages?": ["vi", "en"] }` |
| Response 200 | `{ "data": { "registered": true } }` |

### PUT /api/v1/volunteers/availability

Cap nhat trạng thái sẵn sàng.

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "available": true }` |
| Response 200 | `{ "data": { "available": true } }` |

### POST /api/v1/volunteers/match

Tim tình nguyện viên phu hop (async matching).

| | |
|---|---|
| Auth | JWT required |
| Request body | `{ "reason?": "string" }` |
| Response 200 | `{ "data": { "match_id": "uuid", "room_id": "uuid" } }` |

Trả về ngay. Backend tim volunteer trong background. Poll trạng thái qua:

### GET /api/v1/volunteers/match/:match_id/status

| | |
|---|---|
| Auth | JWT required |
| Response 200 (dang tìm) | `{ "data": { "status": "searching", "attempt": 2 } }` |
| Response 200 (da tìm) | `{ "data": { "status": "matched", "volunteer_name": "...", "room_token": "..." } }` |
| Response 200 (thất bại) | `{ "data": { "status": "failed" } }` |

---

## Family Dashboard

Tất cả endpoints yêu cầu: (1) JWT hợp lệ, (2) `family_links` row ton tại giua requester và `user_id`.

### GET /api/v1/dashboard/:user_id/health

Tổng hợp sức khỏe của user.

| | |
|---|---|
| Auth | JWT required + family link authorization |
| Response 200 | `{ "data": { "medications": [...], "recent_symptoms": [...], "recent_metrics": [...] } }` |

### GET /api/v1/dashboard/:user_id/audit-log

Xem lịch sử tool calls của user.

| | |
|---|---|
| Auth | JWT required + family link authorization |
| Query | `limit` (default 50), `before` (cursor) |
| Response 200 | `{ "data": { "logs": [{ "tool_name", "params", "result", "created_at" }] } }` |

### GET /api/v1/dashboard/:user_id/medications

Danh sách thuoc của user (cho family quản lý).

| | |
|---|---|
| Auth | JWT required + family link authorization |
| Response 200 | `{ "data": { "medications": [{ "id", "name", "dosage", "times", "frequency", "active" }] } }` |

### GET /api/v1/dashboard/:user_id/location

Vi tri GPS cuoi cung của user.

| | |
|---|---|
| Auth | JWT required + family link authorization |
| Response 200 | `{ "data": { "location": { "lat", "lng", "updated_at" } } }` |

---

## Internal (Agent Worker -> Backend)

Chi accessible trong Docker network. Khong expose ra ngoai.

### POST /api/internal/tool-call

Agent Worker goi tool thong qua endpoint này.

| | |
|---|---|
| Auth | `X-Internal-Key` header |
| Request body | `{ "tool": "make_voice_call", "user_id": "uuid", "params": { "contact_name": "con gai" } }` |
| Response 200 | `{ "data": { "result": { ... } } }` |
| Response 400 | `{ "error": { "code": "UNKNOWN_TOOL", "message": "..." } }` |

Backend route tool call toi service tuong ung, ghi audit log, và tra kết quả.

### POST /api/internal/push

Gửi push notification.

| | |
|---|---|
| Auth | `X-Internal-Key` header |
| Request body | `{ "user_id": "uuid", "title": "string", "body": "string", "type": "normal" \| "voip", "data?": {} }` |
| Response 200 | `{ "data": { "sent": true } }` |

---

## Health Check

### GET /healthz

| | |
|---|---|
| Auth | Khong can |
| Response 200 | `{ "status": "ok", "db": "connected", "redis": "connected" }` |
| Response 503 | `{ "status": "unhealthy", "db": "disconnected" }` |
