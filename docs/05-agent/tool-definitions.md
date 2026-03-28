# Tool Definitions

## Tổng quan

AiSee Agent co 20 tools (+ 1 provider tool GoogleSearch). Tools chia thanh 2 loai theo cach xử lý:

| Loai | Handler | Ví dụ |
|------|---------|-------|
| **Backend tools** | Agent gửi HTTP POST toi Go Backend `/api/internal/tool-call` | make_voice_call, send_chat_message, set_medication_reminder |
| **Data channel tools** | Agent gửi JSON qua LiveKit data channel toi App | activate_camera, control_music, navigate_to |

## 1. make_voice_call

Goi điện thoại in-app cho người thân.

| | |
|---|---|
| **Params** | `contact_name: str` (required) — Ten hoặc quan he: "con gai", "Lan", "chong" |
| **Handler** | Agent -> Go Backend HTTP |
| **Backend endpoint** | `POST /api/internal/tool-call` |

**Gemini function call:**

```json
{
  "name": "make_voice_call",
  "args": {
    "contact_name": "con gai"
  }
}
```

**Response:**

```json
{
  "status": "room_created",
  "room_name": "call-550e8400",
  "callee_name": "Lan"
}
```

## 2. make_video_call

Goi video call in-app cho người thân.

| | |
|---|---|
| **Params** | `contact_name: str` (required) |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "make_video_call",
  "args": {
    "contact_name": "con gai"
  }
}
```

**Response:**

```json
{
  "status": "room_created",
  "room_name": "videocall-550e8400",
  "callee_name": "Lan"
}
```

## 3. send_chat_message

Gửi tin nhắn text cho người thân.

| | |
|---|---|
| **Params** | `contact_name: str` (required), `message: str` (required) |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "send_chat_message",
  "args": {
    "contact_name": "con gai",
    "message": "Con oi, mama dang o nha. Khong can lo."
  }
}
```

**Response:**

```json
{
  "status": "sent",
  "message_id": "uuid",
  "receiver_name": "Lan"
}
```

## 4. read_unread_messages

Đọc tin nhắn chưa đọc cho user nghe.

| | |
|---|---|
| **Params** | `contact_name: str` (optional) — Nếu co, chi doc tin tu nguoi này. Nếu không, doc tất cả. |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "read_unread_messages",
  "args": {
    "contact_name": "con gai"
  }
}
```

**Response:**

```json
{
  "messages": [
    {
      "sender_name": "Lan",
      "content": "Mama uong thuoc chua a?",
      "sent_at": "2026-03-28T08:30:00+07:00"
    },
    {
      "sender_name": "Lan",
      "content": "Con se ghe tham chieu nay",
      "sent_at": "2026-03-28T09:15:00+07:00"
    }
  ],
  "count": 2
}
```

Gemini nhan kết quả và doc to cho user: "Ban co 2 tin nhắn tu Lan. Tin thu nhat: Mama uống thuốc chưa a? Tin thu hai: Con se ghe tham chieu này."

## 5. emergency_alert

Gửi SOS toi tất cả người thân. Khong can xác nhận nếu user dang nguy hiểm.

| | |
|---|---|
| **Params** | Khong co |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "emergency_alert",
  "args": {}
}
```

**Response:**

```json
{
  "status": "alerted",
  "alerted_count": 3,
  "room_name": "emergency-uuid"
}
```

## 6. escalate_to_family

Goi mot người thân cu the khi AI không thể giup được.

| | |
|---|---|
| **Params** | `contact_name: str` (optional) — Nếu không chi dinh, goi primary contact |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "escalate_to_family",
  "args": {
    "contact_name": "con gai"
  }
}
```

**Response:**

```json
{
  "status": "calling",
  "callee_name": "Lan",
  "room_name": "escalation-uuid"
}
```

## 7. escalate_to_volunteer

Ket noi với tình nguyện viên khi user can ho tro ma family không có mat.

| | |
|---|---|
| **Params** | `reason: str` (optional) — Lý do can ho tro |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "escalate_to_volunteer",
  "args": {
    "reason": "Can giup doc don thuoc"
  }
}
```

**Response:**

```json
{
  "status": "searching",
  "match_id": "uuid",
  "room_id": "uuid"
}
```

Agent se poll trạng thái matching và thông báo user.

## 8. set_medication_reminder

Đặt lich nhac uống thuốc.

