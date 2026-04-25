# Phân tích chi phí Voice Pipeline — Từ Live API sang STT → LLM → TTS

> **Ngày:** 2026-04-02
> **Tác giả:** Team Đắng
> **Trạng thái:** Nghiên cứu, chưa triển khai

---

## Mục lục

1. [Tóm tắt vấn đề](#1-tóm-tắt-vấn-đề)
2. [Kiến trúc hiện tại vs đề xuất](#2-kiến-trúc-hiện-tại-vs-đề-xuất)
3. [Phân tích chi phí Live API](#3-phân-tích-chi-phí-live-api)
4. [Các model LLM Brain](#4-các-model-llm-brain)
5. [Các giải pháp STT](#5-các-giải-pháp-stt)
6. [Các giải pháp TTS](#6-các-giải-pháp-tts)
7. [5 kịch bản chi phí pipeline](#7-5-kịch-bản-chi-phí-pipeline)
8. [Phân tích theo quy mô người dùng](#8-phân-tích-theo-quy-mô-người-dùng)
9. [Mô hình subscription](#9-mô-hình-subscription)
10. [Khuyến nghị cuối cùng](#10-khuyến-nghị-cuối-cùng)

---

## 1. Tóm tắt vấn đề

### Khó khăn

AiSee hiện dùng **Gemini 3.1 Flash Live API** — model audio-to-audio realtime. Giá:

- Audio input: **$0.005/phút** ($3.00/1M tokens)
- Audio output: **$0.018/phút** ($12.00/1M tokens)

Người dùng AiSee (người lớn tuổi suy giảm thị lực) sử dụng trung bình **lên tới 3 tiếng/ngày**. Dù không active nói 3 tiếng liên tục, nhưng nếu giữ audio stream mở, chi phí tính trên toàn bộ thời gian stream.

Kết quả: **$12.6–$59/user/tháng** — quá đắt để vận hành bền vững với giá subscription phù hợp thị trường Việt Nam.

### Giải pháp

Chuyển từ Live API (audio→audio) sang pipeline tự build: **STT → LLM (text) → TTS**, sử dụng LiveKit Agents framework làm trung gian. Text tokens rẻ hơn audio tokens **10–60 lần**.

### Kết quả mong đợi

| Chỉ số | Live API | Pipeline (tốt nhất) | Pipeline (cân bằng) |
|---|---|---|---|
| Chi phí/user/tháng | $12.6–$59 | $0.54–$0.84 | $3.3–$9.0 |
| Tiết kiệm | — | 93–96% | 49–74% |
| Subscription 99k cover? | ❌ Lỗ nặng | ✅ Lời lớn | ✅/⚠️ Tuỳ kịch bản |

---

## 2. Kiến trúc hiện tại vs đề xuất

### Hiện tại: Gemini Live API (audio-to-audio)

```
User 🎤
  ↓ audio stream
LiveKit SFU
  ↓ audio stream (liên tục)
Gemini Live API (audio→audio, tính phí theo phút stream)
  ↓ audio stream
LiveKit SFU
  ↓
User 🔊

💰 Chi phí = toàn bộ thời gian stream, kể cả im lặng
```

### Đề xuất: Pipeline STT → LLM → TTS

```
User 🎤
  ↓ audio stream
LiveKit SFU
  ↓ audio frames
Agent Worker (Python — đã có)
  ├── 1. Silero VAD (đã có) → phát hiện khi nào user nói
  ├── 2. STT → chuyển giọng nói thành text
  ├── 3. LLM Brain (text API) → xử lý, tool calling, tạo response text
  └── 4. TTS → chuyển text thành giọng nói
  ↓ audio
LiveKit SFU
  ↓
User 🔊

💰 Chi phí = chỉ khi user thực sự nói + LLM xử lý + TTS phát
```

### Ưu nhược điểm

| Tiêu chí | Live API | Pipeline STT→LLM→TTS |
|---|---|---|
| Chi phí | ❌ Rất cao | ✅ Rẻ 10–60x |
| Latency | ✅ ~500ms end-to-end | ⚠️ ~1–2.5 giây (STT+LLM+TTS) |
| Chất lượng giọng nói | ✅ Rất tự nhiên (native audio) | ⚠️ Phụ thuộc TTS engine |
| Độ phức tạp code | ✅ Đơn giản (1 API call) | ⚠️ Phải quản lý 3 service |
| Ngắt lời (interruption) | ✅ Native | ⚠️ Cần tự xử lý |
| Cảm xúc trong giọng | ✅ Nhận biết tone giọng | ❌ Mất khi chuyển qua text |

---

## 3. Phân tích chi phí Live API

### Giá Gemini 3.1 Flash Live

| Loại | Giá/1M tokens | Giá/phút |
|---|---|---|
| Audio input | $3.00 | $0.005 |
| Audio output | $12.00 | $0.018 |
| Text input | $0.75 | — |
| Text output | $4.50 | — |
| Image/video input | $1.00 | $0.002 |

### Chi phí theo mức sử dụng

**Giả định:** user "dùng 3 tiếng/ngày" nhưng thời gian active speech khác nhau tuỳ cách tối ưu.

| Trường hợp | Audio in | Audio out | Image | Tổng/ngày | Tổng/tháng |
|---|---|---|---|---|---|
| **Không tối ưu** (stream 3h liên tục) | 180 phút = $0.90 | 60 phút = $1.08 | 30 phút = $0.06 | **$2.04** | **$61.2** |
| **VAD cơ bản** (cắt khi im lặng 30s) | 60 phút = $0.30 | 20 phút = $0.36 | 15 phút = $0.03 | **$0.69** | **$20.7** |
| **VAD tối ưu** (chỉ gửi khi nói) | 30 phút = $0.15 | 15 phút = $0.27 | 10 phút = $0.02 | **$0.44** | **$13.2** |
| **VAD + idle disconnect** | 20 phút = $0.10 | 10 phút = $0.18 | 5 phút = $0.01 | **$0.29** | **$8.7** |

**Kết luận:** Ngay cả khi tối ưu VAD tối đa, Live API vẫn tốn **$8.7–$13.2/user/tháng**.

### So sánh với Qwen Omni Realtime

| | Gemini 3.1 Flash Live | Qwen3.5-Omni-Plus (Intl) | Qwen3.5-Omni-Plus (China) |
|---|---|---|---|
| Audio in/1M tokens | $3.00 | $4.57 | $2.709 |
| Audio out/1M tokens | $12.00 | $18.13 | $10.766 |
| Ước tính/phút (in+out) | $0.023 | $0.034 | $0.020 |
| VAD | Tự implement | **Built-in** ✅ | **Built-in** ✅ |
| Server gần VN | Trung bình (US) | **Singapore** ✅ | Bắc Kinh |
| Tiếng Việt | **Rất tốt** | Tốt | Tốt |
| Data residency | US | Singapore | ⚠️ China mainland |

**Qwen Omni có built-in VAD** (chỉ tính khi có speech) nhưng **đơn giá đắt hơn Gemini 48%**. Qwen China rẻ hơn 13% nhưng data ở Trung Quốc.

**Kết luận:** Cả hai Live/Realtime API đều đắt. Pipeline là hướng đi đúng.

---

## 4. Các model LLM Brain

### Yêu cầu cho AiSee LLM Brain

1. **Tool/Function calling chính xác** — AiSee có 22 tools (gọi điện, nhắn tin, nhắc thuốc, camera, chỉ đường...)
2. **Tiếng Việt tốt** — hiểu giọng người lớn tuổi, xưng hô đúng (bác, cô, chú...)
3. **TTFT (Time To First Token) thấp** — voice assistant cần phản hồi nhanh, < 500ms lý tưởng
4. **Context đủ lớn** — giữ lịch sử hội thoại dài trong ngày

### Bảng so sánh đầy đủ

#### Nhóm Gemini (Google)

| Model | Input $/1M | Output $/1M | Context | TTFT | Tool Calling | Tiếng Việt | Free Tier |
|---|---|---|---|---|---|---|---|
| Gemini 3.1 Flash-Lite | $0.25 | $1.50 | 1M | ~200ms | Rất tốt | Rất tốt | ✅ Có |
| Gemini 3 Flash | $0.50 | $3.00 | 1M | ~200ms | Tốt nhất | Tốt nhất | ✅ Có |
| **Gemini 2.5 Flash** | **$0.30** | **$2.50** | **1M** | **~200ms** | **Rất tốt** | **Rất tốt** | **✅ Có** |
| Gemini 2.5 Flash-Lite | $0.10 | $0.40 | 1M | ~200ms | Tốt | Tốt | ✅ Có |

#### Nhóm Qwen (Alibaba)

| Model | Input $/1M | Output $/1M | Context | TTFT | Tool Calling | Tiếng Việt | Free Tier |
|---|---|---|---|---|---|---|---|
| **Qwen3.5-Plus** | **$0.26** | **$1.56** | **1M** | **~3 giây** | **72.2 BFCL-V4** | **Tốt** | ❌ |
| **Qwen3.5-Flash** | **$0.065** | **$0.26** | **1M** | **~1–2 giây** | **Tốt** | **Tốt** | ❌ |
| Qwen3-Max | $0.78 | $3.90 | 262K | ~3 giây | Rất tốt | Tốt | ❌ |
| Qwen3-Plus | $0.26 | $1.56 | 1M | ~3 giây | Tốt | Tốt | ❌ |

#### Nhóm khác

| Model | Input $/1M | Output $/1M | Context | TTFT | Tool Calling | Tiếng Việt | Free Tier |
|---|---|---|---|---|---|---|---|
| DeepSeek-V3.2 | $0.28 | $0.42 | 128K | ~400–800ms | Tốt | Tốt | ❌ |
| GLM-4.5-Flash | Free | Free | 128K | ~500ms | Tốt | Trung bình | ✅ Free |
| GPT-4o-mini | $0.15 | $0.60 | 128K | ~300ms | Rất tốt | Tốt | ❌ |
| Claude Haiku 4.5 | $0.80 | $4.00 | 200K | ~300ms | Rất tốt | Tốt | ❌ |
| MiniMax M2.5 | $0.12 | $1.00 | 1M | ~500ms | Tốt | Trung bình | ❌ |

### Đánh giá chi tiết từng ứng viên hàng đầu

#### Gemini 2.5 Flash — ⭐ Ứng viên số 1 cho real-time voice

**Ưu điểm:**
- TTFT ~200ms — nhanh nhất, quan trọng nhất cho voice UX
- Tiếng Việt tốt nhất trong tất cả model (đã proven với Live API hiện tại)
- Tool calling rất tốt, đã hoạt động với 22 tools của AiSee
- Context 1M tokens — đủ cho cả ngày hội thoại
- Có free tier để launch
- Google edge servers ở Singapore/HK — latency thấp từ Việt Nam
- Đã có integration, chỉ cần đổi từ audio API sang text API

**Nhược điểm:**
- Output $2.50/1M — không rẻ nhất
- Phụ thuộc Google (vendor lock-in)

**Chi phí ước tính:** $3.33/user/tháng

#### Qwen3.5-Plus — ⭐ Tool calling tốt nhất, giá tốt

**Ưu điểm:**
- Tool calling 72.2 BFCL-V4 — cao nhất trong nhóm giá này
- Giá $0.26/$1.56 — rẻ hơn Gemini 2.5 Flash ~37%
- Context 1M tokens
- Tiếng Việt tốt (119 ngôn ngữ, SEA focus nhờ SEA-LION training)
- Data storage ở Singapore — tốt cho compliance
- Alibaba Cloud ổn định

**Nhược điểm:**
- ⚠️ **TTFT ~3 giây** — quá chậm cho voice assistant real-time
- Inference không pin ở Singapore, scheduled globally → latency không ổn định
- Không có free tier (international)

**Chi phí ước tính:** $2.34/user/tháng

#### Qwen3.5-Flash — Rẻ nhất có chất lượng

**Ưu điểm:**
- Cực rẻ: $0.065/$0.26 — rẻ hơn Gemini 12x
- Context 1M tokens
- Tool calling đủ tốt

**Nhược điểm:**
- ⚠️ **TTFT ~1–2 giây** — vẫn chậm cho voice
- Kém thông minh hơn Plus, có thể sai tool dispatch
- Không có free tier

**Chi phí ước tính:** $0.54/user/tháng

#### DeepSeek-V3.2 — Rẻ nhất output, nhưng không ổn định

**Ưu điểm:**
- Output chỉ $0.42/1M — rẻ nhất
- Tool calling tốt, OpenAI-compatible SDK
- Tiếng Việt tốt

**Nhược điểm:**
- ⚠️ **TTFT ~400–800ms** — chấp nhận được nhưng không lý tưởng
- Server ở Trung Quốc — latency từ VN không ổn định
- Hay quá tải, response time biến động lớn
- Không có free tier

**Chi phí ước tính:** $0.84/user/tháng

#### GLM-4.5-Flash — Free hoàn toàn

**Ưu điểm:**
- Miễn phí 100%
- Tool calling có hỗ trợ

**Nhược điểm:**
- Rate limit thấp
- Tiếng Việt trung bình
- Không đủ cho production

**Chi phí ước tính:** $0/user/tháng (chỉ phù hợp test/launch nhỏ)

### Ma trận quyết định

```
                    Chất lượng / Tốc độ
                         ▲
                         │
          Gemini 3 Flash │  Gemini 2.5 Flash ★
                         │
              Qwen3.5-Plus    GPT-4o-mini
                         │
          DeepSeek-V3.2  │  Qwen3.5-Flash
                         │
    ───────────────────────────────────────►
    Đắt                                  Rẻ
                         │
              GLM-4.5    │
              (free)     │
                         ▼
                    Chậm / Kém hơn
```

---

## 5. Các giải pháp STT (Speech-to-Text)

### Yêu cầu

1. **Tiếng Việt chính xác** — người lớn tuổi nói chậm, giọng địa phương
2. **Streaming** — transcribe real-time khi user đang nói
3. **Latency thấp** — < 500ms từ khi nói xong tới khi có text
4. **Tích hợp LiveKit** — có SDK hoặc plugin sẵn

### Bảng so sánh

| Giải pháp | Giá | Tiếng Việt | Streaming | Latency | LiveKit Plugin | Ghi chú |
|---|---|---|---|---|---|---|
| **Deepgram Nova-3** | $0.0043/phút | Tốt | ✅ | Thấp | ✅ Có | Dễ tích hợp nhất |
| **Google Cloud STT** | $0.006–0.024/phút | Rất tốt | ✅ | Trung bình | ✅ Có | Tiếng Việt tốt nhất |
| **Whisper large-v3** (self-host) | $0 (cần CPU/GPU) | Tốt | ⚠️ Cần thêm code | Trung bình | ⚠️ Tự build | Miễn phí, cần server |
| **Whisper.cpp** (self-host) | $0 (chạy CPU) | Tốt | ⚠️ | Trung bình | ⚠️ Tự build | Nhẹ, chạy CPU |
| AssemblyAI | $0.015/phút | Hạn chế | ✅ | Thấp | ✅ Có | Tiếng Việt kém |
| Azure Speech | $0.016/phút | Tốt | ✅ | Thấp | ⚠️ | Microsoft |

### Chi phí STT theo mức sử dụng (30 phút active speech/ngày)

| Giải pháp | Chi phí/ngày | Chi phí/tháng |
|---|---|---|
| Whisper self-host | $0 | $0 |
| Deepgram | $0.129 | $3.87 |
| Google Cloud STT | $0.18–0.72 | $5.4–$21.6 |
| AssemblyAI | $0.45 | $13.5 |

### Đánh giá chi tiết

#### Whisper large-v3 (self-host) — Khuyến nghị cho tiết kiệm

- Chạy trên CPU (4 vCPU đủ cho 5–10 concurrent users)
- Hoặc GPU nhỏ (T4) cho 20–50 concurrent users
- Chi phí = chi phí server: ~$30–80/tháng (chia cho nhiều users)
- **Với 100 users: ~$0.3–0.8/user/tháng** — rẻ hơn Deepgram 5–10x
- Cần tự build streaming wrapper (có sẵn thư viện faster-whisper)

#### Deepgram Nova-3 — Khuyến nghị cho dễ triển khai

- Có LiveKit plugin chính thức, plug-and-play
- Tiếng Việt tốt (không bằng Google nhưng đủ dùng)
- $3.87/user/tháng — chấp nhận được
- Không cần quản lý server STT

---

## 6. Các giải pháp TTS (Text-to-Speech)

### Yêu cầu

1. **Tiếng Việt tự nhiên** — giọng ấm, dễ nghe cho người lớn tuổi
2. **Latency thấp** — bắt đầu phát ngay khi có text (streaming TTS)
3. **Giá hợp lý**

### Bảng so sánh

| Giải pháp | Giá | Tiếng Việt | Chất lượng | Streaming | LiveKit Plugin | Ghi chú |
|---|---|---|---|---|---|---|
| **edge-tts** (Microsoft) | **$0** | Tốt | Khá tự nhiên | ✅ | ⚠️ Tự build | Miễn phí, dùng Microsoft Edge voices |
| **Google Cloud TTS** | $4/1M ký tự | Rất tốt | Rất tự nhiên | ✅ | ✅ Có | Nhiều giọng VN (nam/nữ) |
| **Gemini 2.5 Flash TTS** | $0.50 in + $10 out/1M tokens | Tốt | Rất tự nhiên | ✅ | ⚠️ Tự build | Mới, có free tier |
| **Gemini 2.5 Pro TTS** | $1.00 in + $20 out/1M tokens | Tốt | Tự nhiên nhất | ✅ | ⚠️ | Đắt gấp đôi Flash TTS |
| **Viettel AI TTS** | Liên hệ | Rất tốt | Rất tự nhiên | ⚠️ | ❌ | Giọng Việt tốt nhất, khó tích hợp |
| **Piper TTS** (self-host) | $0 | Trung bình | Trung bình | ✅ | ⚠️ | Open-source, cần train voice VN |
| ElevenLabs | $0.30/1k ký tự | ❌ Không có VN | Tốt nhất | ✅ | ✅ | Không hỗ trợ tiếng Việt |

### Chi phí TTS (15 phút AI nói/ngày ≈ ~15,000 ký tự)

| Giải pháp | Chi phí/ngày | Chi phí/tháng |
|---|---|---|
| edge-tts | $0 | $0 |
| Google Cloud TTS | $0.06 | $1.80 |
| Gemini Flash TTS | ~$0.08 | ~$2.40 |
| Piper self-host | $0 | $0 |

### Đánh giá chi tiết

#### edge-tts — Khuyến nghị cho tiết kiệm

- Sử dụng Microsoft Edge online TTS API (không chính thức nhưng ổn định)
- Giọng Việt Nam: `vi-VN-HoaiMyNeural` (nữ), `vi-VN-NamMinhNeural` (nam)
- Chất lượng khá tự nhiên, đủ tốt cho voice assistant
- Hoàn toàn miễn phí
- Hạn chế: không có SLA chính thức, Microsoft có thể thay đổi API

#### Google Cloud TTS — Khuyến nghị cho production

- Giọng Việt tốt nhất trong các managed service
- WaveNet voices rất tự nhiên
- $1.80/user/tháng — chấp nhận được
- Có streaming, có SLA

---

## 7. 5 kịch bản chi phí pipeline

### Giả định chung

- 30 phút active speech/ngày (user nói)
- 15 phút AI response/ngày
- 60 lượt hội thoại/ngày
- ~120k input tokens + ~30k output tokens cho LLM

---

### Kịch bản 1: Rẻ nhất có thể 💰

> *Self-host tất cả, chỉ trả LLM API*

| Khâu | Giải pháp | Chi phí/user/tháng |
|---|---|---|
| STT | Whisper self-host (CPU) | $0 |
| LLM | **Qwen3.5-Flash** ($0.065/$0.26) | $0.54 |
| TTS | edge-tts | $0 |
| **Tổng** | | **$0.54 = ~14,000đ** |

**Ưu điểm:** Cực rẻ, sub 99k lời ~85k/user
**Nhược điểm:** Latency cao (LLM ~1–2s + STT + TTS = ~3–4s tổng). Tool calling có thể kém chính xác. Cần server cho Whisper.

**Phù hợp cho:** Budget tối thiểu, chấp nhận trải nghiệm chậm hơn.

---

### Kịch bản 2: Cân bằng giá — chất lượng ⭐ KHUYẾN NGHỊ

> *Self-host STT/TTS, LLM xịn*

| Khâu | Giải pháp | Chi phí/user/tháng |
|---|---|---|
| STT | Whisper self-host (CPU) | $0 |
| LLM | **Gemini 2.5 Flash** ($0.30/$2.50) | $3.33 |
| TTS | edge-tts | $0 |
| **Tổng** | | **$3.33 = ~86,600đ** |

**Ưu điểm:** LLM nhanh nhất (200ms TTFT), tiếng Việt tốt nhất, tool calling proven. Sub 99k vẫn lời ~13k/user.
**Nhược điểm:** Cần server Whisper. edge-tts không có SLA.

**Phù hợp cho:** Launch production, cân bằng tốt nhất giữa UX và chi phí.

---

### Kịch bản 3: Managed services, giảm ops

> *Dùng dịch vụ managed cho STT/TTS*

| Khâu | Giải pháp | Chi phí/user/tháng |
|---|---|---|
| STT | Deepgram Nova-3 | $3.87 |
| LLM | **Gemini 2.5 Flash** ($0.30/$2.50) | $3.33 |
| TTS | Google Cloud TTS | $1.80 |
| **Tổng** | | **$9.00 = ~234,000đ** |

**Ưu điểm:** Không cần quản lý server STT/TTS. Có SLA, ổn định. Chất lượng cao nhất.
**Nhược điểm:** Sub 99k không cover. Cần sub 249–299k/tháng.

**Phù hợp cho:** Khi có đủ revenue để cover, hoặc B2B pricing.

---

### Kịch bản 4: Qwen làm brain, tiết kiệm hơn nữa

> *Qwen3.5-Plus thay Gemini, vẫn self-host STT/TTS*

| Khâu | Giải pháp | Chi phí/user/tháng |
|---|---|---|
| STT | Whisper self-host | $0 |
| LLM | **Qwen3.5-Plus** ($0.26/$1.56) | $2.34 |
| TTS | edge-tts | $0 |
| **Tổng** | | **$2.34 = ~60,800đ** |

**Ưu điểm:** Rẻ hơn Gemini ~30%. Tool calling rất tốt (BFCL-V4 top). Sub 99k lời ~38k/user.
**Nhược điểm:** ⚠️ **TTFT ~3 giây** — trải nghiệm voice chậm, user phải chờ. Không có free tier.

**Phù hợp cho:** Nếu chấp nhận latency cao hơn để tiết kiệm. Hoặc dùng cho background/non-realtime tasks.

---

### Kịch bản 5: Multi-provider (production scale)

> *Gemini cho realtime, Qwen cho background*

| Khâu | Giải pháp | Chi phí/user/tháng |
|---|---|---|
| STT | Whisper self-host | $0 |
| LLM (realtime voice) | **Gemini 2.5 Flash** | $3.33 |
| LLM (background tasks) | **Qwen3.5-Flash** | $0.10 |
| TTS | edge-tts | $0 |
| **Tổng** | | **$3.43 = ~89,200đ** |

**Background tasks dùng Qwen rẻ:**
- Tóm tắt hội thoại cuối ngày cho family dashboard
- Phân tích xu hướng sức khỏe
- Generate báo cáo thuốc
- Xử lý dữ liệu không cần real-time

**Ưu điểm:** Tối ưu chi phí theo từng use case. Không phụ thuộc 1 provider. Sub 99k vẫn lời.
**Nhược điểm:** Phức tạp hơn trong code (2 LLM providers).

**Phù hợp cho:** Scale 500+ users, cần tối ưu chi phí và giảm vendor lock-in.

---

### Bảng tổng hợp 5 kịch bản

| # | Kịch bản | LLM | Cost/user/tháng | Sub 99k | Latency | Chất lượng |
|---|---|---|---|---|---|---|
| 1 | Rẻ nhất | Qwen3.5-Flash | **14,000đ** ($0.54) | ✅ Lời 85k | ❌ ~3–4s | ⚠️ Trung bình |
| **2** | **Cân bằng** ⭐ | **Gemini 2.5 Flash** | **86,600đ** ($3.33) | **✅ Lời 13k** | **✅ ~1–1.5s** | **✅ Tốt** |
| 3 | Managed | Gemini 2.5 Flash | 234,000đ ($9.00) | ❌ Lỗ | ✅ ~1–1.5s | ✅ Tốt nhất |
| 4 | Qwen brain | Qwen3.5-Plus | 60,800đ ($2.34) | ✅ Lời 38k | ⚠️ ~3–4s | ✅ Tốt |
| 5 | Multi-provider | Gemini + Qwen | 89,200đ ($3.43) | ✅ Lời 10k | ✅ ~1–1.5s | ✅ Tốt |

---

## 8. Phân tích theo quy mô người dùng

### Chi phí server cố định (không tính LLM API)

| Quy mô | VPS Backend | LiveKit | Whisper Server | Database | Tổng server/tháng |
|---|---|---|---|---|---|
| 50 users | $30 (chung) | $30 (chung) | $0 (chung) | $0 (SQLite/local PG) | **$60** |
| 100 users | $60 | $30 | $30 (riêng) | $0 (local PG) | **$120** |
| 500 users | $120 | $60 | $60 (GPU T4) | $50 (managed PG) | **$290** |
| 1,000 users | $160 | $150 | $100 (2x GPU) | $100 (managed PG) | **$510** |

### Tổng chi phí theo kịch bản 2 (khuyến nghị)

| Quy mô | Server/tháng | LLM API/tháng | Tổng/tháng | Per user |
|---|---|---|---|---|
| 50 users | $60 | $167 | **$227** | **$4.53** |
| 100 users | $120 | $333 | **$453** | **$4.53** |
| 500 users | $290 | $1,665 | **$1,955** | **$3.91** |
| 1,000 users | $510 | $3,330 | **$3,840** | **$3.84** |

### Revenue cần thiết (sub 99k/tháng = $3.81)

| Quy mô | Chi phí/tháng | Revenue (100% trả) | Revenue (50% trả) | Lãi/lỗ (50%) |
|---|---|---|---|---|
| 50 users | $227 | $190 | $95 | **-$132** ❌ |
| 100 users | $453 | $381 | $190 | **-$263** ❌ |
| 500 users | $1,955 | $1,905 | $952 | **-$1,003** ❌ |
| 1,000 users | $3,840 | $3,810 | $1,905 | **-$1,935** ❌ |

**⚠️ Nếu chỉ 50% users trả sub 99k → vẫn lỗ ở mọi quy mô.**

### Để break-even cần gì?

| Điều kiện | Chi tiết |
|---|---|
| **Sub 99k, 100% trả** | Break-even từ ~60 users trở lên |
| **Sub 99k, 70% trả** | Break-even từ ~200 users (cần thêm B2B revenue) |
| **Sub 149k** ($5.73) | Break-even từ 50% conversion rate |
| **Sub 99k + B2B $2/user** | Break-even từ 50% conversion + B2B partnership |

---

## 9. Mô hình subscription

### Phân tích tâm lý người trả tiền

**Người trả tiền là con cái (30–50 tuổi)**, không phải user khiếm thị.

| Mức giá | Tâm lý | So sánh đời thường |
|---|---|---|
| 49k/tháng (~$1.9) | "Không đáng suy nghĩ" | Rẻ hơn ly trà sữa |
| **99k/tháng (~$3.8)** | **"Rẻ hơn ly cà phê/ngày"** | **3,300đ/ngày** |
| 149k/tháng (~$5.7) | "Bằng 1 bữa ăn trưa" | Vẫn hợp lý |
| 249k/tháng (~$9.6) | "Cần suy nghĩ" | Đắt hơn Netflix |
| 499k/tháng (~$19) | "Phải hỏi vợ/chồng" | Bằng gói cước điện thoại |

### Đề xuất: 2 gói

#### Gói Cơ bản — 99k/tháng

```
✅ AI trợ lý voice 24/7
✅ Nhắc thuốc tự động (3 loại thuốc)
✅ Gọi điện, nhắn tin trong app
✅ Camera nhận diện (10 lần/ngày)
✅ SOS khẩn cấp
✅ 1 người thân kết nối

"3,300đ/ngày để bố mẹ được an toàn"
```

#### Gói Gia đình — 199k/tháng

```
Tất cả gói Cơ bản, thêm:
✅ Không giới hạn thuốc
✅ Camera không giới hạn
✅ Dashboard sức khỏe chi tiết
✅ Export PDF cho bác sĩ
✅ Tối đa 5 người thân kết nối
✅ Chỉ đường Mapbox
✅ Phát nhạc
✅ Kết nối tình nguyện viên

"6,600đ/ngày — chăm sóc toàn diện"
```

### Unit economics

| Gói | Revenue/user | Cost/user (KBản 2) | Lợi nhuận/user |
|---|---|---|---|
| Cơ bản 99k | $3.81 | $3.33 (ít feature → ít dùng hơn) | **+$0.48** |
| Gia đình 199k | $7.65 | $3.33 | **+$4.32** |

### Nguồn revenue bổ sung

| Kênh | Revenue ước tính | Ghi chú |
|---|---|---|
| **B2B bệnh viện** | $3–5/user/tháng | BV giới thiệu cho bệnh nhân |
| **Đối tác dược phẩm** | $1–3/user/tháng | Nhắc thuốc = kênh marketing tự nhiên |
| **Bảo hiểm sức khỏe** | $2–5/user/tháng | Bundle vào gói bảo hiểm |
| **CSR/Grants** | Lump sum | Google.org, USAID, Quỹ NKT VN |
| **Ads (family interface)** | $0.1–0.3/user/tháng | Bổ sung, không phải revenue chính |

---

## 10. Khuyến nghị cuối cùng

### Kiến trúc đề xuất

```
┌─────────────────────────────────────────────────────┐
│                  Agent Worker (Python)                │
│                                                       │
│  ┌──────────┐   ┌──────────────────┐   ┌──────────┐ │
│  │ Silero   │   │  Gemini 2.5 Flash │   │ edge-tts │ │
│  │ VAD      │──▶│  (text API)       │──▶│ hoặc     │ │
│  │ (có sẵn) │   │  22 tools         │   │ GCloud   │ │
│  └──────────┘   │  $0.30/$2.50/1M   │   │ TTS      │ │
│       │         └──────────────────┘   └──────────┘ │
│       │                │                     │       │
│  ┌──────────┐          │                     │       │
│  │ Whisper  │          │ (background)        │       │
│  │ large-v3 │     ┌──────────────┐           │       │
│  │ (CPU)    │     │ Qwen3.5-Flash│           │       │
│  └──────────┘     │ $0.065/$0.26 │           │       │
│                   └──────────────┘           │       │
└─────────────────────────────────────────────────────┘
```

### Roadmap triển khai

| Giai đoạn | Thời gian | LLM | STT | TTS | Chi phí/user |
|---|---|---|---|---|---|
| **Launch** (< 50 users) | Tháng 1–3 | Gemini free tier | Whisper CPU | edge-tts | **~$0** |
| **Growth** (50–500) | Tháng 3–9 | Gemini 2.5 Flash | Whisper CPU | edge-tts | **$3.33** |
| **Scale** (500+) | Tháng 9+ | Gemini + Qwen fallback | Whisper GPU | Google TTS | **$3.43** |

### Subscription roadmap

| Giai đoạn | Pricing | Target |
|---|---|---|
| **Launch** | Miễn phí (beta) | Thu hút 50 users đầu tiên |
| **Growth** | 99k Cơ bản / 199k Gia đình | Break-even |
| **Scale** | + B2B partnerships | Có lãi |

### Rủi ro và giảm thiểu

| Rủi ro | Xác suất | Giảm thiểu |
|---|---|---|
| Gemini tăng giá | Trung bình | Multi-provider: có Qwen làm fallback |
| edge-tts bị block | Thấp | Chuyển sang Google Cloud TTS ($1.80/user thêm) |
| Whisper không đủ nhanh | Thấp | Chuyển Deepgram ($3.87/user thêm) |
| Latency pipeline quá cao | Trung bình | Tối ưu: streaming STT+TTS, pre-fetch, connection pooling |
| User dùng nhiều hơn dự kiến | Cao | Usage cap: 60 phút AI/ngày cho gói Cơ bản |

---

## Phụ lục: So sánh Live API vs Pipeline (1 bảng)

| | Live API (hiện tại) | Kịch bản 1 (rẻ nhất) | Kịch bản 2 ⭐ | Kịch bản 3 | Kịch bản 4 | Kịch bản 5 |
|---|---|---|---|---|---|---|
| **LLM** | Gemini Live | Qwen3.5-Flash | Gemini 2.5 Flash | Gemini 2.5 Flash | Qwen3.5-Plus | Gemini + Qwen |
| **STT** | Built-in | Whisper | Whisper | Deepgram | Whisper | Whisper |
| **TTS** | Built-in | edge-tts | edge-tts | Google TTS | edge-tts | edge-tts |
| **Cost/user/tháng** | $12.6 | $0.54 | **$3.33** | $9.00 | $2.34 | $3.43 |
| **VND/tháng** | 327,000đ | 14,000đ | **86,600đ** | 234,000đ | 60,800đ | 89,200đ |
| **Latency** | ~500ms | ~3–4s | **~1–1.5s** | ~1–1.5s | ~3–4s | ~1–1.5s |
| **Sub 99k cover?** | ❌ | ✅ | **✅** | ❌ | ✅ | ✅ |
| **Tiết kiệm** | — | 96% | **74%** | 29% | 81% | 73% |
