# Phụ thuộc Google (Google Dependency)

## Tổng quan

AiSee có chủ đích giảm thiểu phụ thuộc Google. Chỉ giữ lại những dịch vụ thực sự
cần thiết và không có thay thế hợp lý. Tài liệu này liệt kê cụ thể từng điểm phụ
thuộc, mức độ rủi ro, và kế hoạch xử lý.

## Những gì đang dùng Google

| Dịch vụ Google | Dùng cho | Bắt buộc? | Mức độ phụ thuộc |
|---------------|---------|----------|-----------------|
| Gemini Live API | AI voice conversation + vision | Có -- sản phẩm cốt lõi | **Cao** |
| Google OAuth2 | Đăng nhập | Không -- có thể thêm cách khác | Trung bình |
| FCM HTTP API | Push notification Android | Có -- Android yêu cầu | Thấp |
| Google Play Services | Chạy trên Android | Có -- platform yêu cầu | Thấp |
| Google Search (qua Gemini) | Tìm kiếm thông tin | Không -- có thể thay thế | Thấp |

## Những gì đã loại bỏ và tại sao

### Firebase SDK --> Loại bỏ hoàn toàn

```
TRƯỚC:                              SAU:
Firebase Auth                       Google OAuth2 trực tiếp
Firebase Cloud Messaging SDK        APNs (iOS) + FCM HTTP API (Android)
Firebase Crashlytics                Docker logs + UptimeRobot
Firebase Analytics                  Không dùng analytics (MVP)
Firebase Realtime Database          PostgreSQL + Redis
```

**Tại sao loại bỏ:**
- Firebase SDK thêm ~5-10MB vào app size
- Firebase Auth phức tạp hơn cần thiết (chỉ cần Google login)
- Firebase SDK tạo phụ thuộc sâu vào Google ecosystem
- Mỗi tính năng Firebase đều có thay thế đơn giản hơn

**Kết quả:** App không import bất kỳ Firebase package nào. Push notification gửi
trực tiếp qua APNs (iOS) và FCM HTTP v1 API (Android, chỉ từ backend).

### Google Maps --> Thay bằng Mapbox

```
TRƯỚC:                              SAU:
google_maps_flutter package         mapbox_maps_flutter package
Google Maps Platform API key        Mapbox access token
$7/1000 requests sau free tier      Free tier 25,000 requests/tháng
```

**Tại sao thay:**
- Mapbox free tier lớn hơn (25k req/tháng vs Google 28k nhưng tính phí cao hơn sau đó)
- Bớt 1 Google dependency
- Mapbox có navigation SDK tốt cho accessibility
- Không cần Google Maps API key (giảm mặt quản lý)

### Google ML Kit --> Thay bằng Gemini Vision

```
TRƯỚC:                              SAU:
ML Kit Text Recognition (on-device) Gemini Live API (qua camera on-demand)
ML Kit Object Detection             Gemini nhận diện vật thể qua video frame
ML Kit Barcode Scanning             Gemini đọc mã vạch qua camera
```

**Tại sao thay:**
- Gemini đã có khả năng nhìn và mô tả -- không cần SDK riêng
- Giảm app size (ML Kit models ~20-50MB)
- Một giao diện thống nhất: user nói, Gemini nghe và nhìn
- Nhược điểm: cần internet (ML Kit chạy offline). Chấp nhận vì AiSee cần internet cho voice

## Phân tích rủi ro từng dịch vụ

### 1. Gemini Live API -- Rủi ro CAO

**Đang dùng:** Core AI conversation (voice + vision). Toàn bộ trải nghiệm người
dùng phụ thuộc vào Gemini.

**Rủi ro cụ thể:**

| Rủi ro | Xác suất | Tác động |
|--------|---------|---------|
| Free tier bị giới hạn hoặc xóa | Trung bình | Cao -- app không chạy được |
| Chất lượng tiếng Việt không đủ tốt | Trung bình | Cao -- user không hiểu/bị hiểu nhầm |
| Gemini bảo trì / downtime | Thấp | Cao -- app không có AI |
| Google thay đổi API / ngừng model | Thấp | Cao -- cần update Agent Worker |
| Gemini hallucination (thông tin sai) | Cao | Trung bình -- có confirmation flow |

**Kế hoạch giảm thiểu:**

```
1. KIẾN TRÚC CHO PHÉP THAY THẾ
   LiveKit Agent framework hỗ trợ nhiều Realtime Model:
   - Google Gemini (đang dùng)
   - OpenAI Realtime API (thay thế số 1)
   - Các model khác qua LiveKit plugin system

   Đổi code khi chuyển: chỉ thay đổi ~10 dòng trong Agent Worker
   (đổi RealtimeModel và system prompt)

2. VALIDATE SỚM (Phase 0)
   Trước khi commit dự án, test:
   - Gemini có nghe/nói tiếng Việt tốt không?
   - Delay có chấp nhận được không? (<1 giây)
   - Tool calling có ổn định không?
   Nếu không đạt --> chuyển sang OpenAI Realtime hoặc STT-->LLM-->TTS pipeline

3. FALLBACK KHI GEMINI DOWN
   - Agent Worker phát hiện lỗi --> thông báo user: "AI tạm thời không khả dụng"
   - Các tính năng không cần AI vẫn chạy: nhắc thuốc (local notif), gọi điện
     (bấm nút), SOS (bấm nút), nghe nhạc
   - Family Dashboard vẫn hoạt động bình thường

4. THEO DÕI GOOGLE ANNOUNCEMENTS
   - Đăng ký nhận thông báo từ Google AI blog
   - Khi có thay đổi API --> update Agent Worker
```

