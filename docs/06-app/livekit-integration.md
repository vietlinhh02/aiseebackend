# Tích hợp LiveKit trong Flutter

## Tổng quan

AiSee sử dụng một kết nối LiveKit duy nhất cho tất cả giao tiếp realtime:

- Hội thoại AI với Gemini Live (audio)
- Camera vision on-demand (video)
- Gọi VoIP với người thân/tình nguyện viên
- Data channel (nhận tool actions từ Agent Worker)

Package: `livekit_client` (Flutter SDK chính thức của LiveKit).

## Kết nối LiveKit từ Flutter

### Lấy token

App gọi Go Backend để lấy LiveKit room token:

```
POST /api/v1/calls/token
Authorization: Bearer <jwt>

Response:
{
  "token": "<livekit-room-token>",
  "url": "wss://livekit.aisee.vn"
}
```

Token chứa: user identity, room name, permissions (publish audio, subscribe).

### Kết nối

```dart
final room = Room();

await room.connect(
  'wss://livekit.aisee.vn',
  token,
  roomOptions: RoomOptions(
    adaptiveStream: true,
    dynacast: true,
    defaultAudioPublishOptions: AudioPublishOptions(
      audioBitrate: AudioPresets.speech.maxBitrate,
    ),
  ),
);
```

Sau khi kết nối, app publish audio track (microphone) để Gemini nghe.

## Room lifecycle

### Luồng chuẩn (user mở app)

```
1. App mở → auto-login (JWT cached)
2. GET /api/v1/calls/token → nhận LiveKit token
3. room.connect(url, token) → join personal room
4. Agent Worker auto-dispatch vào room (LiveKit agent framework)
5. Gemini bắt đầu: "Xin chào [tên], tôi có thể giúp gì?"
6. Session audio-only chạy tối đa 15 phút
7. Gần hết 15 phút → Agent tự reconnect (xem Section 18 trong spec)
8. User đóng app → room.disconnect()
```

### Trạng thái kết nối

| State | Ý nghĩa |
|---|---|
| `ConnectionState.disconnected` | Chưa kết nối |
| `ConnectionState.connecting` | Đang kết nối |
| `ConnectionState.connected` | Đã kết nối, sẵn sàng |
| `ConnectionState.reconnecting` | Mất kết nối, đang thử lại |

App hiển thị trạng thái trên `StatusBar`:
- Connected: "Gemini đang nghe..."
- Reconnecting: "Đang kết nối lại..."
- Disconnected: "Offline"

## Quản lý Audio Track

### Publish microphone

```dart
await room.localParticipant?.setMicrophoneEnabled(true);
```

Microphone được bật ngay khi kết nối. Gemini nhận audio stream liên tục.

### Mute/unmute

```dart
// Mute
await room.localParticipant?.setMicrophoneEnabled(false);

// Unmute
await room.localParticipant?.setMicrophoneEnabled(true);
```

User UI: tap anywhere trên màn hình để toggle mic (mặc định luôn bật).

### Nhận audio từ Agent (Gemini voice)

```dart
room.onTrackSubscribed = (track, publication, participant) {
  if (track is AudioTrack) {
    // Audio tự động phát qua speaker
    // Không cần xử lý thêm
  }
};
```

LiveKit SDK tự động phát audio track từ remote participant (Agent Worker / Gemini) qua speaker.

## Quản lý Video Track (on-demand camera)

Camera chỉ bật khi cần (user yêu cầu hoặc Gemini quyết định). Mặc định audio-only để tiết kiệm pin và kéo dài session (15 phút thay vì 2 phút).

### Bật camera

```dart
await room.localParticipant?.setCameraEnabled(true);
```

Khi camera bật, video track được publish lên room. Gemini nhận frames và phân tích.

### Tắt camera

```dart
await room.localParticipant?.setCameraEnabled(false);
```

Video track bị unpublish. Session quay lại audio-only.

### Hiển thị Camera Overlay

Khi camera bật, app hiển thị `CameraOverlay` lên trên `UserHomeScreen`:

```dart
if (isCameraActive) {
  CameraOverlay(
    videoTrack: room.localParticipant?.videoTrackPublications
        .first?.track as LocalVideoTrack,
  );
}
```

Camera overlay chiếm toàn màn hình với nút "Tắt camera" lớn (64x64dp) ở góc.

### Camera frame rate

Publish video ở 1 FPS để giảm bandwidth và token usage. Gemini không cần video tốc độ cao để đọc nhãn thuốc hay mô tả cảnh vật.

