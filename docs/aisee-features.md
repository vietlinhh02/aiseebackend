# AiSee — Tài Liệu Mô Tả Chức Năng

> **Phiên bản:** MVP v1.0.0  
> **Ngày:** Tháng 4, 2026  
> **Đội ngũ:** Team Đắng — VentureX S3 / VinUni ELAB  
> **Slogan:** *"Không chỉ nhìn thay bạn — hành động cùng bạn"*

---

## Mục Lục

1. [Tổng quan sản phẩm](#1-tổng-quan-sản-phẩm)
2. [Hệ thống vai trò](#2-hệ-thống-vai-trò)
3. [Xác thực & bảo mật](#3-xác-thực--bảo-mật)
4. [Trợ lý giọng nói AI](#4-trợ-lý-giọng-nói-ai)
5. [Bộ 20 công cụ AI](#5-bộ-20-công-cụ-ai)
6. [Quản lý sức khỏe](#6-quản-lý-sức-khỏe)
7. [Hệ thống khẩn cấp (3 cấp độ)](#7-hệ-thống-khẩn-cấp-3-cấp-độ)
8. [Tình nguyện viên](#8-tình-nguyện-viên)
9. [Giao tiếp gia đình](#9-giao-tiếp-gia-đình)
10. [Dashboard gia đình](#10-dashboard-gia-đình)
11. [Nhạc & Giải trí](#11-nhạc--giải-trí)
12. [Định vị & Điều hướng](#12-định-vị--điều-hướng)
13. [Thông báo đẩy & Nhắc nhở](#13-thông-báo-đẩy--nhắc-nhở)
14. [Tải tệp lên](#14-tải-tệp-lên)
15. [Mô hình kinh doanh & Monetization](#15-mô-hình-kinh-doanh--monetization)
16. [Pools quảng cáo (Ads Pool)](#16-pools-quảng-cáo-ads-pool)
17. [Bảng giá & giới hạn gói](#17-bảng-giá--giới-hạn-gói)
18. [Phân loại tool calls theo gói](#18-phân-loại-tool-calls-theo-gói)
19. [Luồng phân bổ usage khi chạm giới hạn](#19-luồng-phân-bổ-usage-khi-chạm-giới-hạn)
20. [Kiến trúc hệ thống](#20-kiến-trúc-hệ-thống)
21. [Công nghệ sử dụng](#21-công-nghệ-sử-dụng)
22. [Cơ sở dữ liệu](#22-cơ-sở-dữ-liệu)
23. [API Endpoints](#23-api-endpoints)
24. [Trạng thái hoàn thiện các tính năng](#24-trạng-thái-hoàn-thiện-các-tính-năng)

---

## 1. Tổng quan sản phẩm

**AiSee** là ứng dụng trợ lý AI voice-first dành cho người khiếm thị tại Việt Nam.Ứng dụng kết hợp:

- **Trò chuyện giọng nói thời gian thực** với AI (Gemini Live) qua LiveKit WebRTC
- **Quản lý sức khỏe** (nhắc thuốc, ghi nhận triệu chứng, chỉ số sức khỏe)
- **Kết nối gia đình** (gọi thoại/video, nhắn tin, dashboard giám sát)
- **Hệ thống khẩn cấp 3 cấp độ** (SOS → gọi gia đình → tình nguyện viên)
- **Tình nguyện viên** (người hỗ trợ cộng đồng)

Ứng dụng có 2 vai trò chính: **User** (người khiếm thị) và **Family** (người thân/người chăm sóc).

---

## 2. Hệ thống vai trò

### User (Người khiếm thị)

- Sử dụng trợ lý AI voice-first
- Nhắc thuốc, ghi nhận sức khỏe
- Gọi điện, nhắn tin cho gia đình
- Gửi SOS khẩn cấp
- **Miễn phí hoàn toàn, KHÔNG bao giờ xem quảng cáo**

### Family (Gia đình / Người chăm sóc)

- Xem dashboard giám sát sức khỏe, vị trí, nhật ký hoạt động
- Quản lý thuốc cho người khiếm thị
- Nhắn tin, gọi điện với người khiếm thị
- Đăng ký tình nguyện viên
- **Có thể xem quảng cáo để tài trợ usage cho người thân**

---

## 3. Xác thực & bảo mật

### Đăng nhập

| Phương thức | Mô tả |
|---|---|
| Google Sign-In | Đăng nhập bằng tài khoản Google duy nhất |
| JWT Access Token | Thời hạn 1 giờ |
| JWT Refresh Token | Thời hạn 30 ngày, xoay vòng (rotation) — token cũ bị blacklist qua Redis |

### Chọn vai trò

Sau khi đăng nhập, người dùng chọn vai trò qua 2 slide:

- **Slide 1:** Người dùng (miễn phí, không quảng cáo)
- **Slide 2:** Người thân / Hỗ trợ (có thể có quảng cáo)

### Middleware bảo mật

| Middleware | Mô tả |
|---|---|
| Auth (JWT) | Xác thực Bearer token, kiểm tra role từ Redis |
| InternalAuth | API key nội bộ cho agent worker (so sánh thời gian hằng) |
| FamilyChecker | Kiểm tra liên kết gia đình trước khi truy cập dashboard |
| RequestLog | Ghi nhận mọi request |

---

## 4. Trợ lý giọng nói AI

### Cách hoạt động

1. Người dùng nói **"cần giúp"** → wake word trên thiết bị (TFLite) nhận diện
2. App khởi tạo kết nối LiveKit → Agent Worker (Python) kết nối Gemini Live API
3. Trò chuyện giọng nói thời gian thực, AI phản hồi bằng giọng nói tiếng Việt
4. AI có thể gọi công cụ (tool calls) để thực hiện hành động behalf người dùng

### Trạng thái trợ lý

| Trạng thái | Mô tả |
|---|---|
| Connecting | Đang kết nối LiveKit |
| Initializing | Đang khởi tạo phiên Gemini |
| Idle | Sẵn sàng nghe |
| Listening | Đang nghe người dùng nói |
| Thinking | Đang xử lý câu nói |
| Speaking | Đang phản hồi |
| Disconnected | Mất kết nối |
| Failed | Lỗi, cần thử lại |

### Tính năng trợ lý

- **Phiên 15 phút** — tự động kết nối lại (session resumption)
- **Wake word "cần giúp"** — nhận diện trên thiết biệt, không cần internet
- **Nhận diện tung xưng 99.2%** với ngưỡng tin cậy 0.88
- **Camera AI** — bật camera để AI nhìn và mô tả
- **Phát nhạc** — AI tìm và phát nhạc YouTube
- **Ngữ cảnh người dùng** — AI biết tên, mức thị lực, liên kết gia đình

---

## 5. Bộ 20 công cụ AI

Agent Worker (Python) triển khai 20 function tools cho Gemini gọi:

### Giao tiếp (4 tools)

| Tool | Mô tả |
|---|---|
| `make_voice_call` | Gọi thoại cho người thân theo tên (tìm kiếm mờ tiếng Việt) |
| `make_video_call` | Gọi video cho người thân (tạo phòng LiveKit, gửi VoIP push) |
| `send_chat_message` | Gửi tin nhắn văn bản cho người thân |
| `read_unread_messages` | Đọc tin nhắn chưa đọc (có thể lọc theo liên hệ) |

### Điều khiển thiết bị (2 tools)

| Tool | Mô tả |
|---|---|
| `activate_camera` | Bật camera để AI nhìn và mô tả |
| `deactivate_camera` | Tắt camera |

### Sức khỏe (4 tools)

| Tool | Mô tả |
|---|---|
| `set_medication_reminder` | Tạo nhắc thuốc + lên lịch Redis reminder |
| `list_medication_reminders` | Liệt kê thuốc với trạng thái đã uống/bỏ qua hôm nay |
| `mark_medication_taken` | Đánh dấu đã uống thuốc (tìm kiếm mờ tên tiếng Việt) |
| `get_health_summary` | Tổng hợp triệu chứng, chỉ số, tuân thủ thuốc |

### Khẩn cấp (2 tools)

| Tool | Mô tả |
|---|---|
| `emergency_alert` | SOS broadcast đến TẤT CẢ gia đình + GPS + tạo phòng LiveKit (cooldown 60s) |
| `escalate_emergency` | Leo thang khẩn cấp lên gia đình hoặc tình nguyện viên |

### Tình nguyện viên (3 tools)

| Tool | Mô tả |
|---|---|
| `start_volunteer_match` | Bắt đầu ghép tình nguyện viên (bất đồng bộ qua Redis) |
| `get_match_status` | Kiểm tra trạng thái ghép tình nguyện viên |
| `rate_volunteer` | Đánh giá tình nguyện viên 1–5 sao |

### Vị trí (2 tools)

| Tool | Mô tả |
|---|---|
| `get_location` | Lấy vị trí GPS hiện tại của người dùng |
| `navigate_to` | Mở Google Maps điều hướng đến địa điểm |

### Tiện ích (3 tools)

| Tool | Mô tả |
|---|---|
| `play_music` | Tìm và phát nhạc YouTube qua yt-dlp |
| `get_time` | Lấy thời gian hiện tại |
| `get_weather` | Lấy thông tin thời tiết (placeholder) |

---

## 6. Quản lý sức khỏe

### Thuốc (Medications)

| Chức năng | Mô tả |
|---|---|
| Tạo thuốc | AI hoặc gia đình tạo thuốc (tên, liều lượng, giờ, tần suất) |
| Danh sách thuốc | Liệt kê thuốc với trạng thái đã uống/bỏ qua hôm nay |
| Đánh dấu đã uống | User hoặc AI đánh dấu đã uống thuốc (tìm mờ tên tiếng Việt) |
| Đánh dấu bỏ qua | Đánh dấu bỏ qua uống thuốc |
| Nhắc uống thuốc | Push notification theo lịch: "Đã đến giờ uống [tên] ([liều])" |
| Tự lên lịch lại | Reminder tự lên lịch lại cho ngày hôm sau |

### Triệu chứng (Symptoms)

- Ghi nhận triệu chứng với 10 danh mục tiếng Việt: đau đầu, chóng mặt, mệt mỏi, buồn nôn, khó thở, đau ngực, sốt, đau cơ, mất ngủ, đau khớp
- Mức độ nghiêm trọng (1–10)
- Ghi chú tự do

### Chỉ số sức khỏe (Metrics)

| Chỉ số | Đơn vị mặc định |
|---|---|
| Huyết áp | mmHg |
| Đường huyết | mg/dL |
| Nhiệt độ | °C |
| Cân nặng | kg |
| Nhịp tim | bpm |
| SpO2 | % |

### Lịch hẹn khám (Appointments)

- Tạo lịch hẹn: tiêu đề, vị trí, ngày giờ, ghi chú
- Nhắc nhở kép: 1 giờ trước + 15 phút trước
- Xoá mềm

---

## 7. Hệ thống khẩn cấp (3 cấp độ)

### Cấp 1: SOS Broadcast

- Gửi SOS đến **TẤT CẢ** thành viên gia đình cùng lúc
- Kèm theo vị trí GPS
- Tự động tạo phòng LiveKit (timeout 5 phút)
- Cooldown 60 giây giữa các lần gửi
-family nhận được push notification + VoIP

### Cấp 2: Leo thang Gia đình

- Gọi một người thân cụ thể (theo tên)
- Chờ 30 giây để nhận cuộc gọi
- Nếu không nhận → tự động gọi người tiếp theo
- Có thể bỏ qua (skip) người

### Cấp 3: Ghép Tình nguyện viên

- Khởi tạo ghép tình nguyện viên bất đồng bộ qua Redis
- Tìm tình nguyện viên sẵn sàng, xếp hạng theo đánh giá
- Thông báo tối đa 5 ứng viên, chờ 30 giây mỗi người
- Đánh giá sau cuộc gọi (1–5 sao)

---

## 8. Tình nguyện viên

| Chức năng | Mô tả |
|---|---|
| Đăng ký | Family member đăng ký làm tình nguyện viên (kèm ngôn ngữ hỗ trợ) |
| Bật/tắt sẵn sàng | Chuyển trạng thái available/unavailable |
| Ghép cuộc gọi | Hệ thống tự động ghép khi Family không thể hỗ trợ |
| Đánh giá | Sau cuộc gọi, user đánh giá 1–5 sao |
| Xếp hạng | Điểm trung bình рейтeed, ưu tiên điểm cao khi ghép |

---

## 9. Giao tiếp gia đình

### Liên kết gia đình (Family Links)

- User tạo **mã mời** (8 ký tự hex) kèm mối quan hệ (Con, Bố, Mẹ, Anh, Chị...)
- Family nhập mã mời để tham gia
- Một tài khoản có thể thuộc **nhiều gia đình**
- Xoá mềm liên kết

### Gọi thoại / Video

- Gọi thoại hoặc video qua LiveKit WebRTC
- Tạo phòng + gửi push notification đến người nhận
- Hỗ trợ VoIP push (APNs VoIP, FCM data-only)
- Toggle mic, loa, camera trong cuộc gọi

### Nhắn tin (Chat)

| Chức năng | Mô tả |
|---|---|
| Tin nhắn văn bản | Gửi nhận text |
| Tin nhắn thoại | Ghi âm và gửi voice message |
| Hình ảnh | Chụp ảnh hoặc chọn từ gallery, gửi |
| Phát audio | Phát voice message trong chat |
| Thời gian thực | WebSocket cho tin nhắn tức thì |
| Đọc chưa | Đếm tin nhắn chưa đọc, đánh dấu đã đọc |
| Phân trang | Dựa trên cursor (tải thêm khi cuộn lên) |
| Push notification | Thông báo khi có tin nhắn mới |

---

## 10. Dashboard gia đình

Dashboard cho vai trò Family giám sát người khiếm thị:

### Tab Sức khỏe (Health)

- Tổng hợp thuốc hôm nay + tỷ lệ tuân thủ
- Triệu chứng gần đây
- Chỉ số sức khỏe gần đây

### Tab Thuốc (Medications)

- Danh sách thuốc đầy đủ
- Sửa / xoá thuốc (qua popup menu)
- Thêm thuốc mới (bottom sheet form)

### Tab Nhật ký (Audit Log)

- Lịch sử mọi tương tác AI (gọi điện, nhắn tin, SOS, thay đổi thuốc, v.v.)
- Phân trang

### Tab Vị trí (Location)

- Xem vị trí GPS gần nhất của người khiếm thị
- Bản đồ tĩnh Mapbox (dark theme)
- Liên kết mở Google Maps

---

## 11. Nhạc & Giải trí

- AI tìm kiếm nhạc qua **yt-dlp** (tìm YouTube, trả về URL audio)
- Trả lời tiêu đề bài hát đang phát
- Hỗ trợ đài VOV (Voice of Vietnam)

---

## 12. Định vị & Điều hướng

| Chức năng | Mô tả |
|---|---|
| Cập nhật vị trí | AI hoặc app cập nhật tọa độ GPS |
| Xem vị trí | Family xem vị trí gần nhất trên bản đồ |
| Reverse geocoding | Chuyển tọa độ thành địa chỉ |
| Điều hướng | AI mở Google Maps điều hướng đến địa điểm |

---

## 13. Thông báo đẩy & Nhắc nhở

### Push Notification

| Nền tảng | Cơ chế |
|---|---|
| Android (FCM) | OAuth2 service account, data-only cho VoIP, notification block cho thường |
| iOS (APNs) | Token-based auth (.p8 key), VoIP push type riêng |

### Loại thông báo

- Cuộc gọi đến (VoIP)
- Tin nhắn mới
- Nhắc uống thuốc ("Đã đến giờ uống [tên] ([liều])")
- Nhắc lịch hẹn (1 giờ và 15 phút trước)
- SOS khẩn cấp
- Yêu cầu tình nguyện viên

### Lịch nhắc (Reminder Scheduler)

- Redis sorted sets (`reminders:medications`, `reminders:appointments`)
- Kiểm tra mỗi 30 giây
- Nhắc thuốc tự lên lịch lại cho ngày hôm sau
- Cờ reminded trong DB tránh gửi trùng

---

## 14. Tải tệp lên

- Upload multipart lên MinIO/S3
- Loại chấp nhận: audio (mp3, wav, ogg, aac, m4a, webm), images (jpeg, png, webp)
- Giới hạn **10MB**
- Content-type sniffing, sanitization tên tệp

---

## 15. Mô hình kinh doanh & Monetization

### Mô hình Ads-Subsidized Usage Pool

AiSee áp dụng mô hình **quảng cáo + subscription + usage limits**, thiết kế đặc biệt cho accessibility:

**Nguyên tắc cốt lõi:** Người khiếm thị (User) **KHÔNG BAO GIỜ** phải xem quảng cáo. Gia đình và cộng đồng xem quảng cáo để **tài trợ usage** cho người khiếm thị.

### Giới hạn usage áp dụng cho

- Số phút hội thoại AI (Gemini Live sessions)
- Số lượt gọi tool calls nâng cao (video call, music, navigation)
- Dung lượng lưu trữ (voice messages, hình ảnh)

### Tools luôn miễn phí (không giới hạn)

- SOS khẩn cấp
- Nhắc thuốc & đánh dấu đã uống
- Xem giờ, đọc tin nhắn

---

## 16. Pools quảng cáo (Ads Pool)

### Pool 1: Quảng cáo Gia đình (Family Ads)

| Thuộc tính | Mô tả |
|---|---|
| Ai xem | Thành viên gia đình (vai trò Family) |
| Cơ chế | Mỗi quảng cáo xem = tăng usage cho người thân khiếm thị |
| Ví dụ | Mẹ xem 5 quảng cáo → con được thêm 10 phút hội thoại AI |
| Ưu tiên | Usage từ quảng cáo gia đình cộng VÀO gói hiện tại trước |
| Cảm xúc | Gia đình thấy mình trực tiếp giúp đỡ → engagement cao |

### Pool 2: Quảng cáo Cộng đồng (Supporter Ads Pool)

| Thuộc tính | Mô tả |
|---|---|
| Ai xem | Tình nguyện viên / bất kỳ ai muốn đóng góp |
| Cơ chế | Tất cả ads xem đổ về POOL TỔNG dùng chung |
| Lợi ích | Pool tổng tài trợ usage cho TẤT CẢ người dùng miễn phí |
| Ví dụ | 100 supporters xem 1,000 ads/ngày → Pool cung cấp 2,000 phút AI |
| Gamification | Bảng xếp hạng, huy hiệu, thống kê "Bạn đã giúp X người Y phút hôm nay" |

---

## 17. Bảng giá & giới hạn gói

| Gói | Giá/tháng | Phút AI/tháng | Tool calls nâng cao | Truy cập Pool Cộng đồng | Xem ads tặng usage |
|---|---|---|---|---|---|
| **Free (User)** | 0 | 30 phút (~1 phút/ngày) | 5 lượt/ngày (chỉ cơ bản) | Không | Không (User không xem ads) |
| **Family Basic** | VND 149K (~$5.9) | 150 phút (~5 phút/ngày) | Tất cả tools, 20 lượt/ngày | Có (khi hết gói) | Gia đình xem ads → +bonus |
| **Family Plus** | VND 399K (~$15.8) | 450 phút (~15 phút/ngày) | Tất cả tools, không giới hạn | Có | Gia đình xem ads → +bonus |
| **Family Premium** | VND 599K (~$23.7) | Không giới hạn | Tất cả tools, không giới hạn | Có | Gia đình xem ads → +bonus |
| **B2B Tổ chức** | $10–20/user/tháng | Tuỳ chỉnh | Tuỳ chỉnh, SLA | Pool riêng | Không áp dụng |

---

## 18. Phân loại tool calls theo gói

| Tool | Free | 149K Basic | 399K Plus | 599K Premium |
|---|---|---|---|---|
| SOS khẩn cấp | Luôn miễn phí | Luôn miễn phí | Luôn miễn phí | Luôn miễn phí |
| Nhắc thuốc & đánh dấu đã uống | Luôn miễn phí | Luôn miễn phí | Luôn miễn phí | Luôn miễn phí |
| Xem giờ, đọc tin nhắn | Luôn miễn phí | Luôn miễn phí | Luôn miễn phí | Luôn miễn phí |
| Gọi thoại gia đình | 3 lượt/ngày | Không giới hạn | Không giới hạn | Không giới hạn |
| Gọi video | Không | 5 lượt/ngày | Không giới hạn | Không giới hạn |
| Gửi tin nhắn thoại/ảnh | 5/ngày | 20/ngày | Không giới hạn | Không giới hạn |
| Phát nhạc/radio | 3 bài/ngày | 15 bài/ngày | Không giới hạn | Không giới hạn |
| Điều hướng (navigate_to) | 1/ngày | 10/ngày | Không giới hạn | Không giới hạn |
| Camera AI | Không | 3 lượt/ngày | 10 lượt/ngày | Không giới hạn |
| Tóm tắt sức khỏe nâng cao | Không | Có | Có + báo cáo xu hướng | Có + export PDF |
| Ghép tình nguyện viên | Chỉ SOS | Có | Có + ưu tiên | Có + ưu tiên cao nhất |

---

## 19. Luồng phân bổ usage khi chạm giới hạn

```
User khiếm thị chạm giới hạn gói
│
├─ 1. Kiểm tra usage bonus từ quảng cáo gia đình
│   └─ Có → Dùng trước
│
├─ 2. Kiểm tra gói đăng ký (149K/399K/599K)
│   └─ Còn quota → Tiếp tục dùng
│
├─ 3. Kiểm tra Pool Cộng đồng (Supporter Ads Pool)
│   └─ Có → Tiếp tục dùng từ Pool
│
└─ 4. Hết toàn bộ
    ├─ User trả phí → Luôn có thể dùng từ Pool Cộng đồng (KHÔNG bị khoá hoàn toàn)
    └─ User miễn phí → Bị giới hạn (nhưng vẫn giữ: SOS, nhắc thuốc, đọc tin nhắn)
```

---

## 20. Kiến trúc hệ thống

```
┌─────────────────────────────────────────────────────────┐
│                    Flutter App                         │
│  (User Role / Family Role)                             │
│  Wake word (TFLite) · LiveKit Client · Firebase Push   │
└───────────┬────────────────────────┬───────────────────┘
            │ WebRTC (audio/video)  │ REST/WebSocket
            ▼                       ▼
┌───────────────────┐   ┌───────────────────────────────┐
│  LiveKit Cloud     │   │       Go Backend              │
│  (WebRTC SFU)      │   │  Chi Router · PostgreSQL     │
│                    │   │  Redis · MinIO/S3             │
└───────┬───────────┘   │  FCM · APNs                   │
        │               └───────┬───────────────────────┘
        │                       │ HTTP (X-Internal-Key)
        ▼                       ▼
┌───────────────────┐   ┌───────────────────────────────┐
│  Agent Worker      │◄──│  Tool Calls (17 tools)        │
│  (Python)          │   │  Audit Logging                 │
│  Gemini Live API   │   └───────────────────────────────┘
│  VAD · Session     │
│  Resume             │
└───────────────────┘
```

---

## 21. Công nghệ sử dụng

### Backend

| Thành phần | Công nghệ |
|---|---|
| Ngôn ngữ | Go 1.26 (Chi router) |
| Cơ sở dữ liệu | PostgreSQL 16 |
| ORM/Query | sqlc (code generation) |
| Bộ nhớ đệm | Redis 7 |
| Lưu trữ tệp | MinIO (S3-compatible) |
| Video/Audio | LiveKit Cloud (WebRTC SFU) |
| Push Android | Firebase Cloud Messaging (FCM) |
| Push iOS | Apple Push Notification service (APNs) |
| Xác thực | Google OAuth2 + JWT (HS256) |

### Agent Worker

| Thành phần | Công nghệ |
|---|---|
| Ngôn ngữ | Python 3.12 |
| AI Model | Google Gemini 3.1 Flash Live |
| Framework | LiveKit Agents SDK |
| VAD | Silero Voice Activity Detection |
| Wake Word | DaVoice + TFLite (trên thiết bị) |

### Mobile App

| Thành phần | Công nghệ |
|---|---|
| Framework | Flutter 3.11.3+ (Dart) |
| Xác thực | Google Sign-In |
| Voice/Video | LiveKit Client SDK |
| Push | Firebase Messaging + flutter_callkit_incoming |
| Audio | just_audio, record |
| TTS | flutter_tts |
| Maps | Mapbox (static preview) |
| Storage | flutter_secure_storage, SharedPreferences |

---

## 22. Cơ sở dữ liệu

11 bảng PostgreSQL với UUID primary keys:

| Bảng | Mô tả | Loại xoá |
|---|---|---|
| `users` | Thông tin người dùng (Google ID, email, role, GPS, push tokens) | Cập nhật |
| `family_links` | Liên kết gia đình (mối quan hệ, liên hệ chính) | Xoá mềm |
| `messages` | Tin nhắn text/voice/image | Chỉ thêm |
| `medications` | Định nghĩa thuốc (tên, liều, giờ, tần suất) | Xoá mềm |
| `medication_logs` | Lịch sử uống thuốc | Chỉ thêm |
| `health_symptoms` | Triệu chứng (mô tả, mức độ) | Chỉ thêm |
| `health_metrics` | Chỉ số sức khỏe (huyết áp, đường huyết, v.v.) | Chỉ thêm |
| `appointments` | Lịch hẹn khám (vị trí, thời gian, ghi chú) | Xoá mềm |
| `volunteers` | Hồ sơ tình nguyện viên (ngôn ngữ, đánh giá) | Cập nhật |
| `volunteer_ratings` | Đánh giá tình nguyện viên (1–5 sao) | Chỉ thêm |
| `audit_logs` | Nhật ký mọi AI tool calls (tên tool, params, kết quả) | Chỉ thêm, không xoá |

---

## 23. API Endpoints

### Public

| Method | Path | Mô tả |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `POST` | `/api/v1/auth/google` | Google Sign-In, trả về JWT pair |
| `POST` | `/api/v1/auth/refresh` | Refresh token rotation |

### Authenticated (`/api/v1`, JWT required)

#### Users

| Method | Path | Mô tả |
|---|---|---|
| `GET` | `/users/me` | Lấy profile hiện tại |
| `PUT` | `/users/me` | Cập nhật profile |
| `POST` | `/users/family-link` | Tạo mã mời gia đình |
| `DELETE` | `/users/family-link/{id}` | Xoá liên kết gia đình |
| `POST` | `/users/join-family` | Tham gia gia đình bằng mã mời |
| `GET` | `/users/me/family` | Danh sách thành viên gia đình |

#### Calls

| Method | Path | Mô tả |
|---|---|---|
| `POST` | `/calls/token` | Lấy LiveKit token + dispatch AI agent |
| `POST` | `/calls/create-room` | Tạo phòng gọi thoại/video |
| `POST` | `/calls/end-room` | Kết thúc phòng gọi |

#### Messages

| Method | Path | Mô tả |
|---|---|---|
| `GET` | `/messages/unread-count` | Đếm tin nhắn chưa đọc |
| `POST` | `/messages/send` | Gửi tin nhắn text/voice/image |
| `GET` | `/messages/{contactID}` | Lịch sử chat (phân trang cursor) |
| `PUT` | `/messages/{id}/read` | Đánh dấu đã đọc |
| `WS` | `/ws/chat` | WebSocket nhắn tin thời gian thực |

#### Upload

| Method | Path | Mô tả |
|---|---|---|
| `POST` | `/upload` | Upload audio/image lên S3 (max 10MB) |

#### My Health (User)

| Method | Path | Mô tả |
|---|---|---|
| `GET` | `/me/medications` | Danh sách thuốc + trạng thái hôm nay |
| `GET` | `/me/appointments` | Lịch hẹn sắp tới |
| `GET` | `/me/health` | Tổng hợp sức khỏe |

#### Dashboard (Family)

| Method | Path | Mô tả |
|---|---|---|
| `GET` | `/dashboard/{user_id}/health` | Tổng hợp sức khỏe (cần family link) |
| `GET` | `/dashboard/{user_id}/medications` | Danh sách thuốc |
| `POST` | `/dashboard/{user_id}/medications` | Tạo thuốc cho user |
| `PUT` | `/dashboard/{user_id}/medications/{med_id}` | Sửa thuốc |
| `DELETE` | `/dashboard/{user_id}/medications/{med_id}` | Xoá thuốc |
| `GET` | `/dashboard/{user_id}/audit-log` | Nhật ký hoạt động AI |
| `GET` | `/dashboard/{user_id}/location` | Vị trí GPS gần nhất |

#### Volunteers

| Method | Path | Mô tả |
|---|---|---|
| `GET` | `/volunteers/status` | Trạng thái tình nguyện viên |
| `POST` | `/volunteers/register` | Đăng ký tình nguyện viên |
| `PUT` | `/volunteers/availability` | Bật/tắt sẵn sàng |
| `POST` | `/volunteers/match` | Bắt đầu ghép tình nguyện viên |
| `GET` | `/volunteers/match/{match_id}/status` | Kiểm tra trạng thái ghép |
| `POST` | `/volunteers/rate` | Đánh giá tình nguyện viên |

### Internal (Agent Worker)

| Method | Path | Mô tả |
|---|---|---|
| `POST` | `/api/internal/tool-call` | AI agent gọi tool (cần X-Internal-Key) |

---

## 24. Trạng thái hoàn thiện các tính năng

| Tính năng | Trạng thái | Hoàn thiện | Ghi chú |
|---|---|---|---|
| Đăng nhập Google OAuth | Hoàn thành | 95% | Chưa có đăng nhập OTP điện thoại |
| Trợ lý giọng nói (Gemini Live) | Hoàn thành | 90% | Phiên 15 phút, có session resumption |
| Wake word "cần giúp" | Hoàn thành | 85% | Ngưỡng 0.88, nhận diện 99.2% |
| Chat text/voice/image | Hoàn thành | 90% | WebSocket thời gian thực, phân trang |
| Gọi thoại/video (LiveKit) | Hoàn thành | 85% | Push VoIP iOS, CallKit |
| Nhắc uống thuốc | Hoàn thành | 90% | Push notification + TTS |
| Theo dõi sức khỏe | Hoàn thành | 80% | Triệu chứng, chỉ số, tuân thủ |
| Dashboard gia đình | Hoàn thành | 85% | Sức khỏe, thuốc, vị trí, nhật ký |
| Liên kết gia đình (mã mời) | Hoàn thành | 90% | Xoá mềm, kiểm soát truy cập |
| SOS khẩn cấp | Hoàn thành | 80% | Broadcast 60s cooldown, VoIP push |
| Chia sẻ vị trí (Mapbox) | Hoàn thành | 75% | Ảnh tĩnh, reverse geocode |
| Nhạc/Radio | Hoàn thành | 70% | yt-dlp, đài VOV |
| Ghép tình nguyện viên | Beta | 60% | Bất đồng bộ Redis, đánh giá |
| Điều hướng | Cơ bản | 50% | Chỉ mở Google Maps |
| **Tích hợp thanh toán** | **CHƯA** | **0%** | Chưa có MoMo/ZaloPay/Stripe |
| **Subscription & gói** | **CHƯA** | **0%** | Mô hình đã thiết kế, chưa code |
| **Ads Pool** | **CHƯA** | **0%** | Mô hình đã thiết kế, chưa code |
| Analytics tracking | CHƯA | 0% | Không có SDK analytics |
| Bảng quản trị (Admin) | CHƯA | 0% | Cần cho quản trị |
| Đa ngôn ngữ (i18n) | CHƯA | 0% | Mã cứng tiếng Việt |
| Chế độ ngoại tuyến | CHƯA | 0% | Cần internet cho mọi tính năng |
| Giới hạn tốc độ API | Yếu | — | Chỉ có cho SOS (60s) |

---

*Được tổng hợp từ source code (Go backend, Python agent worker, Flutter app) và báo cáo phân tích dữ liệu toàn diện v1.0.0 (06/04/2026).*