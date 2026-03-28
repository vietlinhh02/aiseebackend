# Luồng dữ liệu (Data Flow)

Tài liệu này mô tả luồng dữ liệu của các tính năng chính trong AiSee. Mỗi luồng
được trình bày theo trình tự thời gian, từ hành động của người dùng đến kết quả
cuối cùng.

## 1. Hội thoại giọng nói (Voice Conversation)

Đây là luồng cơ bản nhất -- người dùng nói, AI trả lời.

```
User                  Flutter App         LiveKit Server      Agent Worker        Gemini Live API
  |                       |                    |                   |                    |
  |  Noi: "May gio roi?"  |                    |                   |                    |
  +---------------------->|                    |                   |                    |
  |                       | Audio track ------>|                   |                    |
  |                       |    (WebRTC)        | Forward audio --->|                    |
  |                       |                    |                   | Audio stream ----->|
  |                       |                    |                   |                    |
  |                       |                    |                   |    (Gemini xu ly)  |
  |                       |                    |                   |                    |
  |                       |                    |                   | Audio response <---|
  |                       |                    | <--- Audio track  |                    |
  |                       | <--- Audio (WebRTC)|                   |                    |
  | Nghe: "Bay gio la..." |                    |                   |                    |
  |<----------------------|                    |                   |                    |
```

**Đặc điểm:**
- Audio-only mặc định, session tối đa 15 phút
- Gemini Live là streaming 2 chiều (full-duplex) -- user có thể ngắt giữa câu
- Không có bước trung gian STT/TTS -- Gemini nhận/trả audio trực tiếp
- Delay thực tế: ~300-800ms (tương đương gọi điện bình thường)

## 2. Camera theo yêu cầu (On-Demand Vision)

Khi user cần AI "nhìn" (đọc nhãn thuốc, mô tả vật thể, ...).

```
User             Flutter App        LiveKit          Agent Worker      Gemini Live
  |                  |                 |                  |                |
  | "Doc nhan thuoc" |                 |                  |                |
  +----------------->|  audio -------->|  forward ------->|  audio ------->|
  |                  |                 |                  |                |
  |                  |                 |                  |   Gemini quyet |
  |                  |                 |                  |   dinh can     |
  |                  |                 |                  |   camera       |
  |                  |                 |                  |                |
  | Nghe: "Ban muon  |                 |                  | <--- audio --- |
  | toi xem qua      |                 |  <--- audio ---- |                |
  | camera khong?"   | <--- audio ---- |                  |                |
  |<-----------------|                 |                  |                |
  |                  |                 |                  |                |
  | "Co"             |                 |                  |                |
  +----------------->| audio --------->| forward -------->|  audio ------->|
  |                  |                 |                  |                |
  |                  |                 |                  |  Gemini goi    |
  |                  |                 |                  |  activate_     |
  |                  |                 |                  |  camera tool   |
  |                  |                 |                  |                |
  |                  | <-- data channel: "enable_camera"  |                |
  |                  |                 |                  |                |
  |       [Camera ON]|                 |                  |                |
  |                  | Video track --->| forward -------->|  video ------->|
  |                  |   (1 FPS)       |                  |                |
  |                  |                 |                  |                |
  |                  |                 |                  |   Gemini phan  |
  |                  |                 |                  |   tich hinh    |
  |                  |                 |                  |                |
  | Nghe: "Day la    |                 |                  | <-- audio ---- |
  | thuoc Amlodipine | <--- audio ---- | <--- audio ----  |                |
  | 5mg..."          |                 |                  |                |
  |<-----------------|                 |                  |                |
  |                  |                 |                  |  Gemini goi    |
  |                  |                 |                  |  deactivate_   |
  |                  |                 |                  |  camera tool   |
  |                  |                 |                  |                |
  |                  | <-- data channel: "disable_camera" |                |
  |      [Camera OFF]|                 |                  |                |
  |                  | Unpublish video |                  |                |
```

**Lưu ý quan trọng:**
- Video chỉ gửi 1 frame/giây (1 FPS) để tiết kiệm bandwidth và token
- Khi video bật, session limit giảm từ 15 phút xuống 2 phút
- Agent Worker theo dõi thời gian và tắt camera sớm nếu cần
- User cũng có thể bấm nút camera trên màn hình để bật/tắt thủ công

## 3. Gọi người thân (Human Call)

Khi user muốn gọi điện cho người thân qua app.

