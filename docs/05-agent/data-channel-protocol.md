# Data Channel Protocol

## Tổng quan

Agent Worker và Flutter App giao tiep qua LiveKit data channel cho các hành động yêu cầu App thực hiện tren device (bật camera, phát nhạc, mo bản đồ). Protocol sử dụng JSON messages.

## JSON Schema

### Agent -> App (Tool Action Request)

```json
{
  "type": "tool_action",
  "request_id": "string (UUID v4)",
  "action": "string (ten action)",
  "params": { }
}
```

| Field | Type | Mo ta |
|-------|------|-------|
| `type` | string | Luon là `"tool_action"` |
| `request_id` | string | UUID v4 duy nhat cho mỗi request. Dùng de match ACK. |
| `action` | string | Ten action (xem bang ben duoi) |
| `params` | object | Tham số của action. Co the rong `{}`. |

### App -> Agent (Acknowledgment)

```json
{
  "type": "tool_action_ack",
  "request_id": "string (UUID v4)",
  "status": "ok | error",
  "error_message": "string (chi khi status = error)"
}
```

| Field | Type | Mo ta |
|-------|------|-------|
| `type` | string | Luon là `"tool_action_ack"` |
| `request_id` | string | Giong `request_id` trong request |
| `status` | string | `"ok"` nếu thành công, `"error"` nếu thất bại |
| `error_message` | string? | Mo ta lỗi. Chi co khi `status = "error"`. |

## Bang Actions

| Action | Params | App thực hiện |
|--------|--------|--------------|
| `enable_camera` | `{}` | Bật camera, publish video track len LiveKit |
| `disable_camera` | `{}` | Tat camera, unpublish video track |
| `open_maps` | `{ "destination": "string" }` | Mo Mapbox navigation toi dia diem |
| `play_audio` | `{ "url": "string", "title": "string" }` | Phat audio tu URL qua `just_audio` |
| `control_audio` | `{ "command": "string" }` | Dieu khien audio player |

### Chi tiết tung action

#### enable_camera

```json
{
  "type": "tool_action",
  "request_id": "a1b2c3d4-...",
  "action": "enable_camera",
  "params": {}
}
```

App sẽ:
1. Kiem tra camera permission
2. Nếu chưa co permission: tra ACK error
3. Bật camera (rear camera mặc định)
4. Publish video track len LiveKit room
5. Tra ACK `"ok"`

#### disable_camera

```json
{
  "type": "tool_action",
  "request_id": "e5f6g7h8-...",
  "action": "disable_camera",
  "params": {}
}
```

App sẽ:
1. Unpublish video track
2. Tat camera
3. Tra ACK `"ok"`

#### open_maps

```json
{
  "type": "tool_action",
  "request_id": "i9j0k1l2-...",
  "action": "open_maps",
  "params": {
    "destination": "Benh vien Mat trung uong, Ha Noi"
  }
}
```

App sẽ:
1. Mở ứng dụng bản đồ Mapbox với điểm đến
2. Geocode destination string thành tọa độ
3. Hiển thị bản đồ với điểm đến đã điền sẵn
4. Trả ACK `"ok"`

#### play_audio

```json
{
  "type": "tool_action",
  "request_id": "m3n4o5p6-...",
  "action": "play_audio",
  "params": {
    "url": "https://stream.vov.vn/vov1/vov1.m3u8",
    "title": "VOV1 - Dai Tieng noi Viet Nam"
  }
}
```

App sẽ:
1. Load URL vao `just_audio` player
2. Bật dau phat
3. Hien thi mini player (nếu co UI)
4. Tra ACK `"ok"`

#### control_audio

```json
{
  "type": "tool_action",
  "request_id": "q7r8s9t0-...",
  "action": "control_audio",
  "params": {
    "command": "pause"
  }
}
```

Các `command` ho tro:

| Command | Mo ta |
|---------|-------|
| `pause` | Tam dùng phat |
| `resume` | Tiep tuc phat |
| `next` | Chuyen bai tiep theo trong queue |
| `previous` | Quay lai bai truoc |
| `volume_up` | Tang am luong |
| `volume_down` | Giam am luong |
| `stop` | Dùng phat, xóa queue |

## Error Handling

### Timeout và Retry

```
Agent gui tool_action
    │
    ├─ App ACK trong 5 giay → thanh cong
    │
    └─ Khong co ACK sau 5 giay
        │
        ├─ Retry 1 lan (gui lai cung request_id)
        │
        ├─ App ACK → thanh cong
        │
        └─ Van khong co ACK sau 5 giay
            │
            └─ Agent bao user:
               "Toi khong the thuc hien duoc.
                Ban thu [huong dan thu cong] nhe."
```

### Error cases cu the

| Error | ACK message | Agent noi với user |
|-------|-------------|-------------------|
| Camera permission denied | `{ "status": "error", "error_message": "Camera permission denied" }` | "Ban can cho phep camera trong cài đặt." |
| Camera dang được dùng boi app khac | `{ "status": "error", "error_message": "Camera in use" }` | "Camera dang được dùng boi ứng dụng khac." |
| Audio URL invalid | `{ "status": "error", "error_message": "Invalid audio URL" }` | "Khong phat được bai này. De toi tim bai khac." |
| Audio playback error | `{ "status": "error", "error_message": "Playback failed" }` | "Khong phat được. Ban muon thu nguon khac không?" |
| Maps geocoding failed | `{ "status": "error", "error_message": "Cannot find destination" }` | "Toi không tim thay địa chỉ này. Ban có thể noi ro hon không?" |
| No audio playing | `{ "status": "error", "error_message": "No audio playing" }` | "Hien tại không có nhac dang phat." |

### Xử lý khi App mat kết nối

Nếu App disconnect khoi LiveKit room:
1. Data channel không con hoạt động
2. Agent phát hiện disconnect event
3. Agent doi reconnect (LiveKit tự động reconnect)
4. Nếu reconnect thành công trong 10 giây: tiep tuc bình thường
5. Nếu reconnect thất bại: agent thông báo và ket thuc session

## Ví dụ hoan chinh: Bật camera và doc nhan thuoc

```
Thoi gian  Huong      Message
──────────────────────────────────────────────────────

t=0        User       "Doc giup toi cai nhan thuoc nay"

t=0.5      Gemini     "Ban muon toi xem qua camera khong?"

t=2        User       "Uh, co"

t=2.5      Gemini     [goi tool: activate_camera]

t=2.5      Agent→App  {
                        "type": "tool_action",
                        "request_id": "abc-123",
                        "action": "enable_camera",
                        "params": {}
                      }

t=3        App→Agent  {
                        "type": "tool_action_ack",
                        "request_id": "abc-123",
                        "status": "ok"
                      }

t=3        Agent      [tra ve Gemini: camera_enabled]

t=3-5      App        [publish video frames, 1 FPS]

t=5        Gemini     "Day la thuoc Amlodipine 5mg.
                       Uong 1 vien moi ngay..."

t=7        Gemini     [goi tool: deactivate_camera]

t=7        Agent→App  {
                        "type": "tool_action",
                        "request_id": "def-456",
                        "action": "disable_camera",
                        "params": {}
                      }

t=7.2      App→Agent  {
                        "type": "tool_action_ack",
                        "request_id": "def-456",
                        "status": "ok"
                      }

t=7.2      Gemini     "Ban con can gi nua khong?"
```
