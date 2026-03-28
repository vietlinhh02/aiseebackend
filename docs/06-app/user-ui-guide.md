# Hướng dẫn User UI (Voice-First)

## Nguyên tắc thiết kế

User UI được thiết kế cho người suy giảm thị lực. Mọi thứ phải nhìn rõ, chạm dễ, và nghe được.

| Nguyên tắc | Chi tiết |
|---|---|
| Voice-first | Mọi tương tác đều có thể thực hiện bằng giọng nói |
| Font lớn | Tối thiểu 24sp cho tất cả text |
| High contrast | Nền tối, chữ sáng, tỷ lệ contrast >= 7:1 (WCAG AAA) |
| Touch target lớn | Tối thiểu 64x64dp cho mọi nút bấm |
| accessibilityLabel | Mọi element đều có label cho screen reader |
| Tap anywhere | Chạm bất kỳ đâu trên màn hình = kích hoạt mic |
| Không cần navigation | Tất cả qua giọng nói, không cần swipe hay tìm menu |

## Gemini auto-connect

Khi user mở app:

```
1. App mở → auto-login (JWT cached)
2. Kết nối LiveKit → Agent Worker join room
3. Gemini chào: "Xin chào [tên], tôi có thể giúp gì?"
4. Microphone tự động bật → sẵn sàng nghe
```

User không cần thao tác gì. Mở app là nói chuyện được ngay.

Nếu mất mạng, app thông báo bằng giọng: "Ban dang offline. Toi chi co the nhac thuoc va doc tin nhan da luu."

## Các màn hình

### 1. User Home Screen

Màn hình chính, hiển thị ngay khi mở app.

```
┌─────────────────────────────┐
│  ┌─────────────────────┐    │
│  │  Status Bar          │    │
│  │  "Gemini đang nghe…" │    │
│  └─────────────────────┘    │
│                              │
│  ┌──────┐ ┌──────┐ ┌──────┐│
│  │      │ │      │ │      ││
│  │ SOS  │ │ CALL │ │ CAM  ││
│  │      │ │      │ │      ││
│  └──────┘ └──────┘ └──────┘│
│                              │
│  ┌─────────────────────┐    │
│  │  Thẻ nhắc thuốc     │    │
│  │  "Uống Amlodipine   │    │
│  │   5mg lúc 8:00"     │    │
│  └─────────────────────┘    │
│                              │
│  ┌─────────────────────┐    │
│  │  Tin nhắn chưa đọc  │    │
│  │  "3 tin nhắn mới"   │    │
│  └─────────────────────┘    │
│                              │
│  [Chạm bất kỳ đâu = mic]   │
└─────────────────────────────┘
```

Thành phần:

- **Status Bar** -- Hiển thị trạng thái Gemini ("Đang nghe...", "Đang trả lời...", "Offline")
- **3 nút lớn** -- SOS, Call, Camera (chi tiết bên dưới)
- **Thẻ nhắc thuốc** -- Thuốc cần uống tiếp theo, thời gian
- **Badge tin nhắn chưa đọc** -- Số tin nhắn mới từ người thân
- **Vùng chạm** -- Toàn bộ vùng trống trên màn hình khi chạm sẽ kích hoạt mic

### 2. Camera Overlay

Hiện khi Gemini kích hoạt camera (hoặc user nhấn nút Camera).

```
┌─────────────────────────────┐
│                              │
│                              │
│     [Camera preview          │
│      toàn màn hình]          │
│                              │
│                              │
│  ┌──────────────────────┐   │
│  │    TẮT CAMERA        │   │
│  │    (64x64dp)         │   │
│  └──────────────────────┘   │
└─────────────────────────────┘
```

- Camera preview chiếm toàn màn hình
- Nút "Tắt camera" lớn ở dưới cùng
- Gemini mô tả bằng giọng nói những gì nhìn thấy
- Tự động tắt khi Gemini gọi `deactivate_camera`
- Frame rate: 1 FPS (tiết kiệm pin và token)

### 3. Call Screen

Hiện khi đang trong cuộc gọi VoIP.

```
┌─────────────────────────────┐
│                              │
│      Đang gọi với            │
│        LAN                   │
│      03:25                   │
│                              │
│  ┌──────┐ ┌──────┐ ┌──────┐│
│  │      │ │      │ │      ││
│  │ MIC  │ │SPEAKER│ │ END  ││
│  │      │ │      │ │      ││
│  └──────┘ └──────┘ └──────┘│
│                              │
└─────────────────────────────┘
```

- Tên người gọi, thời lượng cuộc gọi (font >=32sp)
- 3 nút lớn: Mute/Unmute, Speaker, Kết thúc
- Nút Kết thúc màu đỏ, nổi bật
- Tất cả nút 64x64dp trở lên

## 3 nút lớn

### Nút SOS (Khẩn cấp)

- **Màu:** Đỏ (#FF0000)
- **Kích thước:** >= 80x80dp
- **Hành vi:** Nhấn giữ 2 giây -> kích hoạt `emergency_alert`
  - Gửi push + GPS tới tất cả người thân
  - Tạo LiveKit room cho cuộc gọi nhóm khẩn cấp
  - Gemini xác nhận: "Da gui tin hieu khan cap toi gia dinh"
- **accessibilityLabel:** "Nút khẩn cấp SOS. Nhấn giữ 2 giây để gửi cảnh báo tới gia đình"
- **Nhấn giữ** (không phải nhấn thường) để tránh kích hoạt nhầm

### Nút Call (Gọi)

- **Màu:** Xanh lá (#00C853)
- **Kích thước:** >= 80x80dp
- **Hành vi:** Nhấn -> Gemini hỏi "Ban muon goi cho ai?" -> User nói tên -> gọi
- **accessibilityLabel:** "Nút gọi điện. Nhấn để gọi cho người thân"

### Nút Camera

- **Màu:** Xanh dương (#2979FF)
- **Kích thước:** >= 80x80dp
- **Hành vi:** Nhấn -> bật camera -> Gemini nhìn và mô tả
- **accessibilityLabel:** "Nút camera. Nhấn để AI nhìn qua camera và mô tả"

## Accessibility (Tiếp cận)

### VoiceOver (iOS) / TalkBack (Android)

Mọi element có `Semantics` widget với label tiếng Việt:

```dart
Semantics(
  label: 'Nút khẩn cấp SOS. Nhấn giữ 2 giây để gửi cảnh báo',
  button: true,
  child: SOSButton(),
)
```

### Navigation pattern

Screen reader navigation theo thứ tự:
1. Status bar (trạng thái Gemini)
2. SOS button
3. Call button
4. Camera button
5. Thẻ nhắc thuốc (nếu có)
6. Badge tin nhắn (nếu có)

### Voice feedback

Mọi hành động có phản hồi bằng giọng:

| Hành động | Phản hồi |
|---|---|
| Mở app | Gemini chào |
| Nhấn SOS | "Đã gửi tín hiệu khẩn cấp" |
| Nhấn Call | "Bạn muốn gọi cho ai?" |
| Nhấn Camera | "Đã bật camera" |
| Mất mạng | "Bạn đang offline" |
| Có tin nhắn mới | "Bạn có [N] tin nhắn mới" |

### Tap anywhere

Toàn bộ vùng trống trên `UserHomeScreen` là một `GestureDetector`. Chạm vào bất kỳ đâu ngoài 3 nút sẽ kích hoạt mic (nếu đang mute) hoặc gửi tín hiệu "đang nói" cho Gemini. Giúp user không cần tìm nút cụ thể để tương tác.