```
User             App          Agent        Go Backend       LiveKit       Family App
  |               |             |              |               |              |
  | "Goi con gai" |             |              |               |              |
  +-------------->| audio ----->|              |               |              |
  |               |             |              |               |              |
  | Nghe: "Toi se |             |              |               |              |
  | goi cho Lan   | <-- audio --|              |               |              |
  | nhe?"         |             |              |               |              |
  |<--------------|             |              |               |              |
  |               |             |              |               |              |
  | "U"           |             |              |               |              |
  +-------------->| audio ----->|              |               |              |
  |               |             |              |               |              |
  |               |             | Goi tool:    |               |              |
  |               |             | make_voice_  |               |              |
  |               |             | call         |               |              |
  |               |             |              |               |              |
  |               |             | HTTP POST -->|               |              |
  |               |             | /tool-call   |               |              |
  |               |             |              |               |              |
  |               |             |              | Create room ->|              |
  |               |             |              |               |              |
  |               |             |              | Push notif ---|------------->|
  |               |             |              | (APNs / FCM)  |              |
  |               |             |              |               |              |
  |               |             |              |               |     [RING!]  |
  |               |             |              |               |  Native call |
  |               |             |              |               |  UI hien len |
  |               |             |              |               |              |
  |               |             |              |               |  Lan tra loi |
  |               |             |              |               |<-------------|
  |               |             |              |               |  Lan join    |
  |               |             |              |               |  room        |
  |               |             |              |               |              |
  |               |       [Agent im tieng      |               |              |
  |               |        hoac roi room]      |               |              |
  |               |             |              |               |              |
  | User va Lan noi chuyen truc tiep qua LiveKit room         |              |
  |<==========================================================>|<============>|
  |               |             |              |               |              |
  |               |             |              |               |  Lan cup may |
  |               |             |              |               |<-------------|
  |               |             |              |               |  Roi room    |
  |               |             |              |               |              |
  |               |       [Agent bat lai]      |               |              |
  |               |             |              |               |              |
  | Nghe: "Cuoc   |             |              |               |              |
  | goi voi Lan   | <-- audio --|              |               |              |
  | da ket thuc"  |             |              |               |              |
  |<--------------|             |              |               |              |
```

**Đặc điểm:**
- User và người thân nói chuyện trong **cùng LiveKit room** -- không cần kết nối mới
- Agent Worker im tiếng (mute) hoặc rời room trong lúc gọi điện
- Sau khi người thân cúp, Agent quay lại hội thoại
- Người thân nhận push notification và thấy màn hình gọi đến native (như cuộc gọi thường)

## 4. Gọi đến từ người thân (Incoming Call)

Khi người thân gọi cho user.

```
Family App        Go Backend          LiveKit           Flutter App        User
    |                 |                  |                  |                |
    | POST             |                  |                  |                |
    | /calls/          |                  |                  |                |
    | create-room      |                  |                  |                |
    +---------------->|                  |                  |                |
    |                 | Create room ----->|                  |                |
    |                 |                  |                  |                |
    |                 | Push notification:                   |                |
    |                 |   iOS: VoIP Push (PushKit) --------->|                |
    |                 |   Android: FCM high-priority ------->|                |
    |                 |                  |                  |                |
    |                 |                  |                  |   [RING!]      |
    |                 |                  |                  |   Man hinh     |
    |                 |                  |                  |   goi den      |
    |                 |                  |                  |   native       |
    |                 |                  |                  |                |
    |                 |                  |                  |   User tra loi |
    |                 |                  |                  |<---------------|
    |                 |                  | <-- User join ---|                |
    | Join room ----->|                  |                  |                |
    |                 |                  |                  |                |
    | User va nguoi than noi chuyen trong LiveKit room      |                |
    |<=====================================================>|<==============>|
```

**Lưu ý:**
- iOS dùng VoIP Push (PushKit) để hiện màn hình gọi đến ngay cả khi app bị tắt
- Android dùng FCM high-priority để đánh thức app
- Trong lúc gọi, Gemini agent vẫn có thể hỗ trợ nếu user hỏi

## 5. Chat message

Khi user gửi tin nhắn (qua AI) hoặc người thân gửi tin nhắn.

### User gửi tin nhắn (qua AI)

