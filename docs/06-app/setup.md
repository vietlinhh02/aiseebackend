# Hướng dẫn cài đặt Flutter App

## Yêu cầu hệ thống

| Công cụ | Phiên bản tối thiểu | Ghi chú |
|---|---|---|
| Flutter SDK | 3.11+ | Bao gồm Dart SDK |
| Dart SDK | 3.3+ | Đi kèm Flutter |
| Android Studio | 2024.1+ | Bao gồm Android SDK, emulator |
| Xcode | 15.0+ | Chỉ trên macOS, cần cho iOS build |
| CocoaPods | 1.14+ | Cần cho iOS dependencies |
| Git | 2.30+ | Quản lý source code |

### Thiết bị test

- **Android:** API level 24+ (Android 7.0), khuyến nghị API 33+ (Android 13)
- **iOS:** iOS 16.0+, cần Apple Developer Account cho VoIP Push

## Cài đặt

### 1. Clone repository

```bash
git clone <repo-url> aisee_app
cd aisee_app
```

### 2. Cài đặt dependencies

```bash
flutter pub get
```

Với iOS, cài thêm native dependencies:

```bash
cd ios && pod install && cd ..
```

### 3. Cấu hình environment

Tạo file `.env` từ template:

```bash
cp .env.example .env
```

Nội dung `.env`:

```dotenv
# Go Backend API
API_BASE_URL=https://api.aisee.vn/api/v1

# LiveKit Server
LIVEKIT_URL=wss://livekit.aisee.vn

# Google OAuth2 Client ID
GOOGLE_CLIENT_ID_ANDROID=<your-android-client-id>
GOOGLE_CLIENT_ID_IOS=<your-ios-client-id>

# Mapbox
MAPBOX_ACCESS_TOKEN=<your-mapbox-token>
```

### 4. Cấu hình Google Sign-In

> **Lưu ý:** AiSee dùng Google Sign-In (OAuth2) để xác thực, KHÔNG dùng Firebase. File `google-services.json` chỉ chứa cấu hình Google OAuth2 Client ID, không phải Firebase config.

**Android:**
1. Tạo project trên Google Cloud Console
2. Tạo OAuth 2.0 Client ID cho Android (cần SHA-1 fingerprint)
3. Đặt `google-services.json` vào `android/app/` (file này chứa OAuth2 Client ID cho Google Sign-In, không liên quan đến Firebase)

**iOS:**
1. Tạo OAuth 2.0 Client ID cho iOS (cần Bundle Identifier)
2. Thêm URL scheme vào `ios/Runner/Info.plist`:
   ```xml
   <key>CFBundleURLTypes</key>
   <array>
     <dict>
       <key>CFBundleURLSchemes</key>
       <array>
         <string>com.googleusercontent.apps.YOUR_CLIENT_ID</string>
       </array>
     </dict>
   </array>
   ```

### 5. Cấu hình VoIP Push (iOS)

1. Bật **Push Notifications** capability trong Xcode
2. Bật **Background Modes** > Voice over IP
3. Tạo VoIP Services Certificate trên Apple Developer Portal
4. Upload certificate (.p12) lên Go Backend server

### 6. Cấu hình FCM (Android)

FCM được xử lý hoàn toàn từ Go Backend (HTTP v1 API). App chỉ cần lấy FCM token và gửi lên backend qua `POST /api/v1/users/me`.

## Chạy ứng dụng

### Trên emulator

```bash
# Liệt kê thiết bị
flutter devices

# Chạy trên Android emulator
flutter run -d emulator-5554

# Chạy trên iOS Simulator
flutter run -d "iPhone 15 Pro"
```

### Trên thiết bị thật

```bash
# Android: bật USB Debugging, kết nối cáp USB
flutter run -d <device-id>

# iOS: cần provisioning profile
flutter run -d <ios-device-id>
```

### Build modes

```bash
# Debug (mặc định, có hot reload)
flutter run

# Release (tối ưu, không debug)
flutter run --release

# Profile (đo performance)
flutter run --profile
```

## Lưu ý quan trọng

- **Camera và microphone** cần được cấp quyền thủ công trên thiết bị test
- **LiveKit server** phải chạy và accessible từ thiết bị (xem docs/07-infrastructure/)
- **Go Backend** phải chạy để auth và tool calls hoạt động
- Trên iOS Simulator, camera không hoạt động -- cần thiết bị thật để test camera
- Trên Android Emulator, microphone sử dụng mic của máy host