| | |
|---|---|
| **Params** | `medicine_name: str` (required), `dosage: str` (required), `times: list[str]` (required), `frequency: str` (required — "daily", "weekly", "as_needed") |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "set_medication_reminder",
  "args": {
    "medicine_name": "Amlodipine",
    "dosage": "5mg",
    "times": ["08:00", "20:00"],
    "frequency": "daily"
  }
}
```

**Response:**

```json
{
  "status": "created",
  "medication_id": "uuid",
  "next_reminder": "2026-03-28T20:00:00+07:00"
}
```

## 9. list_medication_reminders

Lấy danh sách thuoc hom này.

| | |
|---|---|
| **Params** | Khong co |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "list_medication_reminders",
  "args": {}
}
```

**Response:**

```json
{
  "medications": [
    {
      "name": "Amlodipine",
      "dosage": "5mg",
      "times": ["08:00", "20:00"],
      "taken_today": ["08:00"],
      "missed_today": []
    },
    {
      "name": "Metformin",
      "dosage": "500mg",
      "times": ["07:00", "12:00", "19:00"],
      "taken_today": ["07:00", "12:00"],
      "missed_today": []
    }
  ]
}
```

Gemini đọc: "Hom này ban co 2 loai thuoc. Amlodipine 5mg, da uong luc 8 gio sang, con 1 lan luc 8 gio toi..."

## 10. mark_medication_taken

Danh dau da uống thuốc.

| | |
|---|---|
| **Params** | `medicine_name: str` (required) |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "mark_medication_taken",
  "args": {
    "medicine_name": "Amlodipine"
  }
}
```

**Response:**

```json
{
  "status": "logged",
  "medication_name": "Amlodipine",
  "taken_at": "2026-03-28T20:05:00+07:00"
}
```

## 11. set_appointment_reminder

Đặt lịch hẹn kham bac si.

| | |
|---|---|
| **Params** | `doctor: str` (required), `datetime: str` (required — ISO 8601), `notes: str` (optional) |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "set_appointment_reminder",
  "args": {
    "doctor": "Bac si Nguyen Van A",
    "datetime": "2026-04-15T09:00:00+07:00",
    "notes": "Mang theo so kham benh"
  }
}
```

**Response:**

```json
{
  "status": "created",
  "appointment_id": "uuid"
}
```

## 12. log_health_symptom

Ghi lai triệu chứng sức khỏe.

| | |
|---|---|
| **Params** | `symptom: str` (required), `severity: str` (optional — "nhe", "vua", "nang"), `notes: str` (optional) |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "log_health_symptom",
  "args": {
    "symptom": "dau dau",
    "severity": "vua",
    "notes": "sau bua trua"
  }
}
```

**Response:**

```json
{
  "status": "logged",
  "symptom_id": "uuid"
}
```

## 13. log_health_metric

Ghi lai chỉ số sức khỏe.

| | |
|---|---|
| **Params** | `metric_type: str` (required — "blood_pressure", "blood_sugar", "weight", "temperature"), `value: str` (required), `unit: str` (optional) |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "log_health_metric",
  "args": {
    "metric_type": "blood_pressure",
    "value": "130/85",
    "unit": "mmHg"
  }
}
```

**Response:**

```json
{
  "status": "logged",
  "metric_id": "uuid"
}
```

## 14. activate_camera

Bật camera de Gemini nhin qua điện thoại của user.

| | |
|---|---|
| **Params** | `reason: str` (optional) — Lý do bật camera |
| **Handler** | Agent -> App data channel |

**Gemini function call:**

```json
{
  "name": "activate_camera",
  "args": {
    "reason": "doc nhan thuoc"
  }
}
```

**Data channel message gửi toi App:**

```json
{
  "type": "tool_action",
  "request_id": "uuid",
  "action": "enable_camera",
  "params": {}
}
```

**App ACK:**

```json
{
  "type": "tool_action_ack",
  "request_id": "uuid",
  "status": "ok"
}
```

**Response trả về Gemini:**

```json
{
  "status": "camera_enabled"
}
```

## 15. deactivate_camera

Tat camera, tro ve audio-only.

| | |
|---|---|
| **Params** | Khong co |
| **Handler** | Agent -> App data channel |

**Gemini function call:**

```json
{
  "name": "deactivate_camera",
  "args": {}
}
```

**Data channel message:** `{ "action": "disable_camera" }`

**Response:** `{ "status": "camera_disabled" }`

## 16. navigate_to

Mở ứng dụng bản đồ Mapbox với điểm đến đã điền sẵn.

| | |
|---|---|
| **Params** | `destination: str` (required) — Dia chi hoặc ten dia diem |
| **Handler** | Agent -> App data channel |

**Gemini function call:**

```json
{
  "name": "navigate_to",
  "args": {
    "destination": "Benh vien Mat trung uong, Ha Noi"
  }
}
```

**Data channel message:**