```
User             Agent           Go Backend         WebSocket Hub      Family App
  |                |                  |                  |                |
  | "Nhan cho      |                  |                  |                |
  |  con gai:      |                  |                  |                |
  |  me khoe"      |                  |                  |                |
  +--------------->|                  |                  |                |
  |                |                  |                  |                |
  | Nghe: "Toi se  |                  |                  |                |
  | gui 'me khoe'  |                  |                  |                |
  | cho Lan nhe?"  |                  |                  |                |
  |<---------------|                  |                  |                |
  |                |                  |                  |                |
  | "U"            |                  |                  |                |
  +--------------->|                  |                  |                |
  |                | HTTP POST ------>|                  |                |
  |                | /tool-call       | Save to DB       |                |
  |                | (send_chat_      |                  |                |
  |                |  message)        | Push notif ------|--------------->|
  |                |                  |                  |                |
  |                |                  | WS message ----->| Forward ------>|
  |                |                  |                  |  (real-time)   |
  |                |                  |                  |                |
  |                | <-- { ok } ------|                  |                |
  |                |                  |                  |                |
  | Nghe: "Da gui" |                  |                  |                |
  |<---------------|                  |                  |                |
```

### Người thân gửi tin nhắn

```
Family App       Go Backend        WebSocket Hub       Agent           User
    |                |                  |                 |               |
    | POST           |                  |                 |               |
    | /messages/send |                  |                 |               |
    +--------------->| Save to DB       |                 |               |
    |                |                  |                 |               |
    |                | Push notif ------|---------------->| App nhận     |
    |                |                  |                 |               |
    |                | WS message ----->| Forward ------->| Hiển thị    |
    |                |                  |                 | notification |
    |                |                  |                 |               |
    |    (Nếu user đang nói chuyện với Gemini,            |               |
    |     Gemini đọc tin nhắn cho user nghe)               |               |
```

## 6. Emergency Alert (SOS)

Khi user gặp nguy hiểm hoặc cần giúp khẩn cấp.

```
User             Agent          Go Backend         Push Service       Family Members
  |                |                |                   |                  |
  | "Cuu toi!" /   |                |                   |                  |
  | Bam nut SOS    |                |                   |                  |
  +--------------->|                |                   |                  |
  |                |                |                   |                  |
  |                | (Truong hop    |                   |                  |
  |                |  nguy hiem:    |                   |                  |
  |                |  KHONG hoi     |                   |                  |
  |                |  xac nhan)     |                   |                  |
  |                |                |                   |                  |
  |                | HTTP POST ---->|                   |                  |
  |                | emergency_     |                   |                  |
  |                | alert          |                   |                  |
  |                |                |                   |                  |
  |                |                | Lay GPS user -----|                  |
  |                |                |                   |                  |
  |                |                | Push BROADCAST -->| APNs (iOS) ---->|
  |                |                | den TAT CA       | FCM (Android) ->|
  |                |                | nguoi than        |                  |
  |                |                |                   |                  |
  |                |                | Push noi dung:                      |
  |                |                | "[User] can giup khan cap!"         |
  |                |                | + GPS location                      |
  |                |                |                   |                  |
  |                |                | Tao LiveKit room  |                  |
  |                |                | de nguoi than     |                  |
  |                |                | co the goi lai   |                  |
  |                |                |                   |                  |
  |                | <-- room_id ---|                   |                  |
  |                |                |                   |                  |
  | Nghe: "Da gui  |                |                   |                  |
  | canh bao den   |                |                   |                  |
  | tat ca nguoi   |                |                   |                  |
  | than."         |                |                   |                  |
  |<---------------|                |                   |                  |
```

**Đặc điểm:**
- Emergency alert là tool call DUY NHẤT không cần xác nhận khi user có vẻ nguy hiểm
- Gửi push đến TẤT CẢ người thân (không chỉ primary contact)
- Kèm theo GPS vị trí hiện tại
- Tạo LiveKit room để người thân có thể gọi lại ngay

## 7. Tool Call Flow (Tổng quát)

Tất cả 20 tools đều đi theo một luồng chung. Dưới đây là luồng tổng quát.

```
                    Gemini Live API
                         |
                         | Tool call: { name, params }
                         v
                    Agent Worker (Python)
                         |
            +------------+------------+
            |                         |
    App-side tools            Backend-side tools
    (qua data channel)       (qua HTTP)
            |                         |
            v                         v
    Flutter App               Go Backend
    - enable_camera           - make_voice_call
    - disable_camera          - send_chat_message
    - open_maps               - emergency_alert
    - play_audio              - set_medication_reminder
    - control_audio           - log_health_symptom
                              - ... (16 tools)
                                      |
                                      v
                              PostgreSQL / Redis
                              (lưu dữ liệu, queue)
```

### Chi tiết một tool call (ví dụ: set_medication_reminder)

