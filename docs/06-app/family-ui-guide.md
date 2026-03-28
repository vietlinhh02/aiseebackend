# Hướng dẫn Family UI (Screen-Based)

## Nguyên tắc thiết kế

Family UI dành cho người thân của user (con cái, vợ chồng, cháu chắt). Sử dụng Material Design chuẩn, không cần accessibility đặc biệt.

| Nguyên tắc | Chi tiết |
|---|---|
| Material Design 3 | Tuân thủ Material Design guidelines |
| Tab navigation | 4 tab chính cho các chức năng |
| Full information | Hiển thị đầy đủ dữ liệu (biểu đồ, lịch sử, chi tiết) |
| Quản lý từ xa | Quản lý thuốc, lịch hẹn, danh bạ cho user |

## 4 Tabs

### Tab 1: Chat

Nhắn tin với user (người thân suy giảm thị lực).

```
┌─────────────────────────────┐
│  Chat  Health  Calls  Settings│
│  ━━━━                        │
│                              │
│  ┌─────────────────────┐    │
│  │ Bố (user)           │    │
│  │ "Con ơi, thuốc bố   │    │
│  │  hết rồi"           │    │
│  │ 🎤 0:05   14:30      │    │
│  └─────────────────────┘    │
│                              │
│  ┌─────────────────────┐    │
│  │           Bạn        │    │
│  │  "Con sẽ mua chiều  │    │
│  │   nay ạ"            │    │
│  │            14:32     │    │
│  └─────────────────────┘    │
│                              │
│  ┌────────────────┐ ┌────┐ │
│  │  Nhập tin nhắn │ │ 🎤 │ │
│  └────────────────┘ └────┘ │
└─────────────────────────────┘
```

Tính năng:
- **Tin nhắn text** -- Gõ và gửi bình thường
- **Tin nhắn giọng nói** -- Nhấn giữ nút mic để ghi âm, thả để gửi. Hiển thị dạng voice bubble với nút play và thời lượng.
- **Trạng thái đã đọc** -- Hiện tick khi user đã đọc (hoặc Gemini đã đọc cho user)
- **Real-time** -- Tin nhắn mới hiện ngay qua WebSocket

### Voice message bubble

```
┌──────────────────────────┐
│ ▶  ━━━━━━━━━━━━━  0:05   │
│ Bố, 14:30                │
└──────────────────────────┘
```

- Nút play/pause
- Waveform hoặc progress bar
- Thời lượng
- Package: `flutter_chat_ui` + `just_audio`

### Tab 2: Health

Dashboard sức khỏe của user.

```
┌─────────────────────────────┐
│  Chat  Health  Calls  Settings│
│         ━━━━━                │
│                              │
│  ┌─ Thuốc hôm nay ────────┐│
│  │ ✅ Amlodipine 5mg  8:00 ││
│  │ ⬜ Metformin 500mg 12:00││
│  │ ⬜ Vitamin D       20:00││
│  └─────────────────────────┘│
│                              │
│  ┌─ Triệu chứng gần đây ─┐│
│  │ 27/03 - Đau đầu (nhẹ)  ││
│  │ 25/03 - Mờ mắt (vừa)   ││
│  │ 23/03 - Chóng mặt (nhẹ)││
│  └─────────────────────────┘│
│                              │
│  ┌─ Huyết áp (7 ngày) ────┐│
│  │  140│    *               ││
│  │  130│  *   *    *        ││
│  │  120│*       *    * *    ││
│  │     └──────────────────  ││
│  │     T2 T3 T4 T5 T6 T7 CN││
│  └─────────────────────────┘│
└─────────────────────────────┘
```

3 phần chính:

#### Lịch uống thuốc (Medication Schedule)

- Danh sách thuốc hôm nay, sắp xếp theo thời gian
- Trạng thái: da uong (checkmark xanh), chua uong (checkbox trống), bo qua (X đỏ)
- Nhấn vào thuốc để xem chi tiết (liều lượng, tần suất, ghi chú)
- Dữ liệu từ `GET /api/v1/dashboard/:user_id/medications`

