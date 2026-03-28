# Hành vi Offline

## Tổng quan

AiSee hoạt động chủ yếu qua mạng (Gemini AI, LiveKit, Go Backend). Khi mất mạng, app chuyển sang chế độ offline với chức năng giới hạn.

## Hoạt động được khi offline

### 1. Đọc tin nhắn đã cache

- Tin nhắn đã tải về được lưu trong Drift (SQLite) database
- User có thể xem lại tin nhắn cũ
- Gemini không đọc được (cần mạng), nhưng screen reader (VoiceOver/TalkBack) vẫn đọc được text messages

### 2. Xem lịch uống thuốc

- Medications đã sync được lưu trong `cached_medications` table
- Danh sách thuốc hôm nay hiển thị bình thường
- Trạng thái "đã uống" được lưu local, sync lên server khi có mạng

### 3. Local notifications (nhắc thuốc, nhắc lịch hẹn)

- `flutter_local_notifications` hoạt động hoàn toàn offline
- Nhắc thuốc đúng giờ bất kể có mạng hay không
- Nhắc lịch hẹn bác sĩ đúng giờ
- Notification schedule đã được tạo sẵn khi có mạng

### 4. SOS (hạn chế khi offline)

- SOS yêu cầu kết nối internet để gửi cảnh báo đến Go Backend
- Nếu offline, yêu cầu SOS được xếp vào hàng đợi và gửi ngay khi có kết nối trở lại
- Bao gồm GPS location cuối cùng đã biết
- **Đây là hạn chế quan trọng:** trong trường hợp khẩn cấp mà không có internet, SOS sẽ bị trì hoãn

### 5. Xem danh bạ gia đình

- Danh sách người thân cache trong `cached_contacts`
- Hiển thị tên, relationship, số điện thoại

## Không hoạt động khi offline

### 1. Hội thoại với Gemini

- Gemini Live API yêu cầu kết nối internet liên tục
- Không có chế độ offline cho AI conversation
- App thông báo: "Ban dang offline. Toi chi co the nhac thuoc va doc tin nhan da luu."

### 2. Camera vision

- Camera cần Gemini để phân tích hình ảnh
- Không có on-device vision model (tiết kiệm dung lượng app)
- Nút Camera disabled, voice alert khi nhấn: "Can ket noi internet de su dung camera"

### 3. Gọi VoIP

- LiveKit cần internet để thiết lập kết nối WebRTC
- Nút Call disabled
- Voice alert: "Can ket noi internet de goi dien"

### 4. Gửi tin nhắn mới

- Tin nhắn mới được queue lại, gửi khi có mạng (xem phần Message Queue)

### 5. Tìm kiếm, điều hướng, nhạc streaming

- Tất cả cần internet

## Auto-reconnect strategy

### LiveKit auto-reconnect (built-in)

LiveKit SDK có cơ chế reconnect tự động:

```dart
room.onDisconnected = (reason) {
  // LiveKit tự thử reconnect
  // Nếu thất bại sau 3 lần → báo user
};

room.onReconnected = () {
  _announceVoice('Da ket noi lai. Toi san sang.');
};
```

### Luồng reconnect

```
Mất kết nối
    │
    ▼
LiveKit SDK tự retry (exponential backoff)
    │
    ├─ Thành công trong 30 giây
    │   └─ Voice: "Đã kết nối lại"
    │   └─ Agent Worker resume session
    │
    └─ Thất bại sau 3 lần (khoảng 30 giây)
        │
        ▼
    App kiểm tra internet connectivity
        │
        ├─ Có internet → retry LiveKit connect
        │   └─ Lấy token mới: POST /api/v1/calls/token
        │   └─ room.connect(url, newToken)
        │
        └─ Không internet → chuyển sang chế độ offline
            └─ Voice: "Bạn đang offline"
            └─ Disable 3 nút chính
            └─ Hiện "Offline" trên status bar
```

### Monitoring connectivity

```dart
// Kiểm tra internet liên tục
Connectivity().onConnectivityChanged.listen((result) {
  if (result != ConnectivityResult.none && !isConnected) {
    _attemptReconnect();
  }
});
```

Khi phát hiện có internet trở lại:
1. Lấy LiveKit token mới
2. Kết nối lại room
3. Sync message queue
4. Voice: "Da ket noi lai. Toi san sang."

## Message Queue (gửi khi có mạng)

Tin nhắn gửi khi offline được lưu vào local queue:

### Lưu tin nhắn vào queue

```dart
// Khi user gửi tin nhắn mà offline
Future<void> sendMessage(Message msg) async {
  if (isOnline) {
    await apiClient.post('/messages/send', data: msg.toJson());
  } else {
    await localDb.pendingMessages.insert(msg);
    _announceVoice('Tin nhan se duoc gui khi co mang.');
  }
}
```

### Sync khi có mạng

```dart
Future<void> syncPendingMessages() async {
  final pending = await localDb.pendingMessages.getAll();
  for (final msg in pending) {
    try {
      await apiClient.post('/messages/send', data: msg.toJson());
      await localDb.pendingMessages.delete(msg.id);
    } catch (e) {
      break; // Dừng nếu mất mạng lại, giữ thứ tự
    }
  }
}
```

### Quy tắc queue

- FIFO -- tin nhắn gửi theo đúng thứ tự user tạo
- Retry -- nếu gửi thất bại, giữ lại trong queue
- Giới hạn -- tối đa 100 tin nhắn pending (tránh tràn storage)
- Thông báo -- voice alert khi tin nhắn pending được gửi thành công: "Da gui [N] tin nhan cho [ten]"

## Voice alerts cho trạng thái offline

Người suy giảm thị lực không thể đọc banner "Offline" trên màn hình. Mọi thay đổi trạng thái mạng phải thông báo bằng giọng nói.

| Event | Voice alert |
|---|---|
| Mất mạng | "Bạn đang offline. Tôi chỉ có thể nhắc thuốc và đọc tin nhắn đã lưu." |
| Có mạng lại | "Đã kết nối lại. Tôi sẵn sàng." |
| LiveKit reconnecting | "Đang kết nối lại, xin chờ..." |
| LiveKit reconnect failed | "Không kết nối được. Vui lòng kiểm tra internet." |
| Nhấn nút khi offline | "Cần kết nối internet để [hành động]." |
| Tin nhắn pending đã gửi | "Đã gửi [N] tin nhắn cho [tên]." |
| SOS khi offline | "Không có internet. Cảnh báo khẩn cấp sẽ được gửi ngay khi có kết nối." |

### Implementation

Voice alerts dùng `flutter_tts` hoặc `SemanticsService.announce`:

```dart
Future<void> _announceVoice(String message) async {
  // TTS cho user nghe
  await tts.speak(message);

  // Đồng thời announce cho screen reader
  SemanticsService.announce(message, TextDirection.ltr);
}
```

Ưu tiên TTS vì không phải lúc nào user cũng bật screen reader. TTS hoạt động offline (engine tích hợp sẵn trên iOS/Android).
