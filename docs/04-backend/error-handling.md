# Error Handling

## Nguyên tắc cot lỗi

**Voi nguoi suy giảm thị lực, lỗi im lang la không chap nhan được.** Mỗi lỗi phai tạo ra mot phản hồi bang giọng nói. User không doc được man hinh lỗi — ho can nghe được chuyen gi dang xay ra.

## Standard Error Response Format

Tất cả API errors trả về cung format JSON:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Mo ta loi doc duoc"
  }
}
```

| Field | Mo ta |
|-------|-------|
| `code` | Ma lỗi viet hoa, dùng de xử lý trong code. Ví dụ: `INVALID_TOKEN`, `NOT_FOUND` |
| `message` | Mo ta lỗi doc được cho developer. KHONG hiển thị truc tiep cho user. |

### HTTP Status Codes

| Status | Khi nào |
|--------|---------|
| 200 | Thành công |
| 400 | Request không hợp lệ (thieu field, sai format) |
| 401 | Chưa xác thực hoặc token hết hạn |
| 403 | Khong co quyen (ví dụ: truy cập dashboard của user không phai family) |
| 404 | Resource không tim thay |
| 429 | Rate limited |
| 500 | Loi server nội bộ |

### Ví dụ response

**400 Bad Request:**

```json
{
  "error": {
    "code": "MISSING_FIELD",
    "message": "Field 'contact_name' is required"
  }
}
```

**401 Unauthorized:**

```json
{
  "error": {
    "code": "TOKEN_EXPIRED",
    "message": "Access token has expired. Use refresh token to get a new one."
  }
}
```

**403 Forbidden:**

```json
{
  "error": {
    "code": "NO_FAMILY_LINK",
    "message": "You are not authorized to view this user's data"
  }
}
```

**500 Internal Server Error:**

```json
{
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "An unexpected error occurred"
  }
}
```

Loi 500 KHONG BAO GIO chưa chi tiết nội bộ (stack trace, query, etc.). Chi tra message chung.

## Error Categories

Bang danh mục lỗi và cach xử lý cho tung thành phần trong hệ thống:

### Network & Connection Errors

| Loi | Phat hien | User nghe được |
|-----|-----------|----------------|
| Khong co internet | App kiểm tra connectivity luc khởi động | "Ban dang offline. Toi chi có thể nhắc thuốc và doc tin nhắn da lưu." |
| LiveKit kết nối thất bại | Connect timeout 10 giây | "Khong kết nối được. Dang thử lại..." (3 lan, sau đó: "Vui long kiểm tra internet.") |
| LiveKit mat kết nối giua session | Disconnect event | Tu dong reconnect (built-in). Nếu thất bại sau 3 lan: thông báo bang giọng nói |

### AI & Agent Errors

| Loi | Phat hien | User nghe được |
|-----|-----------|----------------|
| Gemini session error | Agent Worker exception | Im lang 1-3 giây, agent tao session mỗi và tiep tuc. User không cần biet. |
| Tool call toi Backend thất bại | HTTP 5xx hoặc timeout 10 giây | "Toi không thực hiện được. Ban thử lại sau nhe." Ghi vao audit log. |

### Auth Errors

| Loi | Phat hien | User nghe được |
|-----|-----------|----------------|
| Google OAuth token bi revoke | 401 tu token refresh | "Ban can đăng nhập lai." App hien login screen. |
| JWT hết hạn | 401 tu API request | App tự động refresh. Nếu refresh cung het: đăng nhập lai. |

### Device Permission Errors

| Loi | Phat hien | User nghe được |
|-----|-----------|----------------|
| Camera permission bi tu choi | App ACK với error | "Ban can cho phep camera trong cài đặt." |
| Microphone permission bi tu choi | LiveKit connect fail | "Ban can cho phep microphone de toi có thể nghe ban." |

### Push Notification Errors

| Loi | Phat hien | User nghe được |
|-----|-----------|----------------|
| Push delivery thất bại | Backend nhan lỗi tu APNs/FCM | Khong co — user không biet. Backend retry 3 lan với exponential backoff. Log warning. |
| Incoming call app da bi kill | VoIP Push / FCM không đến | Khong co fallback — phụ thuộc OS push delivery. |

## Logging Strategy

### Tool: slog (Go stdlib)

Su dung `slog` (có sẵn tu Go 1.21+) với JSON format, output ra stdout.

### Cấu hình

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
slog.SetDefault(logger)
```

### Log levels

| Level | Khi nào dùng | Ví dụ |
|-------|-------------|-------|
| `ERROR` | Loi can xử lý, anh huong den user | Database connection lost, push delivery failed |
| `WARN` | Vấn đề tiem an nhưng chưa anh huong | Token expired, retry succeeded |
| `INFO` | Su kien quan trọng trong flow bình thường | User login, tool call executed, room created |
| `DEBUG` | Chi tiết cho debugging (tắt tren production) | Request/response bodies, SQL queries |

### Format log

Mỗi log entry la mot JSON object:

```json
{
  "time": "2026-03-28T10:15:30.123+07:00",
  "level": "ERROR",
  "msg": "push notification failed",
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "push_type": "apns",
  "error": "BadDeviceToken",
  "attempt": 3
}
```

### Quy tắc logging

1. **Luon co context.** Log phải có du thông tin de debug: user_id, request_id, tool_name, etc.
2. **Khong log sensitive data.** Khong log JWT token, password, Google ID token, push token.
3. **Khong log request/response body tren production.** Chi log o DEBUG level.
4. **Mỗi error phải có log.** Khong bao gio catch error ma không log.
5. **Structured logging only.** Dùng `slog.With()` de thêm fields, không format string.

### Ví dụ

```go
// Tot
slog.Error("tool call failed",
    "user_id", userID,
    "tool", toolName,
    "error", err.Error(),
)

// Khong tot — thieu context, dung format string
slog.Error(fmt.Sprintf("error: %v", err))
```

## Nguyên tắc "Khong im lang"

Danh sách kiểm tra cho mỗi feature:

1. Mỗi HTTP handler co error response ro rang (không tra 200 với body rong)
2. Mỗi error tu database/external service được log và tra response phu hop
3. Agent Worker bật mỗi exception và thông báo user bang giọng nói
4. Push notification failures được retry trước khi bỏ qua
5. App hiển thị trạng thái kết nối ("Dang kết nối...", "Offline", "Dang nghe...")
6. Khong co `catch` rong hoặc `_ = err` trong code

## Error Response Helper

Tất cả handlers dung helper function từ `pkg/response/json.go`:

```go
// Thanh cong
response.JSON(w, http.StatusOK, data)

// Loi
response.Error(w, http.StatusBadRequest, "MISSING_FIELD", "Field 'name' is required")

// Loi noi bo (log chi tiet, tra message chung)
response.InternalError(w, err)
// → Log: slog.Error("internal error", "error", err.Error(), ...)
// → Response: {"error": {"code": "INTERNAL_ERROR", "message": "An unexpected error occurred"}}
```