```dart
await room.localParticipant?.setCameraEnabled(
  true,
  cameraCaptureOptions: CameraCaptureOptions(
    maxFrameRate: 1,
  ),
);
```

## Data Channel -- nhận tool actions từ Agent

Agent Worker gửi các lệnh qua LiveKit data channel để app thực hiện hành động local (bật camera, phát nhạc, mở bản đồ).

### Lắng nghe data channel

```dart
room.onDataReceived = (data, participant, topic) {
  final message = jsonDecode(utf8.decode(data));
  _handleToolAction(message);
};
```

### Xử lý tool actions

```dart
void _handleToolAction(Map<String, dynamic> message) {
  if (message['type'] != 'tool_action') return;

  final requestId = message['request_id'];
  final action = message['action'];
  final params = message['params'] ?? {};

  switch (action) {
    case 'enable_camera':
      _enableCamera(requestId);
    case 'disable_camera':
      _disableCamera(requestId);
    case 'open_maps':
      _openMaps(requestId, params['destination']);
    case 'play_audio':
      _playAudio(requestId, params['url'], params['title']);
    case 'control_audio':
      _controlAudio(requestId, params['command']);
  }
}
```

### Gửi acknowledgment

```dart
void _sendAck(String requestId, {String? error}) {
  final ack = jsonEncode({
    'type': 'tool_action_ack',
    'request_id': requestId,
    'status': error == null ? 'ok' : 'error',
    if (error != null) 'error_message': error,
  });

  room.localParticipant?.publishData(
    utf8.encode(ack),
    reliable: true,
  );
}
```

### Protocol xử lý lỗi

- Agent đợi ACK tối đa 5 giây
- Không nhận ACK -> Agent retry 1 lần
- Retry thất bại -> Agent thông báo bằng giọng nói: "Toi khong the thuc hien duoc..."
- Camera permission denied -> App gửi ACK error, Agent hướng dẫn user cấp quyền

## Xử lý Incoming Call

### iOS -- CallKit (qua PushKit VoIP Push)

```
1. Go Backend gửi VoIP Push qua APNs
2. iOS nhận push -> đánh thức app -> hiện native call screen
3. User trả lời (swipe hoặc voice command)
4. App nhận callback -> lấy LiveKit token -> join room
5. flutter_callkit_incoming xử lý native call UI
```

Cấu hình trong `Info.plist`:

```xml
<key>UIBackgroundModes</key>
<array>
  <string>voip</string>
  <string>audio</string>
</array>
```

### Android -- FCM High-Priority + ConnectionService

```
1. Go Backend gửi FCM high-priority message
2. Android nhận -> hiển thị incoming call UI (flutter_callkit_incoming)
3. User trả lời
4. App lấy LiveKit token -> join room
```

### flutter_callkit_incoming setup

```dart
final params = CallKitParams(
  id: callId,
  nameCaller: callerName,
  type: 0, // voice call
  duration: 30000, // ring 30 giây
  textAccept: 'Nghe',
  textDecline: 'Từ chối',
  android: AndroidParams(
    isShowLogo: true,
    ringtonePath: 'ringtone_default',
    backgroundColor: '#0955fa',
  ),
  ios: IOSParams(
    handleType: 'generic',
    supportsVideo: false,
    audioSessionMode: 'default',
    audioSessionActive: true,
    audioSessionPreferredSampleRate: 44100.0,
    audioSessionPreferredIOBufferDuration: 0.005,
    ringtonePath: 'ringtone_default',
  ),
);

await FlutterCallkitIncoming.showCallkitIncoming(params);
```

### Call lifecycle

```
Incoming push
    │
    ▼
Hiện native call screen (CallKit / ConnectionService)
    │
    ├─ User từ chối → dismiss, gửi decline event
    │
    └─ User trả lời
        │
        ▼
    Lấy LiveKit token (POST /api/v1/calls/token)
        │
        ▼
    room.connect(url, token) → join call room
        │
        ▼
    Cuộc gọi diễn ra (audio, có thể video)
        │
        ▼
    Một bên kết thúc → room.disconnect()
        │
        ▼
    Đóng call screen
```

### Trong cuộc gọi, Gemini vẫn available

Khi user đang gọi với người thân, Agent Worker vẫn ở trong room. User có thể yêu cầu Gemini hỗ trợ (ví dụ: "AiSee, dịch giúp tôi câu này"). Agent Worker sẽ mute khi không cần thiết để không gây nhiễu.
