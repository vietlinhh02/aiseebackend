# Mô hình kinh doanh AiSee

> **Lưu ý:** Đây là đề xuất ban đầu, chưa được xác nhận. Cần nghiên cứu thêm về ads platform phù hợp.

---

## Nguyên tắc cốt lõi

**Người suy giảm thị lực dùng miễn phí, không quảng cáo.** Chi phí AI và hạ tầng rất cao -- mô hình thu phí 49k/tháng không đủ cover. Thay vào đó, monetize qua quảng cáo trên giao diện người thân (người nhìn được).

---

## Hai nhóm người dùng, hai mô hình

### 1. Người suy giảm thị lực (user)

**Giá: MIỄN PHÍ hoàn toàn**

- Tất cả 20 tính năng AI
- Hỏi AI không giới hạn
- Gọi điện, gọi video, nhắn tin không giới hạn
- Nhắc thuốc, ghi chỉ số sức khỏe
- Mở bản đồ Mapbox để chỉ đường
- Phát nhạc
- Kết nối tình nguyện viên
- **Không có quảng cáo** -- người khiếm thị không thể tương tác với ads, và ads gây phiền trong trải nghiệm voice-first
- Camera nhìn giúp (cần internet -- sử dụng Gemini AI)

### 2. Người thân / người hỗ trợ (family)

**Giá: MIỄN PHÍ, có quảng cáo có chủ đích (targeted ads)**

- Tất cả tính năng giao diện gia đình
- Theo dõi thuốc, sức khỏe bố/mẹ
- Nhắn tin và gọi điện qua ứng dụng
- Quản lý danh sách thuốc và lịch khám
- Nhận thông báo khẩn cấp
- **Hiển thị quảng cáo có chủ đích** trên giao diện gia đình

---

## Tại sao mô hình này?

### Chi phí AI và hạ tầng cao

- Gemini API, LiveKit, VPS -- chi phí vận hành $60-130/tháng cho MVP
- Khi scale lên 500-1000 users: $200-500/tháng
- Mô hình subscription 49k/tháng (~$2) cho người có thu nhập thấp không đủ cover

### Người khiếm thị cần được ưu tiên

- Lương hưu trung bình ở Việt Nam khoảng 3-5 triệu VND/tháng
- Người khiếm thị thường có thu nhập thấp hơn trung bình
- Bất kỳ rào cản chi phí nào cũng giảm số người được hỗ trợ
- AiSee đặt mục tiêu xã hội lên hàng đầu

### Quảng cáo trên giao diện family là hợp lý

- Người thân (con cái, 30-50 tuổi) nhìn được, tương tác được với ads
- Giao diện family là UI chuẩn (tab-based, Material Design) -- ads tích hợp tự nhiên
- Targeted ads (sản phẩm sức khỏe, dịch vụ chăm sóc người cao tuổi, dược phẩm) phù hợp context
- Không ảnh hưởng trải nghiệm voice-first của người dùng chính

---

## So sánh với sản phẩm ngoại

| Sản phẩm | Giá | Ghi chú |
|---|---|---|
| AiSee (user) | Miễn phí | Đầy đủ tính năng, không ads |
| AiSee (family) | Miễn phí + ads | Ads trên giao diện gia đình |
| Be My Eyes | Miễn phí | Không có quản lý sức khỏe, không agentic |
| OrCam | ~110 triệu VND | Thiết bị riêng |
| Envision | ~17-85 triệu VND | Thiết bị + đăng ký hàng tháng |

---

## Cần nghiên cứu thêm

- **Ads platform phù hợp:** Google AdMob, Meta Audience Network, hoặc ads trực tiếp từ đối tác dược phẩm/sức khỏe?
- **Revenue per user:** Ước tính CPM/CPC cho targeted ads ở Việt Nam trong nhóm 30-50 tuổi?
- **Đối tác tiềm năng:** Các thương hiệu dược phẩm, bảo hiểm sức khỏe, dịch vụ chăm sóc người cao tuổi có muốn quảng cáo trên AiSee không?
- **Mô hình hybrid:** Có nên cung cấp gói premium cho family (không ads) với giá hợp lý không?
- **Tài trợ:** Có thể xin tài trợ từ quỹ hỗ trợ người khuyết tật, NGO, hoặc chương trình CSR của doanh nghiệp không?

---

## Cách đăng ký

1. Tải ứng dụng AiSee (miễn phí)
2. Chọn vai trò: "Tôi dùng cho mình" hoặc "Tôi chăm sóc người thân"
3. Đăng nhập bằng Google
4. Bắt đầu sử dụng ngay -- không cần thanh toán

Con cái có thể cài app và đăng ký giúp bố mẹ.