```json
{
  "type": "tool_action",
  "request_id": "uuid",
  "action": "open_maps",
  "params": {
    "destination": "Benh vien Mat trung uong, Ha Noi"
  }
}
```

**Response:** `{ "status": "navigation_started" }`

## 17. search_information

Tim kiem thông tin tren internet.

| | |
|---|---|
| **Params** | `query: str` (required) |
| **Handler** | Gemini + GoogleSearch provider tool |

Đây la provider tool — Gemini tự động sử dụng GoogleSearch de tìm kiếm. Agent không cần route request này.

**Gemini function call:**

```json
{
  "name": "search_information",
  "args": {
    "query": "thoi tiet Ha Noi hom nay"
  }
}
```

Gemini nhan kết quả search và tom tắt cho user.

## 18. play_music

Tim và phát nhạc/radio/podcast.

| | |
|---|---|
| **Params** | `query: str` (required), `source: str` (optional — "youtube", "radio", "podcast") |
| **Handler** | Agent -> Go Backend HTTP (search) -> Agent -> App data channel (play) |

**Gemini function call:**

```json
{
  "name": "play_music",
  "args": {
    "query": "nhac Trinh Cong Son",
    "source": "youtube"
  }
}
```

**Backend response (search result):**

```json
{
  "track": {
    "title": "Diem Xua - Khanh Ly",
    "url": "https://...",
    "duration": "4:32"
  }
}
```

**Data channel message gửi toi App:**

```json
{
  "type": "tool_action",
  "request_id": "uuid",
  "action": "play_audio",
  "params": {
    "url": "https://...",
    "title": "Diem Xua - Khanh Ly"
  }
}
```

## 19. control_music

Dieu khien phát nhạc.

| | |
|---|---|
| **Params** | `action: str` (required — "pause", "resume", "next", "previous", "volume_up", "volume_down", "stop") |
| **Handler** | Agent -> App data channel |

**Gemini function call:**

```json
{
  "name": "control_music",
  "args": {
    "action": "pause"
  }
}
```

**Data channel message:**

```json
{
  "type": "tool_action",
  "request_id": "uuid",
  "action": "control_audio",
  "params": {
    "command": "pause"
  }
}
```

**Response:** `{ "status": "paused" }`

## 20. add_to_queue

Thêm bai hat vao hang doi phat.

| | |
|---|---|
| **Params** | `query: str` (required) |
| **Handler** | Agent -> Go Backend HTTP |

**Gemini function call:**

```json
{
  "name": "add_to_queue",
  "args": {
    "query": "Noi Vong Tay Lon"
  }
}
```

**Response:**

```json
{
  "status": "added",
  "track": {
    "title": "Noi Vong Tay Lon - Trinh Cong Son",
    "url": "https://...",
    "duration": "3:45"
  },
  "queue_position": 3
}
```

## 21. GoogleSearch (Provider Tool)

Khong phai agent tool ma la provider tool — Gemini tu quyet dinh khi nào goi.

| | |
|---|---|
| **Cấu hình** | `session.provider_tools = [google.tools.GoogleSearch()]` |
| **Khi nào dùng** | Gemini can thông tin cập nhật (tin tuc, thoi tiet, dia diem) |
| **Agent co can lam gi không** | Khong — Gemini tu xử lý |

## Bang tom tắt

| # | Tool | Handler | Can xác nhận user? |
|---|------|---------|-------------------|
| 1 | make_voice_call | Backend HTTP | Co |
| 2 | make_video_call | Backend HTTP | Co |
| 3 | send_chat_message | Backend HTTP | Co |
| 4 | read_unread_messages | Backend HTTP | Khong |
| 5 | emergency_alert | Backend HTTP | Co (tru khi nguy hiểm) |
| 6 | escalate_to_family | Backend HTTP | Co |
| 7 | escalate_to_volunteer | Backend HTTP | Co |
| 8 | set_medication_reminder | Backend HTTP | Co |
| 9 | list_medication_reminders | Backend HTTP | Khong |
| 10 | mark_medication_taken | Backend HTTP | Khong |
| 11 | set_appointment_reminder | Backend HTTP | Co |
| 12 | log_health_symptom | Backend HTTP | Khong |
| 13 | log_health_metric | Backend HTTP | Khong |
| 14 | activate_camera | App data channel | Co (tru khi ro rang can) |
| 15 | deactivate_camera | App data channel | Khong |
| 16 | navigate_to | App data channel | Co |
| 17 | search_information | Gemini GoogleSearch | Khong |
| 18 | play_music | Backend HTTP + data channel | Khong |
| 19 | control_music | App data channel | Khong |
| 20 | add_to_queue | Backend HTTP | Khong |
| 21 | GoogleSearch (provider) | Gemini tự động | Khong |
