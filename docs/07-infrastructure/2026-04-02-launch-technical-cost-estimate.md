# Chi phí kỹ thuật dự kiến cho giai đoạn launch 3-6 tháng

## Phạm vi

- Chỉ tính chi phí kỹ thuật vận hành
- Không tính nhân sự kỹ thuật
- Đơn vị trình bày: VND
- Kịch bản áp dụng: launch MVP và vận hành 3-6 tháng đầu

## Giả định tính toán

- Tỷ giá quy đổi tạm tính: 1 USD = 26.000 VND
- Kiến trúc hiện tại theo repo: Go backend + PostgreSQL + Redis + LiveKit + Agent Worker + Gemini Live API
- Gemini, Mapbox, FCM/APNs, UptimeRobot ưu tiên dùng free tier trong giai đoạn launch
- Kịch bản `Tiết kiệm`: gom dịch vụ tối thiểu, ưu tiên chạy chung server nếu tải cho phép
- Kịch bản `An toàn`: tách riêng các thành phần quan trọng hơn để giảm rủi ro khi launch

## Tóm tắt ngân sách đề xuất

| Hạng mục | Tiết kiệm | An toàn |
|---|---:|---:|
| Chi phí one-time | 3.480.000 | 3.480.000 |
| Chi phí hàng tháng | 1.560.000 | 3.380.000 |
| Tổng 3 tháng | 8.160.000 | 13.620.000 |
| Tổng 6 tháng | 12.840.000 | 23.760.000 |

## 1. Chi phí one-time

| Hạng mục | Dự kiến | Ghi chú |
|---|---:|---|
| Apple Developer Program | 2.574.000 | ~99 USD/năm, bắt buộc nếu phát hành iOS |
| Google Play Console | 650.000 | ~25 USD, đóng 1 lần |
| Domain | 256.000 | Theo estimate hiện có trong repo |
| SSL | 0 | Sử dụng Let's Encrypt |
| **Tổng one-time** | **3.480.000** | |

## 2. Chi phí hàng tháng

### Kịch bản 1: Tiết kiệm

| Hạng mục | Dự kiến/tháng | Ghi chú |
|---|---:|---|
| VPS chính + LiveKit dùng chung | 1.300.000 | 1 máy 2-4 vCPU, 4-8 GB RAM |
| Agent Worker | 0 | Chạy cùng máy LiveKit/VPS chính |
| Gemini Live API | 0 | Mức launch cố gắng nằm trong free tier |
| Mapbox | 0 | Nằm trong free tier |
| Push notification | 0 | FCM/APNs cơ bản không tính phí |
| Monitoring cơ bản | 0 | UptimeRobot free |
| Backup/logs/phát sinh nhỏ | 260.000 | Vùng dự phòng kỹ thuật cơ bản |
| **Tổng/tháng** | **1.560.000** | |

### Kịch bản 2: An toàn

| Hạng mục | Dự kiến/tháng | Ghi chú |
|---|---:|---|
| VPS backend | 1.560.000 | 1 máy riêng cho backend + database |
| LiveKit server | 1.040.000 | 1 máy riêng cho WebRTC |
| Agent Worker | 0 | Vẫn có thể chạy chung máy LiveKit |
| Gemini Live API | 260.000 | Dự phòng đã vượt free tier nhẹ |
| Monitoring/logging | 260.000 | Gói cơ bản hoặc dịch vụ thay thế free tier |
| Backup/phát sinh kỹ thuật | 260.000 | Margin cho traffic, disk, network |
| **Tổng/tháng** | **3.380.000** | |

## 3. Tổng chi phí theo thời gian

### Tổng 3 tháng

| Kịch bản | One-time | Chi phí 3 tháng | Tổng |
|---|---:|---:|---:|
| Tiết kiệm | 3.480.000 | 4.680.000 | 8.160.000 |
| An toàn | 3.480.000 | 10.140.000 | 13.620.000 |

### Tổng 6 tháng

| Kịch bản | One-time | Chi phí 6 tháng | Tổng |
|---|---:|---:|---:|
| Tiết kiệm | 3.480.000 | 9.360.000 | 12.840.000 |
| An toàn | 3.480.000 | 20.280.000 | 23.760.000 |

## 4. Khuyến nghị sử dụng

- Nếu cần bản đề xuất ngân sách gọn để chốt nhanh: dùng mốc `8,2 triệu / 3 tháng` theo kịch bản `Tiết kiệm`
- Nếu cần mức an toàn hơn để launch thật: dùng mốc `13,6 triệu / 3 tháng`
- Nếu dự án có khả năng tăng số cuộc gọi AI sớm: ưu tiên chốt ngân sách theo kịch bản `An toàn`

## 5. Ghi chú quan trọng

- Đây là bảng dự kiến để lập ngân sách, chưa phải báo giá mua dịch vụ thực tế
- Chi phí Gemini API có thể tăng nếu số session voice/video tăng nhanh sau launch
- Nếu buộc phải tách database managed sớm, tổng chi phí 6 tháng sẽ tăng thêm
- Nếu chưa phát hành iOS ngay từ đầu, có thể tạm hoãn khoản Apple Developer để giảm chi phí one-time
