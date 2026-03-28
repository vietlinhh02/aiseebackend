# Cấu trúc Flutter Project

## Directory tree

```
aisee_app/
├── lib/
│   ├── main.dart                        # Entry point, khởi tạo ProviderScope
│   ├── app.dart                         # MaterialApp, routing, theme
│   │
│   ├── core/
│   │   ├── theme/
│   │   │   ├── app_theme.dart           # Base theme, shared colors
│   │   │   ├── user_theme.dart          # High contrast, font >=24sp
│   │   │   └── family_theme.dart        # Standard Material theme
│   │   ├── constants.dart               # API URLs, timeout, kích thước
│   │   └── router.dart                  # GoRouter config
│   │
│   ├── services/
│   │   ├── api_client.dart              # Dio HTTP client tới Go Backend
│   │   ├── livekit_service.dart         # LiveKit connect, data channel
│   │   ├── push_service.dart            # APNs/FCM token management
│   │   ├── local_notif_service.dart     # Nhắc thuốc local
│   │   ├── audio_player_service.dart    # Nhạc/radio/podcast
│   │   ├── auth_service.dart            # Google Sign-In + JWT
│   │   └── local_db_service.dart        # Drift database
│   │
│   ├── models/
│   │   ├── user.dart                    # User, role, settings
│   │   ├── message.dart                 # Chat message
│   │   ├── medication.dart              # Thuốc, lịch uống
│   │   ├── health_record.dart           # Triệu chứng, chỉ số
│   │   └── call_record.dart             # Lịch sử cuộc gọi
│   │
│   ├── providers/
│   │   ├── auth_provider.dart           # Auth state
│   │   ├── livekit_provider.dart        # LiveKit connection state
│   │   ├── chat_provider.dart           # Messages, unread count
│   │   ├── health_provider.dart         # Medications, symptoms, metrics
│   │   ├── music_provider.dart          # Playback state, queue
│   │   └── contact_provider.dart        # Family contacts
│   │
│   ├── features/
│   │   ├── auth/
│   │   │   ├── login_screen.dart        # Google Sign-In
│   │   │   └── role_select_screen.dart  # Chọn role: user hoặc family
│   │   │
│   │   ├── user/                        # UI Voice-First
│   │   │   ├── user_home_screen.dart    # 3 nút lớn + Gemini status
│   │   │   ├── camera_overlay.dart      # Camera view khi kích hoạt
│   │   │   ├── call_screen.dart         # Giao diện trong cuộc gọi
│   │   │   └── widgets/
│   │   │       ├── sos_button.dart
│   │   │       ├── call_button.dart
│   │   │       ├── camera_button.dart
│   │   │       ├── status_bar.dart      # "Gemini đang nghe..."
│   │   │       ├── unread_badge.dart
│   │   │       └── medication_card.dart
│   │   │
│   │   ├── family/                      # UI Screen-Based
│   │   │   ├── family_home_screen.dart
│   │   │   ├── chat_screen.dart
│   │   │   ├── health_screen.dart
│   │   │   ├── calls_screen.dart
│   │   │   ├── settings_screen.dart
│   │   │   └── widgets/
│   │   │       ├── health_chart.dart
│   │   │       ├── medication_list.dart
│   │   │       ├── symptom_timeline.dart
│   │   │       └── location_card.dart
│   │   │
│   │   └── shared/
│   │       ├── chat/
│   │       │   ├── chat_page.dart
│   │       │   └── voice_message_bubble.dart
│   │       ├── contacts/
│   │       │   ├── contact_list.dart
│   │       │   └── invite_family.dart
│   │       └── incoming_call_handler.dart
│   │
│   └── db/
│       ├── app_database.dart            # Drift database definition
│       └── tables/
│           ├── cached_messages.dart
│           ├── cached_medications.dart
│           └── cached_contacts.dart
│
├── android/
│   └── app/src/main/
│       ├── AndroidManifest.xml
│       └── res/xml/shortcuts.xml        # "Hey Google, mở AiSee"
│
├── ios/
│   └── Runner/
│       ├── AppDelegate.swift
│       ├── Info.plist                   # VoIP background mode
│       └── AiSeeIntents.swift           # Siri "Open AiSee"
│
├── pubspec.yaml
└── analysis_options.yaml
```

## Giải thích từng thư mục

### `core/`

Chứa các thành phần nền tảng dùng chung cho toàn app.

- **`theme/`** -- Hai bộ theme riêng biệt. `user_theme.dart` dùng high contrast, font lớn (>=24sp), touch target 64x64dp cho người suy giảm thị lực. `family_theme.dart` dùng Material Design chuẩn cho người thân. `app_theme.dart` chứa base colors và typography dùng chung.
- **`constants.dart`** -- API URLs, timeout values, kích thước UI chuẩn.
- **`router.dart`** -- GoRouter configuration. Routing phân nhánh theo role: user vào `user_home_screen`, family vào `family_home_screen`.

