# Push Notifications

## Tổng quan

AiSee gửi push notifications truc tiep tu Go Backend, không dùng Firebase SDK. Co 3 loai push:

| Loai | Platform | Thu vien | Mục đích |
|------|----------|----------|----------|
| iOS APNs Standard | iOS | `sideshow/apns2` | Tin nhận, nhắc thuốc, alerts |
| iOS VoIP Push (PushKit) | iOS | `sideshow/apns2` | Incoming calls — danh thuc app |
| Android FCM HTTP v1 | Android | `net/http` (Go stdlib) | Tất cả notifications + incoming calls |

## iOS APNs Standard Push

### Khi nào dùng

- Tin nhan mỗi tu family member
- Nhac uống thuốc
- Emergency alert thông báo
- Thong bao tình nguyện viên da kết nối

### Cấu hình

Can file AuthKey `.p8` tu Apple Developer portal:

```env
APNS_KEY_PATH=./certs/AuthKey_XXXXXXXXXX.p8
APNS_KEY_ID=XXXXXXXXXX
APNS_TEAM_ID=YYYYYYYYYY
APNS_BUNDLE_ID=com.dang.aisee
```

### Payload mau

```json
{
  "aps": {
    "alert": {
      "title": "Lan",
      "body": "Mama uong thuoc chua a?"
    },
    "sound": "default",
    "badge": 3
  },
  "data": {
    "type": "message",
    "sender_id": "uuid"
  }
}
```

### Cach gửi (Go)

```go
// pkg/push/apns.go
notification := &apns2.Notification{
    DeviceToken: user.APNSToken,
    Topic:       bundleID,
    Payload:     payload,
    Priority:    apns2.PriorityHigh,
}
resp, err := apnsClient.Push(notification)
```

## iOS VoIP Push (PushKit)

### Khi nào dùng

**Chi dùng cho incoming calls.** VoIP Push la cach duy nhat de hiển thị native call UI tren iOS khi app bi kill.

Apple yêu cầu: mỗi VoIP push PHAI hiển thị `CallKit` incoming call screen. Nếu không, app se bi ban khoi App Store.

### Cach hoạt động

```
Go Backend                         iOS Device
    │                                  │
    │  VoIP Push (PushKit)             │
    │─────────────────────────────────>│
    │                                  │
    │                   PushKit danh thuc app
    │                   AppDelegate nhan push
    │                   Hien thi native call screen
    │                   (flutter_callkit_incoming)
    │                                  │
    │                   User tra loi → join LiveKit room
```

### Payload mau

```json
{
  "aps": {},
  "caller_name": "Lan",
  "caller_id": "uuid",
  "room_name": "call-uuid",
  "call_type": "voice"
}
```

VoIP Push dung `apns_voip_token` (khac với `apns_token` thong thuong). Topic là `<bundle_id>.voip`:

```go
notification := &apns2.Notification{
    DeviceToken: user.APNSVoIPToken,
    Topic:       bundleID + ".voip",
    Payload:     payload,
    Priority:    apns2.PriorityHigh,
    PushType:    apns2.PushTypeVoIP,
}
```

### Quan trong

- VoIP Push chi dùng cho calls. Nếu dùng cho mục dich khac, Apple se reject app.
- User PHAI đăng ký `PushKit` rieng de lấy `apns_voip_token`.
- App can enable "Voice over IP" background mode trong Xcode.

## Android FCM HTTP v1 API

### Khi nào dùng

Tất cả notifications tren Android:
- Tin nhan mỗi
- Nhac thuoc
- Emergency alerts
- Incoming calls (FCM high-priority thay cho VoIP Push)

### Cấu hình

Can file service account JSON tu Google Cloud Console:

```env
FCM_SERVICE_ACCOUNT_PATH=./certs/fcm-service-account.json
```

### Incoming call tren Android

Android không có VoIP Push tuong duong iOS. Thay vao đó, dùng FCM **high-priority data message**:

```json
{
  "message": {
    "token": "<fcm_token>",
    "android": {
      "priority": "HIGH"
    },
    "data": {
      "type": "incoming_call",
      "caller_name": "Lan",
      "caller_id": "uuid",
      "room_name": "call-uuid",
      "call_type": "voice"
    }
  }
}
```

App nhan FCM message → `flutter_callkit_incoming` hiển thị native incoming call screen.

### Normal notification mau

```json
{
  "message": {
    "token": "<fcm_token>",
    "notification": {
      "title": "Lan",
      "body": "Mama uong thuoc chua a?"
    },
    "data": {
      "type": "message",
      "sender_id": "uuid"
    }
  }
}
```

### Cach gửi (Go)

```go
// pkg/push/fcm.go
// Su dung FCM HTTP v1 API voi OAuth2 service account

// 1. Lay access token tu service account
token, err := getAccessToken(serviceAccountPath)

// 2. Gui request
url := "https://fcm.googleapis.com/v1/projects/<project-id>/messages:send"
req, _ := http.NewRequest("POST", url, body)
req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("Content-Type", "application/json")
resp, err := http.DefaultClient.Do(req)
```

## Bang so sanh các loai push

| | iOS APNs | iOS VoIP Push | Android FCM |
|---|---|---|---|
| **Mục đích** | Notifications chung | Incoming calls ONLY | Tất cả notifications + calls |
| **Token field** | `apns_token` | `apns_voip_token` | `fcm_token` |
| **Danh thuc app** | Co (limited) | Co (luon luon) | Co (high priority) |
| **Native call UI** | Khong | Co (CallKit) | Co (flutter_callkit_incoming) |
| **Thu vien** | sideshow/apns2 | sideshow/apns2 | net/http (FCM v1 API) |
| **Yêu cầu đặc biệt** | AuthKey .p8 | Background mode VoIP | Service account JSON |

## Token Management (Device Registration)

### Flow đăng ký token

```
App khoi dong
    │
    ├─ iOS: Dang ky APNs → nhan apns_token
    │       Dang ky PushKit → nhan apns_voip_token
    │
    ├─ Android: Dang ky FCM → nhan fcm_token
    │
    └─ Gui tat ca tokens len backend:
       PUT /api/v1/users/me
       {
         "fcm_token": "...",
         "apns_token": "...",
         "apns_voip_token": "..."
       }
```

### Cap nhat token

Tokens có thể thay doi khi:
- App được cài đặt lai
- User restore tu backup
- Token hết hạn (hiem)

App PHAI gửi token mỗi len backend moi khi:
1. App khởi động (`didFinishLaunchingWithOptions` / `onCreate`)
2. Token thay doi (callback tu APNs/FCM)

### Xử lý token hết hạn

Khi gửi push ma nhan lỗi token invalid:
1. Backend xóa token cu khoi database
2. Log warning (không retry với token cu)
3. Lan tiep theo app khởi động se đăng ký token mỗi

## Error Handling

### Retry strategy

Khi gửi push thất bại:

```
Lan 1: Gui ngay
Lan 2: Sau 1 giay
Lan 3: Sau 3 giay
Sau 3 lan: Log warning, bo qua
```

### Các lỗi thuong gap

| Loi | Xử lý |
|-----|-------|
| Token invalid / unregistered | Xóa token khoi database |
| Rate limited (429) | Retry với exponential backoff |
| Service unavailable (503) | Retry |
| Payload too large | Log error, kiểm tra payload size |
| Authentication error | Kiem tra certificates / service account |
