# Nguồn nghiên cứu (Research Sources)

Tài liệu tổng hợp các nguồn tham khảo đã sử dụng trong quá trình thiết kế kỹ thuật AiSee.

## AI Models

### Speech-to-Text (STT)

| Model | Nguồn | Ghi chú |
|---|---|---|
| PhoWhisper | [VinAI Research](https://github.com/VinAIResearch/PhoWhisper) | Fine-tuned Whisper cho tiếng Việt. WER ~6-8% trên VLSP benchmark. Dùng trong V1 architecture. |
| Parakeet-TDT | [NVIDIA NeMo](https://catalog.ngc.nvidia.com/orgs/nvidia/teams/nemo/models/parakeet-tdt-0.6b-v2) | 600M params, multi-language, có Vietnamese support. Alternative cho PhoWhisper. |
| Whisper (OpenAI) | [OpenAI](https://github.com/openai/whisper) | Base model, hỗ trợ Vietnamese nhưng chất lượng thấp hơn PhoWhisper. |
| faster-whisper | [GitHub](https://github.com/SYSTRAN/faster-whisper) | CTranslate2 inference cho Whisper, nhanh 4x, dùng ít RAM hơn. Dùng với PhoWhisper trong V1. |
| Gemini Live (STT built-in) | [Google AI](https://ai.google.dev/gemini-api/docs/live) | V2 architecture dùng Gemini native STT. Chất lượng Vietnamese tốt. |

### Large Language Models (LLM)

| Model | Nguồn | Ghi chú |
|---|---|---|
| Gemini 2.5 Flash | [Google AI](https://ai.google.dev/gemini-api/docs) | Model chính cho V2. Hỗ trợ realtime voice, vision, tool calling. |
| Gemini 2.5 Pro | [Google AI](https://ai.google.dev/gemini-api/docs) | Backup cho trường hợp cần reasoning phức tạp. |
| Qwen2.5-VL | [Alibaba/Qwen](https://github.com/QwenLM/Qwen2.5-VL) | Vision-Language model. Cân nhắc cho V1 self-hosted vision. 7B/72B params. |
| Kimi K2.5 | [Moonshot AI](https://kimi.moonshot.cn/) | Alternative LLM với Vietnamese support tốt. |

### Text-to-Speech (TTS)

| Model | Nguồn | Ghi chú |
|---|---|---|
| VieNeu-TTS | Research paper | Vietnamese neural TTS. Chất lượng giọng tự nhiên. Dùng trong V1 architecture. |
| Gemini Live (TTS built-in) | [Google AI](https://ai.google.dev/gemini-api/docs/live) | V2 dùng Gemini native TTS. Realtime, chất lượng tốt. |

### Vision Models

| Model | Nguồn | Ghi chú |
|---|---|---|
| Gemini Vision (built-in) | [Google AI](https://ai.google.dev/gemini-api/docs) | V2 dùng Gemini native vision. Đọc nhãn thuốc, mô tả cảnh vật. |
| Qwen2.5-VL | [Alibaba/Qwen](https://github.com/QwenLM/Qwen2.5-VL) | Alternative cho self-hosted vision. |

## Infrastructure

### LiveKit

| Resource | URL | Ghi chú |
|---|---|---|
| LiveKit Server | [GitHub](https://github.com/livekit/livekit) | WebRTC SFU, Go, open-source |
| LiveKit Agents | [GitHub](https://github.com/livekit/agents) | Python agent framework |
| livekit-plugins-google | [GitHub](https://github.com/livekit/agents/tree/main/livekit-plugins/livekit-plugins-google) | Gemini Live integration |
| livekit_client (Flutter) | [pub.dev](https://pub.dev/packages/livekit_client) | Flutter WebRTC SDK |
| LiveKit Docs | [docs.livekit.io](https://docs.livekit.io/) | Official documentation |
| LiveKit Cloud | [livekit.io](https://livekit.io/) | Managed option (post-MVP) |

### Inference Servers (V1 reference)

| Tool | URL | Ghi chú |
|---|---|---|
| vLLM | [GitHub](https://github.com/vllm-project/vllm) | High-throughput LLM serving. PagedAttention. |
| faster-whisper | [GitHub](https://github.com/SYSTRAN/faster-whisper) | CTranslate2-based Whisper inference |
| TensorRT-LLM | [NVIDIA](https://github.com/NVIDIA/TensorRT-LLM) | NVIDIA optimized LLM inference |

## Cloud APIs

### Gemini API

| Resource | URL |
|---|---|
| Gemini API Docs | [ai.google.dev](https://ai.google.dev/gemini-api/docs) |
| Gemini Live API | [ai.google.dev/gemini-api/docs/live](https://ai.google.dev/gemini-api/docs/live) |
| Pricing | [ai.google.dev/pricing](https://ai.google.dev/pricing) |
| Free tier limits | 2 RPM, 32k TPM (Gemini 2.0 Flash) |

### Twilio (reference, không dùng trong MVP)

| Resource | URL | Ghi chú |
|---|---|---|
| Twilio Voice | [twilio.com](https://www.twilio.com/voice) | Alternative cho PSTN calls. Không dùng vì AiSee chỉ dùng VoIP. |
| Twilio Pricing | [twilio.com/pricing](https://www.twilio.com/pricing) | Voice: $0.013/phút (US), đắt hơn cho VN numbers. |

### Deepgram (reference)

| Resource | URL | Ghi chú |
|---|---|---|
| Deepgram STT | [deepgram.com](https://deepgram.com/) | Cloud STT service. Vietnamese support qua Whisper model. |
| Deepgram Pricing | [deepgram.com/pricing](https://deepgram.com/pricing) | $0.0043/phút (Nova-2). Alternative nếu Gemini STT không đủ. |

### Mapbox

| Resource | URL |
|---|---|
| Mapbox Maps Flutter | [pub.dev](https://pub.dev/packages/mapbox_maps_flutter) |
| Mapbox Navigation | [docs.mapbox.com](https://docs.mapbox.com/api/navigation/) |
| Mapbox Pricing | Free tier: 25k requests/tháng |

## Market Research

### Thống kê thị lực

| Nguồn | Dữ liệu | URL |
|---|---|---|
| WHO | 2.2 tỷ người trên toàn cầu có suy giảm thị lực | [who.int/news-room/fact-sheets/detail/blindness-and-visual-impairment](https://www.who.int/news-room/fact-sheets/detail/blindness-and-visual-impairment) |
| WHO | 43% suy giảm thị lực do tật khúc xạ chưa được điều chỉnh | WHO Global Eye Health Report |
| IAPB Vision Atlas | Dữ liệu chi tiết theo quốc gia và khu vực | [atlas.iapb.org](https://www.iapb.org/learn/vision-atlas/) |

### Dữ liệu Việt Nam

| Nguồn | Dữ liệu |
|---|---|
| Bộ Y tế Việt Nam | Khoảng 2 triệu người mù, 5+ triệu người suy giảm thị lực nặng |
| Hội Nhãn khoa Việt Nam | Tỷ lệ mù lòa: 3.1% dân số trên 50 tuổi |
| Tổng cục Thống kê | Dân số cao tuổi (60+): ~12 triệu người (2025) |
| Viện Mắt Trung ương | Đục thủy tinh thể và glaucoma là nguyên nhân hàng đầu gây mù ở người cao tuổi VN |

### Smartphone penetration

| Nguồn | Dữ liệu |
|---|---|
| We Are Social / Kepios | VN smartphone penetration: ~73% (2025) |
| Statista | VN internet users: ~78 triệu (2025) |
| Google/Temasek | Southeast Asia digital economy report |

## Flutter Packages

| Package | URL | Version | Mục đích |
|---|---|---|---|
| riverpod | [pub.dev](https://pub.dev/packages/flutter_riverpod) | 2.x | State management |
| livekit_client | [pub.dev](https://pub.dev/packages/livekit_client) | 2.x | LiveKit WebRTC |
| flutter_callkit_incoming | [pub.dev](https://pub.dev/packages/flutter_callkit_incoming) | 0.x | Native call UI |
| flutter_local_notifications | [pub.dev](https://pub.dev/packages/flutter_local_notifications) | 17.x | Local notifications |
| flutter_chat_ui | [pub.dev](https://pub.dev/packages/flutter_chat_ui) | 1.x | Chat UI components |
| drift | [pub.dev](https://pub.dev/packages/drift) | 2.x | SQLite ORM |
| dio | [pub.dev](https://pub.dev/packages/dio) | 5.x | HTTP client |
| just_audio | [pub.dev](https://pub.dev/packages/just_audio) | 0.9.x | Audio playback |
| camera | [pub.dev](https://pub.dev/packages/camera) | 0.11.x | Camera access |
| google_sign_in | [pub.dev](https://pub.dev/packages/google_sign_in) | 6.x | Google OAuth2 |
| flutter_secure_storage | [pub.dev](https://pub.dev/packages/flutter_secure_storage) | 9.x | Secure key-value storage |
| mapbox_maps_flutter | [pub.dev](https://pub.dev/packages/mapbox_maps_flutter) | 2.x | Maps + navigation |
| permission_handler | [pub.dev](https://pub.dev/packages/permission_handler) | 11.x | Runtime permissions |
