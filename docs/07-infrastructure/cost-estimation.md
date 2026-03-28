# Ước tính chi phí

## Chi phí MVP (V2 Architecture -- Cloud API)

AiSee V2 sử dụng Gemini Live API (cloud) thay vì self-hosted AI models. Giảm chi phí infrastructure đáng kể.

### Chi tiết từng component

| Component | Spec | Chi phí/tháng | Ghi chú |
|---|---|---|---|
| VPS chính | 2-4 vCPU, 4-8 GB RAM | $30-80 | Go Backend + PostgreSQL + Redis |
| LiveKit Server | 2-4 vCPU, 4 GB RAM (chung hoặc riêng VPS) | $30-50 | WebRTC SFU, có thể chung VPS chính |
| Agent Worker | Python process, ~200 MB RAM | $0 (included) | Chạy chung VPS LiveKit |
| Domain + SSL | .vn domain + Let's Encrypt | $10/năm | Let's Encrypt miễn phí |
| Gemini Live API | Free tier | $0 | 2 RPM, 32k TPM (free) |
| Mapbox | Free tier 25k requests/tháng | $0 | Navigation, geocoding |
| FCM/APNs | Free | $0 | Push notifications |
| Apple Developer | Annual fee | $99/năm | Bắt buộc cho iOS |
| Google Play | One-time fee | $25 (1 lần) | Bắt buộc cho Android |
| UptimeRobot | Free tier | $0 | Monitoring |
| **Tổng** | | **$60-130/tháng** | |

### Free tier limits

| Service | Free tier | Khi nào vượt |
|---|---|---|
| Gemini Live API | 2 requests/phút, 32k tokens/phút | >2 concurrent users gọi Gemini |
| Mapbox | 25,000 requests/tháng | >800 navigation requests/ngày |
| FCM | Unlimited | Không giới hạn |
| UptimeRobot | 50 monitors, 5 phút interval | Đủ cho MVP |

### Khi Gemini free tier không đủ

Nếu vượt 2 RPM:
- **Gemini 2.0 Flash:** $0.10/triệu input tokens, $0.40/triệu output tokens
- Ước tính: 1 session 5 phút = ~3,000 tokens = ~$0.001
- 100 sessions/ngày = $0.10/ngày = $3/tháng

Chi phí Gemini API rất thấp ngay cả khi trả phí.

## So sánh V1 (Self-Hosted AI) vs V2 (Cloud API)

### V1 Architecture (ban đầu)

Self-hosted AI models: PhoWhisper (STT) + LLM + VieNeu-TTS.

| Component | Spec | Chi phí/tháng |
|---|---|---|
| GPU VPS (AI models) | 1x NVIDIA T4/A10, 16+ GB VRAM | $200-500 |
| VPS chính | 4 vCPU, 8 GB RAM | $40-80 |
| LiveKit Server | 2 vCPU, 4 GB RAM | $30-50 |
| Total | | **$270-630/tháng** |

### V2 Architecture (hiện tại)

Cloud Gemini Live API -- không cần GPU.

| Component | Spec | Chi phí/tháng |
|---|---|---|
| VPS chính | 2-4 vCPU, 4-8 GB RAM | $30-80 |
| LiveKit Server | 2-4 vCPU, 4 GB RAM | $30-50 |
| Gemini API | Free tier / pay-as-you-go | $0-10 |
| Total | | **$60-140/tháng** |

### So sánh

| Tiêu chí | V1 (Self-Hosted AI) | V2 (Cloud API) |
|---|---|---|
| Chi phí/tháng | $270-630 | $60-140 |
| Tiết kiệm | -- | **75-80%** |
| GPU cần thiết | Yes (T4/A10) | No |
| Vietnamese quality | Tốt (PhoWhisper) | Rất tốt (Gemini native) |
| Maintenance | Cao (model updates, GPU drivers) | Thấp |
| Latency | Thấp (on-prem) | Thấp (Gemini optimized) |
| Vendor lock-in | Thấp | Trung bình (Gemini) |
| Scalability | Cần thêm GPU | Auto-scale |

## Scaling Costs

### 100 users (MVP target)

| Component | Chi phí/tháng |
|---|---|
| VPS (4 vCPU, 8 GB) | $60 |
| LiveKit (2 vCPU, 4 GB) | $30 |
| Gemini API (giả sử vượt free tier) | $10 |
| Mapbox (trong free tier) | $0 |
| **Total** | **$100/tháng** |

Giả định: peak 20 concurrent users, 50 Gemini sessions/ngày.

### 500 users

| Component | Chi phí/tháng |
|---|---|
| VPS (8 vCPU, 16 GB) | $120 |
| LiveKit (4 vCPU, 8 GB) | $60 |
| Gemini API | $30-50 |
| Mapbox (có thể vượt free tier) | $0-20 |
| **Total** | **$210-250/tháng** |

Giả định: peak 100 concurrent users, 250 sessions/ngày.

### 1,000 users

| Component | Chi phí/tháng |
|---|---|
| VPS cluster (2x 4 vCPU, 8 GB) | $160 |
| LiveKit (8 vCPU, 16 GB hoặc LiveKit Cloud) | $100-200 |
| Gemini API | $50-100 |
| Mapbox | $20-50 |
| Database (managed PostgreSQL) | $50-100 |
| **Total** | **$380-610/tháng** |

Giả định: peak 200 concurrent users, 500 sessions/ngày. Cần managed database và có thể cân nhắc LiveKit Cloud.

### Chi phí per user

| Scale | Chi phí/tháng | Chi phí/user/tháng |
|---|---|---|
| 100 users | $100 | $1.00 |
| 500 users | $230 | $0.46 |
| 1,000 users | $500 | $0.50 |

Chi phí per user giảm khi scale nhờ shared infrastructure. Mức $0.50-1.00/user/tháng rất cạnh tranh so với các dịch vụ tương tự (Be My Eyes Enterprise: $10+/user/tháng).

## Tối ưu chi phí

### Ngắn hạn (MVP)

- Dùng Gemini free tier tối đa
- Chạy tất cả trên 1-2 VPS
- Mapbox free tier 25k requests
- UptimeRobot free monitoring

### Trung hạn (100-500 users)

- Chuyển sang Gemini pay-as-you-go (rất rẻ)
- Tách LiveKit sang VPS riêng
- Cân nhắc reserved instances (giảm 30-50% giá VPS)

### Dài hạn (1,000+ users)

- Managed database (AWS RDS, Google Cloud SQL)
- LiveKit Cloud (giảm ops burden)
- CDN cho static assets
- Cân nhắc hybrid: Gemini cho conversation, self-hosted vision model cho camera (giảm API cost)
