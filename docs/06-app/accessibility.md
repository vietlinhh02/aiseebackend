# Accessibility (Tiếp cận cho người khuyết tật)

AiSee phục vụ người suy giảm thị lực. Accessibility không phải tính năng phụ -- nó là yêu cầu cốt lõi.

## VoiceOver (iOS) -- Checklist

VoiceOver là screen reader tích hợp sẵn trên iOS. User điều hướng bằng cách swipe trái/phải qua các element.

### Yêu cầu bắt buộc

- [ ] Mọi widget tương tác có `Semantics` với `label` tiếng Việt
- [ ] Mọi nút có `Semantics(button: true)`
- [ ] Mọi hình ảnh có `Semantics(image: true, label: 'mô tả')`
- [ ] Thứ tự focus hợp lý (trên xuống dưới, trái sang phải)
- [ ] Trạng thái thay đổi được announce (connected, offline, có tin nhắn mới)
- [ ] Custom actions cho gesture phức tạp (nhấn giữ SOS -> cung cấp alternative)
- [ ] Không có element bị ẩn khỏi VoiceOver trừ decoration thuần túy
- [ ] Test trên thiết bị thật (iPhone) với VoiceOver bật

### Kiểm tra

```
Settings > Accessibility > VoiceOver > ON
```

Swipe qua từng element trên mỗi màn hình. Đảm bảo VoiceOver đọc đúng:
1. Tên element
2. Loại (nút, text, hình ảnh)
3. Trạng thái (nếu có)
4. Hướng dẫn thao tác (nếu có)

## TalkBack (Android) -- Checklist

TalkBack là screen reader tích hợp sẵn trên Android. Hoạt động tương tự VoiceOver.

### Yêu cầu bắt buộc

- [ ] Mọi widget tương tác có `Semantics` với `label` tiếng Việt
- [ ] `contentDescription` không trống cho ImageView/Icon
- [ ] Thứ tự focus (`sortKey` trong `Semantics`) đúng logic
- [ ] Live region cho status bar (`liveRegion: true`) để announce thay đổi tự động
- [ ] Headings đánh dấu đúng (`header: true` trong `Semantics`)
- [ ] Custom actions cho nhấn giữ (SOS button)
- [ ] Test trên thiết bị thật với TalkBack bật

### Kiểm tra

```
Settings > Accessibility > TalkBack > ON
```

## accessibilityLabel cho mọi element

Mỗi element tương tác phải có label tiếng Việt rõ ràng, mô tả chức năng chứ không phải hình dạng.

### Quy tắc đặt label

| Sai | Đúng |
|---|---|
| "Nút đỏ" | "Nút khẩn cấp SOS. Nhấn giữ 2 giây để gửi cảnh báo tới gia đình" |
| "Icon phone" | "Nút gọi điện. Nhấn để gọi cho người thân" |
| "Hình camera" | "Nút camera. Nhấn để AI nhìn qua camera và mô tả" |
| "Checkbox" | "Amlodipine 5mg, 8 giờ sáng, chưa uống" |

### Ví dụ code

```dart
Semantics(
  label: 'Nút khẩn cấp SOS. Nhấn giữ 2 giây để gửi cảnh báo tới gia đình',
  button: true,
  child: GestureDetector(
    onLongPress: _triggerSOS,
    child: Container(
      width: 80,
      height: 80,
      decoration: BoxDecoration(
        color: Colors.red,
        borderRadius: BorderRadius.circular(16),
      ),
      child: Center(
        child: Text('SOS', style: TextStyle(fontSize: 28)),
      ),
    ),
  ),
)
```

### Trạng thái động

Khi trạng thái thay đổi, label cần cập nhật:

```dart
Semantics(
  label: isConnected
      ? 'Gemini đang nghe. Sẵn sàng trò chuyện'
      : 'Offline. Không có kết nối internet',
  liveRegion: true, // TalkBack tự announce khi thay đổi
  child: StatusBar(isConnected: isConnected),
)
```

## Dynamic Type / Font Scaling

### Flutter implementation

User UI luôn dùng font lớn, bất kể system setting:

```dart
// core/theme/user_theme.dart

TextTheme _userTextTheme() {
  return TextTheme(
    displayLarge: TextStyle(fontSize: 40, fontWeight: FontWeight.bold),
    headlineMedium: TextStyle(fontSize: 32, fontWeight: FontWeight.w600),
    bodyLarge: TextStyle(fontSize: 24),  // Minimum cho User UI
    bodyMedium: TextStyle(fontSize: 24),
    labelLarge: TextStyle(fontSize: 28, fontWeight: FontWeight.bold),
  );
}
```

### Hỗ trợ system font scaling

App phải tôn trọng system font scale. Nếu user đã bật font lớn ở system level, app scale thêm:

```dart
MediaQuery(
  data: MediaQuery.of(context).copyWith(
    textScaler: TextScaler.linear(
      MediaQuery.of(context).textScaler.scale(1.0).clamp(1.0, 2.0),
    ),
  ),
  child: child,
)
```

Giới hạn scale tối đa 2.0 để layout không vỡ.

### Test

1. Android: Settings > Display > Font size > đặt Largest
2. iOS: Settings > Accessibility > Display & Text Size > Larger Text > kéo hết cỡ
3. Kiểm tra mọi màn hình không bị overflow, text không bị cắt

## High Contrast Theme

### User theme colors

```dart
// core/theme/user_theme.dart

class UserTheme {
  static const background = Color(0xFF121212);     // Gần đen
  static const surface = Color(0xFF1E1E1E);        // Nền card
  static const onBackground = Color(0xFFFFFFFF);    // Trắng
  static const onSurface = Color(0xFFE0E0E0);      // Xám sáng
  static const primary = Color(0xFF2979FF);         // Xanh dương sáng
  static const error = Color(0xFFFF0000);           // Đỏ (SOS)
  static const success = Color(0xFF00C853);         // Xanh lá (Call)

  // Contrast ratios (WCAG AAA = 7:1 minimum):
  // onBackground/background = 17.4:1
  // onSurface/surface = 12.1:1
  // error/background = 5.3:1 (tăng brightness nếu cần)
}
```

### Contrast checking

Dùng tool WCAG Contrast Checker cho mọi cặp màu foreground/background.

| Cặp màu | Ratio | WCAG Level |
|---|---|---|
| Trắng trên nền đen (#121212) | 17.4:1 | AAA |
| Xám sáng trên nền card (#1E1E1E) | 12.1:1 | AAA |
| Xanh dương (#2979FF) trên nền đen | 5.7:1 | AA Large |
| Đỏ (#FF0000) trên nền đen | 5.3:1 | AA Large |

Tất cả text content phải đạt WCAG AA (4.5:1) trở lên. Text lớn (>=24sp) đạt AA Large (3:1) là đủ.

## Large Touch Targets

### Kích thước tối thiểu

| Element | Kích thước tối thiểu | Ghi chú |
|---|---|---|
| 3 nút chính (SOS, Call, Camera) | 80x80dp | Lớn hơn chuẩn |
| Nút trong cuộc gọi (Mute, Speaker, End) | 64x64dp | Chuẩn AiSee |
| Nút tắt camera | 64x64dp | Chuẩn AiSee |
| Thẻ nhắc thuốc | Full width, >= 64dp cao | Dễ chạm |
| Checkbox uống thuốc | 48x48dp | Android guidelines minimum |

### Khoảng cách giữa các nút

Khoảng cách tối thiểu giữa 2 nút: 16dp. Tránh user chạm nhầm.

### Implementation

```dart
SizedBox(
  width: 80,
  height: 80,
  child: ElevatedButton(
    onPressed: _onTap,
    style: ElevatedButton.styleFrom(
      minimumSize: Size(80, 80),
      padding: EdgeInsets.all(16),
    ),
    child: Text('SOS', style: TextStyle(fontSize: 28)),
  ),
)
```

## Screen Reader Navigation Patterns

### User Home Screen -- thứ tự focus

```
1. Status bar          → "Gemini đang nghe. Sẵn sàng trò chuyện"
2. SOS button          → "Nút khẩn cấp SOS..."
3. Call button         → "Nút gọi điện..."
4. Camera button       → "Nút camera..."
5. Medication card     → "Uống Amlodipine 5mg lúc 8 giờ sáng"
6. Unread badge        → "3 tin nhắn chưa đọc từ Lan"
```

### Semantic grouping

Nhóm các element liên quan để screen reader đọc logic:

```dart
MergeSemantics(
  child: Row(
    children: [
      Icon(Icons.medication),
      Column(
        children: [
          Text('Amlodipine 5mg'),
          Text('8:00 sáng'),
        ],
      ),
      Checkbox(value: taken),
    ],
  ),
)
// Screen reader đọc: "Amlodipine 5mg, 8 giờ sáng, chưa uống, checkbox"
```

### Focus management

Khi có event quan trọng (tin nhắn mới, offline, SOS response), dùng `SemanticsService.announce` để thông báo:

```dart
SemanticsService.announce(
  'Bạn có tin nhắn mới từ Lan',
  TextDirection.ltr,
);
```

### Heading hierarchy

Dùng `Semantics(header: true)` cho tiêu đề section:

```dart
Semantics(
  header: true,
  child: Text('Thuốc hôm nay',
      style: Theme.of(context).textTheme.headlineMedium),
)
```

Screen reader user có thể nhảy giữa các heading để navigate nhanh.