### `services/`

Lớp service xử lý giao tiếp với bên ngoài (network, native platform, local storage).

| File | Chức năng |
|---|---|
| `api_client.dart` | Dio HTTP client, tự đính JWT vào header, auto-refresh token |
| `livekit_service.dart` | Kết nối LiveKit, quản lý audio/video track, xử lý data channel |
| `push_service.dart` | Lấy APNs/FCM token, gửi lên backend |
| `local_notif_service.dart` | Schedule local notification cho nhắc thuốc, nhắc lịch hẹn |
| `audio_player_service.dart` | Wrapper quanh `just_audio`, quản lý playback state |
| `auth_service.dart` | Google Sign-In flow, lưu JWT vào `flutter_secure_storage` |
| `local_db_service.dart` | Drift database cho offline cache |

### `models/`

Data classes thuần (immutable). Mỗi model map 1:1 với JSON response từ Go Backend.

- `user.dart` -- User profile, role (`user` hoặc `family`), settings
- `message.dart` -- Chat message (text hoặc voice URL)
- `medication.dart` -- Thuốc, liều lượng, thời gian uống, trạng thái
- `health_record.dart` -- Triệu chứng (symptom) và chỉ số sức khỏe (metric)
- `call_record.dart` -- Lịch sử cuộc gọi (LiveKit room metadata)

### `providers/`

Riverpod providers quản lý state toàn app.

| Provider | State quản lý |
|---|---|
| `auth_provider.dart` | User hiện tại, JWT, login/logout |
| `livekit_provider.dart` | Connection state, room, data channel listener |
| `chat_provider.dart` | Danh sách tin nhắn, unread count, WebSocket |
| `health_provider.dart` | Medications, symptoms, metrics, appointments |
| `music_provider.dart` | Playback state, queue, current track |
| `contact_provider.dart` | Family contacts, invite codes |

### `features/`

Chứa UI screens và widgets, tổ chức theo feature.

- **`auth/`** -- Login flow. Sau khi đăng nhập, user chọn role (user hoặc family) để router điều hướng đúng UI.
- **`user/`** -- UI voice-first cho người suy giảm thị lực. Tối giản, 3 nút lớn (SOS, Call, Camera), tap anywhere kích hoạt mic.
- **`family/`** -- UI screen-based cho người thân. 4 tabs (Chat, Health, Calls, Settings), hiển thị đầy đủ thông tin.
- **`shared/`** -- Components dùng chung giữa hai UI: chat page, voice message bubble, contact list, incoming call handler.

### `db/`

Drift (SQLite) database cho offline cache.

- **`app_database.dart`** -- Database definition, migration logic
- **`tables/`** -- Table definitions cho cached messages, medications, contacts. Dữ liệu sync từ backend khi có mạng, đọc từ local khi offline.

## State Management với Riverpod

AiSee sử dụng Riverpod để quản lý state vì:

1. **Compile-safe** -- Lỗi dependency được phát hiện lúc compile, không phải runtime
2. **Không phụ thuộc BuildContext** -- Provider truy cập được từ bất kỳ đâu
3. **Auto-dispose** -- Tự giải phóng state khi không còn listener
4. **Testable** -- Override provider dễ dàng trong test

### Kiến trúc provider

```
UI Widget
    │
    ▼
Provider (state + logic)
    │
    ▼
Service (network, platform)
    │
    ▼
Go Backend API / LiveKit / Local DB
```

### Các loại provider sử dụng

| Loại | Dùng cho |
|---|---|
| `StateNotifierProvider` | State phức tạp có nhiều action (auth, chat, health) |
| `FutureProvider` | Fetch data một lần (user profile, contact list) |
| `StreamProvider` | Data realtime (WebSocket messages, LiveKit events) |
| `Provider` | Computed values, service instances |

### Ví dụ: LiveKit Provider

```dart
// providers/livekit_provider.dart

final livekitProvider =
    StateNotifierProvider<LiveKitNotifier, LiveKitState>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final authState = ref.watch(authProvider);
  return LiveKitNotifier(apiClient, authState.userId);
});

class LiveKitState {
  final ConnectionState connectionState;
  final bool isCameraActive;
  final bool isMicMuted;
  // ...
}
```

Widget lắng nghe thay đổi và rebuild tự động:

```dart
// features/user/user_home_screen.dart

class UserHomeScreen extends ConsumerWidget {
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final lkState = ref.watch(livekitProvider);

    return Scaffold(
      body: Column(
        children: [
          StatusBar(state: lkState.connectionState),
          // 3 nút lớn...
        ],
      ),
    );
  }
}
```