### 2. Google OAuth2 -- Rủi ro THẤP

**Đang dùng:** Đăng nhập duy nhất.

**Rủi ro:**

| Rủi ro | Xác suất | Tác động |
|--------|---------|---------|
| Google thay đổi OAuth2 flow | Rất thấp | Thấp -- OAuth2 là standard |
| User không có Google account | Thấp | Trung bình -- không đăng nhập được |
| Google khóa tài khoản user | Rất thấp | Trung bình -- mất truy cập |

**Kế hoạch giảm thiểu:**

```
- OAuth2 là open standard, rất ổn định, thay đổi rất ít
- Post-MVP: thêm đăng nhập bằng email/password hoặc số điện thoại (OTP)
  cho user không có Google account
- Database lưu google_id nhưng không phụ thuộc vào nó để query
  (dùng UUID làm primary key)
```

### 3. FCM HTTP API (Android Push) -- Rủi ro THẤP

**Đang dùng:** Gửi push notification cho Android (gọi đến, tin nhắn, SOS).

**Rủi ro:**

| Rủi ro | Xác suất | Tác động |
|--------|---------|---------|
| FCM down | Rất thấp | Trung bình -- Android không nhận push |
| Google thay đổi FCM API | Thấp | Thấp -- HTTP v1 API ổn định |
| FCM bị tính phí | Rất thấp | Thấp -- chỉ gửi ít push |

**Kế hoạch giảm thiểu:**

```
- FCM là cách DUY NHẤT gửi push trên Android (không có thay thế)
- Dùng FCM HTTP v1 API (không dùng SDK) --> phụ thuộc tối thiểu
- Backend gửi trực tiếp, không cần Firebase SDK trong app
- iOS dùng APNs riêng (không phụ thuộc Google)
```

### 4. Google Play Services -- Rủi ro THẤP

**Đang dùng:** Android runtime requirement (Google Sign-In, FCM).

**Rủi ro:**

| Rủi ro | Xác suất | Tác động |
|--------|---------|---------|
| User không có Play Services (Huawei) | Thấp (ở VN) | Cao -- app không chạy |

**Kế hoạch giảm thiểu:**

```
- Target user: người dùng tại VN, hầu hết dùng điện thoại có Play Services
- Huawei (không có Play Services): ~5% thị phần tại VN
- Post-MVP: xem xét hỗ trợ Huawei Push Kit nếu có nhu cầu
```

### 5. Google Search (qua Gemini) -- Rủi ro THẤP

**Đang dùng:** Gemini dùng GoogleSearch provider tool để tìm kiếm thông tin.

**Rủi ro:**

| Rủi ro | Xác suất | Tác động |
|--------|---------|---------|
| Google chặn search API | Thấp | Thấp -- chỉ mất tính năng tìm kiếm |

**Kế hoạch giảm thiểu:**

```
- Tìm kiếm là tính năng phụ, không phải core
- Gemini có thể trả lời nhiều câu hỏi từ kiến thức có sẵn
- Thay thế: Bing Search API, DuckDuckGo, hoặc các search provider khác
```

## Tóm tắt: Ma trận phụ thuộc

```
+---------------------+-----------+------------+------------------+
| Dịch vụ             | Mức độ    | Thay thế   | Độ khó chuyển    |
|                     | phụ thuộc | được không?| đổi              |
+---------------------+-----------+------------+------------------+
| Gemini Live API     | CAO       | Có         | Thấp (~10 LOC    |
|                     |           | (OpenAI,   | trong Agent      |
|                     |           | etc.)      | Worker)          |
+---------------------+-----------+------------+------------------+
| Google OAuth2       | TRUNG BÌNH| Có         | Trung bình       |
|                     |           | (email/    | (thêm auth       |
|                     |           | phone)     | method)          |
+---------------------+-----------+------------+------------------+
| FCM HTTP API        | THẤP      | Không      | N/A (Android     |
| (Android only)      |           | (bắt buộc) | bắt buộc)        |
+---------------------+-----------+------------+------------------+
| Google Play Services| THẤP      | Không      | N/A (platform)   |
+---------------------+-----------+------------+------------------+
| Google Search       | THẤP      | Có         | Thấp (đổi        |
| (qua Gemini)        |           | (Bing,..)  | provider tool)   |
+---------------------+-----------+------------+------------------+
```

## Nguyên tắc thiết kế chống phụ thuộc

```
1. KHÔNG DÙNG FIREBASE SDK
   Mỗi tính năng Firebase đều có thay thế đơn giản hơn.
   Push gửi trực tiếp qua APNs/FCM HTTP API.

2. AGENT WORKER LÀ LỚP TRỪU TƯỢNG
   Gemini chạy qua LiveKit plugin system.
   Chuyển model = đổi 1 file Python (~10 dòng).
   Business logic KHÔNG nằm trong Agent --> không mất khi đổi model.

3. AUTH TÁCH BIỆT KHỎI GOOGLE
   Database dùng UUID làm primary key, không dùng google_id.
   google_id chỉ là cách liên kết tài khoản.
   Thêm auth method = thêm 1 endpoint, không ảnh hưởng hệ thống.

4. MAPS ĐÃ TÁCH
   Mapbox thay Google Maps. Không còn phụ thuộc.

5. VISION ĐÃ TÁCH
   Gemini Vision thay ML Kit. Nếu đổi model AI, vision cũng đổi theo.
   Không có SDK riêng cho vision.
```
