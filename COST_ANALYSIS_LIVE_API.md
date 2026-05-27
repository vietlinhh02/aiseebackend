# Phân Tích Chi Phí Live/Realtime API

> Nguồn: [Google AI Pricing](https://ai.google.dev/gemini-api/docs/pricing), [OpenAI Pricing](https://platform.openai.com/docs/pricing)
> Cập nhật: 05/2026

---

## 1. Giá Gốc (USD / 1M tokens)

| Model | Text Input | Audio Input | Text Output | Audio Output |
|---|---|---|---|---|
| **Gemini 3.1 Flash Live Preview** (đang dùng) | $0.75 | $3.00 | $4.50 | $12.00 |
| Gemini 2.5 Flash Native Audio (Live API) | $0.50 | $3.00 | $2.00 | $12.00 |
| OpenAI gpt-realtime-2 | $4.00 | $32.00 | $24.00 | $64.00 |
| OpenAI gpt-realtime (1.5) | $4.00 | $32.00 | $16.00 | $64.00 |

| Model | Input ảnh/video | Cache Text | Cache Audio | Variant rẻ nhất |
|---|---|---|---|---|
| **Gemini 3.1 Flash Live Preview** | $1.00 | — | — | — |
| Gemini 2.5 Flash Native Audio | $3.00 | — | — | — |
| OpenAI gpt-realtime-2 | $5.00 | $0.40 | $0.40 | — |
| OpenAI gpt-realtime (1.5) | $5.00 | $0.40 | $0.40 | — |

---

## 2. Ước Lượng Chi Phí / Phút (Audio-to-Audio)

Giả định: 1 phút đàm thoại (~40% user nói, ~60% bot nói)

| Model | Audio In (¢/min) | Audio Out (¢/min) | Total Audio (¢/min) | System Prompt (¢/min) | **Tổng (¢/min)** |
|---|---|---|---|---|---|
| **Gemini 3.1 Flash Live Preview** | 0.50* | 1.80* | 2.30* | negligible | **~2.3** |
| Gemini 2.5 Flash Native Audio | ~0.43 | ~2.16 | ~2.59 | negligible | **~2.6** |
| OpenAI gpt-realtime-2 (no cache) | ~1.92 | ~7.68 | ~9.60 | ~5-10 | **~15-20** |
| OpenAI gpt-realtime-2 (cache) | ~1.92 | ~7.68 | ~9.60 | ~0.5-1 | **~10-11** |

> *Gemini 3.1 Flash Live Preview có sẵn giá per-minute: Audio Input $0.005/min, Audio Output $0.018/min
>
> OpenAI: 1 token = 100ms input audio, 50ms output audio → 600 input token + 1200 output token / min

---

## 3. So Sánh Trực Quan

```
Model đang dùng: Gemini 3.1 Flash Live Preview

Giá audio output Gemini:  $12.00/M  →  rẻ hơn ~5.3x so với OpenAI ($64/M)
Giá audio input Gemini:   $3.00/M   →  rẻ hơn ~10.7x so với OpenAI ($32/M)
Giá text input Gemini:    $0.75/M   →  rẻ hơn ~5.3x so với OpenAI ($4/M)
Giá text output Gemini:   $4.50/M   →  rẻ hơn ~5.3x so với OpenAI ($24/M)

Chi phí audio/min Gemini:  ~2.3¢     →  rẻ hơn ~5-8x so với OpenAI (~15-20¢)
```

---

## 4. Kết Luận

**Gemini 3.1 Flash Live Preview** rẻ hơn OpenAI Realtime API **5-10 lần** trên mọi modality.

Ưu điểm của Gemini:
- Có sẵn **free tier** (OpenAI không có free tier cho Realtime)
- Giá per-minute cố định dễ tính toán ($0.005/min in, $0.018/min out)
- Hỗ trợ video input ($0.002/min) — OpenAI gpt-realtime không hỗ trợ video
- Google Search grounding có 5,000 prompts/tháng miễn phí

Hạn chế của Gemini:
- Model vẫn đang ở trạng thái **Preview** (có thể thay đổi hoặc bị discontinue)
- Không có cache discount như OpenAI (80-90% discount trên cache hit)
- Rate limit có thể hạn chế hơn

**Với scenario voice agent**, sử dụng Gemini 3.1 Flash Live Preview là lựa chọn **rẻ nhất** trên thị trường hiện tại.
