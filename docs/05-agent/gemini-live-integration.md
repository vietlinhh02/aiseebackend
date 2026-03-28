# Gemini Live Integration

## Tổng quan

AiSee sử dụng Gemini Live API thong qua LiveKit de tạo hội thoại giọng nói thời gian thuc với người dùng. Gemini Live xử lý đồng thời:
- Nghe giọng nói của user (audio input)
- Tra lỗi bang giọng nói (audio output)
- Nhin qua camera khi được bật (video input)
- Goi tools khi can thực hiện hành động

## Kien truc: Gemini Live qua LiveKit

```
Flutter App          LiveKit Server         Agent Worker
    │                     │                     │
    │  Audio track ──────>│────── Audio ──────>│
    │  (user noi)         │                    │  Gemini Live
    │                     │                    │  RealtimeModel
    │  <── Audio track ───│<────── Audio ──────│
    │  (Gemini tra loi)   │                    │
    │                     │                    │
    │  Video track ──────>│────── Frames ─────>│  (khi camera bat)
    │  (camera frames)    │                    │
    │                     │                    │
    │  <── Data channel ──│<────── JSON ───────│  (tool actions)
    │  ── Data channel ──>│─────── JSON ──────>│  (tool ACK)
```

LiveKit dong vai trò SFU (Selective Forwarding Unit), chuyen tiep media giua App và Agent Worker. Agent Worker không tu xử lý WebRTC — LiveKit SDK lam viec đó.

## RealtimeModel Configuration

```python
from livekit.plugins import google

model = google.realtime.RealtimeModel(
    model="gemini-2.5-flash-live-preview",
    system_instructions=SYSTEM_PROMPT_VI,
    voice="Aoede",           # Giong nu, ro rang
    temperature=0.7,         # Can bang giua creativity va consistency
    max_output_tokens=1024,
)
```

### Tham số quan trọng

| Tham số | Giá trị | Giải thích |
|---------|---------|-----------|
| `model` | `gemini-2.5-flash-live-preview` | Model ho tro realtime audio/video |
| `system_instructions` | System prompt tieng Viết | Xem file `system-prompt.md` |
| `voice` | `"Aoede"` | Giong noi. Can test các giong khac cho tieng Viết. |
| `temperature` | `0.7` | Thap hon = nhat quan hon. Cao hon = tu nhien hon. |
| `max_output_tokens` | `1024` | Gioi han do dai tra lỗi — giu ngan gon. |

### Model selection

| Model | Audio | Video | Tools | Session limit |
|-------|-------|-------|-------|---------------|
| `gemini-2.5-flash-live-preview` | Co | Co | Co | 15 min audio, 2 min audio+video |

Khi Gemini Live model mỗi ra, thay doi model ID o mot cho duy nhat.

## Audio Format

### Input (User -> Gemini)

| Thuoc tinh | Giá trị |
|------------|---------|
| Format | PCM (raw audio) |
| Bit depth | 16-bit |
| Sample rate | 16 kHz |
| Channels | Mono |

LiveKit Client SDK tự động encode audio tu microphone thanh format này.

### Output (Gemini -> User)

| Thuoc tinh | Giá trị |
|------------|---------|
| Format | PCM (raw audio) |
| Bit depth | 16-bit |
| Sample rate | 24 kHz |
| Channels | Mono |

LiveKit Client SDK tự động decode và play audio này qua loa/tại nghe.

### Ghi chú ve chat luong am thanh

- Gemini Live xử lý audio truc tiep, không cần STT/TTS rieng
- Chat luong tieng Viết can được validate o Phase 0 trước khi commit
- Nếu chat luong không du: fallback la pipeline STT -> LLM -> TTS qua LiveKit

## Video Frame Handling

### Mac dinh: Audio-only

Camera TAT mặc định. Chi bật khi:
- User yêu cầu ("doc cai này giup toi", "phia truoc co gì")
- User bam nut camera
- Gemini quyet dinh can nhin (qua tool `activate_camera`)

### Frame rate

| Che đó | FPS | Lý đó |
|--------|-----|-------|
| User dang noi (active) | 1 FPS | Du de Gemini nhin và mô tả |
| User im lang (idle) | 1/3 FPS (~0.33) | Tiet kiem tokens và bandwidth |

### Cach hoạt động

```
1. Gemini goi tool: activate_camera(reason="doc nhan thuoc")
2. Agent gui data channel: { "action": "enable_camera" }
3. App bat camera, publish video track len LiveKit
4. LiveKit chuyen video frames toi Agent Worker
5. Agent Worker forward frames toi Gemini Live session
6. Gemini nhin va mo ta: "Day la thuoc Amlodipine 5mg..."
7. Gemini goi tool: deactivate_camera()
8. Agent gui data channel: { "action": "disable_camera" }
9. App tat camera, unpublish video track
```

### Tai sao không bật camera lien tuc?

| Lý đó | Chi tiết |
|-------|---------|
| Session limit | Audio+video chi 2 phút. Audio-only 15 phút. |
| Pin | Camera + video encoding tieu ton nhiều pin |
| Tokens | Mỗi frame tieu ton tokens của Gemini API |
| Privacy | User không muon camera bật lien tuc |

## Provider Tools

### GoogleSearch

Gemini có thể tu tìm kiếm thông tin tren Google khi can:

```python
from livekit.plugins import google

session.provider_tools = [google.tools.GoogleSearch()]
```

GoogleSearch là "provider tool" — Gemini tu quyet dinh khi nào goi, không cần Agent route. Kết quả trả về truc tiep cho Gemini de tra lỗi user.

### Khi nào Gemini dung GoogleSearch

- User hoi thông tin thoi su, tin tuc
- User hoi cau hoi ma Gemini không biet cau tra lỗi
- User hoi thông tin dia phuong (nha hang, benh vien gan đây)

### Khi nào Gemini KHONG can tool

Gemini tu tra lỗi truc tiep (không goi tool nào) khi:
- Tro chuyen thong thuong
- Hoi ve thoi tiet (Gemini biet tu training data + search)
- Hoi kien thuc chung
- Giao duc / đây hoc
- Giai tri (ke chuyen, hat)

## Error Handling trong Gemini Session

| Loi | Xử lý |
|-----|-------|
| Gemini session disconnect | Tao session mỗi trong 1-3 giây. User nghe im lang ngan. |
| Gemini tra lỗi qua cham | Timeout 30 giây, thông báo user: "Toi dang xử lý, xin cho." |
| Gemini tu choi tra lỗi (safety filter) | Agent noi: "Toi không thể tra lỗi cau hoi này." |
| Tool call thất bại | Agent noi: "Toi không thực hiện được. Ban thử lại sau nhe." Ghi audit log. |
| Video frame send thất bại | Tat camera, quay lai audio-only. Thong bao user. |
