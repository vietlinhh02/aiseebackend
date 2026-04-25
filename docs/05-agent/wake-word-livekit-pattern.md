# Wake word và pattern LiveKit (hello-wakeword ↔ AiSee)

LiveKit **không** chạy wake word trên SFU hay trong agent server. Pattern chính thức: **client** phát hiện wake (ONNX / thư viện cục bộ) → **sau đó** kết nối phòng WebRTC và stream audio; **agent** chỉ xử lý hội thoại sau khi user đã “mở phiên”.

Tham chiếu: [livekit-examples/hello-wakeword](https://github.com/livekit-examples/hello-wakeword), [livekit-wakeword](https://github.com/livekit/livekit-wakeword).

## So sánh kiến trúc


| Bước (ý tưởng LiveKit)   | hello-wakeword (Python)                       | AiSee (Flutter)                                                                         |
| ------------------------ | --------------------------------------------- | --------------------------------------------------------------------------------------- |
| Nghe wake, mic idle      | `livekit-wakeword` + ONNX, `WakeWordListener` | `flutter_wake_word` + DaVoice + ONNX                                                    |
| Sau wake, mở phiên thoại | Join room + stream audio                      | LiveKit đã kết nối → bật mic + `wake_conversation_ready` + Gemini Live (`agent_worker`) |
| Agent                    | LiveKit Agents (ví dụ STT/LLM/TTS)            | LiveKit Agents + `livekit-plugins-google` (Gemini Live)                                 |


## AiSee đã bám pattern này chưa?

Có: wake chạy **trên điện thoại**; sau wake **kích hoạt mic và phiên thoại** trên LiveKit; agent là **participant agent** trong cùng room.

Khác biệt có chủ đích so với ví dụ Python:

- **Room cố định** `user-{id}` thay vì `wakeword-{uuid}` mỗi lần — hợp lệ cho app đăng nhập; ví dụ upstream dùng room tạm để minh họa vòng lặp “ngắt hẳn → nghe lại”.
- App **giữ** kết nối LiveKit và chỉ tắt/bật mic + grace period; không cần disconnect toàn phòng sau mỗi câu.

## Model ONNX và livekit-wakeword

- [livekit-wakeword](https://github.com/livekit/livekit-wakeword) dùng để **huấn luyện / export ONNX** (tương thích openWakeWord). **Không** thay thế trực tiếp `flutter_wake_word` trên Dart.
- Để dùng model do pipeline này sinh ra trên AiSee, cần **DaVoice / runtime ONNX trên app** chấp nhận đúng định dạng — đối chiếu tài liệu nhà cung cấp.

File mẫu cấu hình huấn luyện (copy ra repo riêng khi chạy CLI): [wakeword-livekit-aisee.example.yaml](./wakeword-livekit-aisee.example.yaml).

## Phản hồi người dùng sau wake (giống chuông `in.wav`)

Trong hello-wakeword, sau khi detect có file âm báo rồi mới vào room. Trên Flutter, AiSee gọi callback **ngay khi wake được chấp nhận** (trước khi nhả engine wake / bật mic) — xem `AssistantAudioCoordinator.onWakeAcknowledged` và `UserHomeScreen`.

## Không nên làm

- Chạy wake word **chỉ** trên `agent_worker` để thay wake trên máy user: khi app chưa publish audio / nền, server **không** nghe được mic điện thoại như client.