```
Gemini                Agent Worker              Go Backend            PostgreSQL
  |                       |                        |                     |
  | Tool call:            |                        |                     |
  | set_medication_       |                        |                     |
  | reminder(             |                        |                     |
  |   name="Amlodipine",  |                        |                     |
  |   dosage="5mg",       |                        |                     |
  |   times=["8:00"],     |                        |                     |
  |   frequency="daily")  |                        |                     |
  +---------------------->|                        |                     |
  |                       |                        |                     |
  |                       | POST /api/internal/    |                     |
  |                       | tool-call              |                     |
  |                       | {                      |                     |
  |                       |   "tool": "set_med..", |                     |
  |                       |   "user_id": "...",    |                     |
  |                       |   "params": { ... }    |                     |
  |                       | }                      |                     |
  |                       +----------------------->|                     |
  |                       |                        |                     |
  |                       |                        | INSERT INTO         |
  |                       |                        | medications(...)    |
  |                       |                        +-------------------->|
  |                       |                        |                     |
  |                       |                        | <-- OK ------------|
  |                       |                        |                     |
  |                       |  Data channel -------->| App đặt local      |
  |                       |  "set_local_notif"     | notification       |
  |                       |                        | cho 8:00 hàng ngày |
  |                       |                        |                     |
  |                       | <-- { ok, med_id } ----|                     |
  |                       |                        |                     |
  | <-- Tool result:      |                        |                     |
  | { status: "created",  |                        |                     |
  |   med_id: "..." }     |                        |                     |
  |<----------------------|                        |                     |
  |                       |                        |                     |
  | Trả lời user:         |                        |                     |
  | "Đã đặt nhắc thuốc    |                        |                     |
  | Amlodipine 5mg        |                        |                     |
  | lúc 8 giờ sáng        |                        |                     |
  | hàng ngày."           |                        |                     |
```

### Xác nhận trước khi thực hiện (Confirmation Flow)

Hầu hết tool call đều qua bước xác nhận:

```
User: "Gọi cho con gái"

Gemini: "Tôi sẽ gọi cho Lan nhé?"        <-- Hỏi xác nhận
                 |
     +-----------+-----------+
     |                       |
User: "Ừ" / "Có"      User: "Không"
     |                       |
     v                       v
Tool call thực hiện    Hủy bỏ, tiếp tục
                       hội thoại
```

**Ngoại lệ:** `emergency_alert` KHÔNG cần xác nhận khi user có vẻ nguy hiểm
("cứu tôi", "đau quá", "ngã rồi").

## 8. Session Reconnect (Tiếp nối session)

Gemini Live API giới hạn 15 phút (audio-only). Khi gần hết thời gian:

```
Agent Worker                     Gemini Live
     |                              |
     |  Theo dõi: 13 phút đã trôi   |
     |                              |
     |  Nói với user: "Tôi cần     |
     |  kết nối lại một chút"       |
     |                              |
     |  Tạo summary 10 turn cuối    |
     |  Lưu vào Redis:              |
     |  session:{user_id}           |
     |                              |
     |  Đóng session cũ ----------->| Session closed
     |                              |
     |  Mở session mới,             |
     |  inject summary vào          |
     |  system instruction -------->| Session mới
     |                              |
     |  "Đã kết nối lại.            |
     |   Chúng ta đang nói          |
     |   về [topic]."               |
     |                              |
     |  Thời gian mất: 1-3 giây     |
```

**Hầu hết user sẽ không gặp giới hạn này.** Tương tác thường mất 1-5 phút
(hỏi đáp, gọi điện, kiểm tra thuốc).

## 9. Data Channel Protocol

Agent Worker và Flutter App giao tiếp qua LiveKit data channel cho các lệnh
cần app thực hiện tại local.

### Format tin nhắn (JSON)

**Agent --> App:**
```json
{
  "type": "tool_action",
  "request_id": "uuid-v4",
  "action": "enable_camera | disable_camera | open_maps | play_audio | control_audio",
  "params": { }
}
```

**App --> Agent (xác nhận):**
```json
{
  "type": "tool_action_ack",
  "request_id": "uuid-v4",
  "status": "ok | error",
  "error_message": "Camera permission denied"
}
```

### Xử lý lỗi

- App không ACK trong 5 giây --> Agent thử lại 1 lần
- Thử lại thất bại --> Agent nói với user: "Tôi không thực hiện được. Bạn thử [hành động thủ công] nhé."
- Camera bị từ chối quyền --> Agent hướng dẫn user cấp quyền trong cài đặt
- Phát nhạc lỗi --> Agent đề xuất thử nguồn khác