#### Timeline triệu chứng (Symptom Timeline)

- Danh sách triệu chứng user đã báo qua giọng nói
- Hiển thị: ngày, triệu chứng, mức độ (nhẹ/vừa/nặng)
- Sắp xếp theo thời gian mới nhất
- Dữ liệu từ `GET /api/v1/dashboard/:user_id/health`

#### Biểu đồ chỉ số (Metric Charts)

- Biểu đồ line chart cho các chỉ số sức khỏe
- Chỉ số phổ biến: huyết áp, đường huyết, nhịp tim, cân nặng
- Lọc theo khoảng thời gian: 7 ngày, 30 ngày, 3 tháng
- User báo chỉ số qua giọng nói ("Huyết áp hôm nay 130/80"), Gemini log vào hệ thống

### Tab 3: Calls

```
┌─────────────────────────────┐
│  Chat  Health  Calls  Settings│
│                ━━━━━         │
│                              │
│  ┌──────────────────────┐   │
│  │  📞  GỌI CHO BỐ     │   │
│  │      (nút lớn)       │   │
│  └──────────────────────┘   │
│                              │
│  Lịch sử cuộc gọi           │
│  ┌──────────────────────┐   │
│  │ 📞 Bố  14:30  5 phút │   │
│  │ 📞 Bố  Hôm qua 3 phút│   │
│  │ ❌ Bố  26/03  Nhỡ     │   │
│  └──────────────────────┘   │
└─────────────────────────────┘
```

- **Nút gọi lớn** -- Gọi VoIP cho user qua LiveKit
- **Lịch sử cuộc gọi** -- Danh sách cuộc gọi, thời lượng, trạng thái (thành công/nhỡ)
- Cuộc gọi đi qua LiveKit, hiện native call UI trên điện thoại của user

### Tab 4: Settings

```
┌─────────────────────────────┐
│  Chat  Health  Calls  Settings│
│                       ━━━━━━ │
│                              │
│  Quản lý cho: Bố            │
│                              │
│  ┌─ Thuốc ─────────────┐   │
│  │ Amlodipine 5mg       │   │
│  │  8:00, hàng ngày     │   │
│  │ Metformin 500mg      │   │
│  │  12:00, 20:00        │   │
│  │        [+ Thêm thuốc]│   │
│  └──────────────────────┘   │
│                              │
│  ┌─ Lịch hẹn ──────────┐   │
│  │ BS. Nguyễn           │   │
│  │  01/04, 9:00         │   │
│  │        [+ Thêm hẹn]  │   │
│  └──────────────────────┘   │
│                              │
│  ┌─ Liên hệ ───────────┐   │
│  │ Lan (con gái)  Primary│   │
│  │ Minh (con trai)       │   │
│  │    [+ Mời người thân] │   │
│  └──────────────────────┘   │
│                              │
│  [Đăng xuất]                 │
└─────────────────────────────┘
```

## Quản lý thuốc cho user

Người thân có quyền quản lý thuốc và lịch hẹn thay user. Flow:

### Thêm thuốc mới

```
1. Vào Settings > Thuốc > [+ Thêm thuốc]
2. Nhập: tên thuốc, liều lượng, thời gian uống, tần suất
3. Bấm Lưu
4. Backend lưu medication + tạo local notification trên app user
5. Gemini sẽ nhắc user uống thuốc đúng giờ
```

### Chỉnh sửa/xóa thuốc

- Swipe trái trên thuốc để xóa (soft delete)
- Nhấn vào thuốc để chỉnh sửa chi tiết
- Thay đổi sync tới app user ngay lập tức

### Xem lịch sử uống thuốc

- Nhấn vào thuốc > "Lịch sử" để xem log
- Hiển thị: ngày uống, giờ uống, bỏ qua
- Giúp người thân theo dõi user có uống thuốc đều không

### Authorization

Tất cả thao tác quản lý đều kiểm tra `family_links` table:
- Người thân phải có relationship với user
- Endpoint `GET/POST/PUT/DELETE /api/v1/dashboard/:user_id/*` verify JWT + family link
- Không có link -> 403 Forbidden
