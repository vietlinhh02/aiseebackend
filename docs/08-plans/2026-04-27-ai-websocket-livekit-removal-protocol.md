# AI WebSocket Protocol For LiveKit-Free Assistant

Date: 2026-04-27

Status: implementation contract

Goal: remove LiveKit from the AI assistant path while keeping LiveKit for human
voice/video calls.

## Boundary

AI assistant traffic uses:

```text
Flutter app <-> /api/v1/ai/ws <-> backend/agent <-> Google Gemini Live SDK
```

Human calls still use LiveKit rooms and the existing call screens/services.

## HTTP Session

`POST /api/v1/ai/session`

Authenticated with the normal app JWT. The backend checks AI quota and returns:

```json
{
  "session_id": "ai-uuid",
  "ws_url": "wss://host/api/v1/ai/ws?token=...",
  "audio_input": {"encoding": "pcm_s16le", "sample_rate": 16000, "channels": 1},
  "audio_output": {"encoding": "pcm_s16le", "sample_rate": 24000, "channels": 1},
  "camera": {"encoding": "jpeg", "max_fps": 1}
}
```

## WebSocket Frames

Text frames are UTF-8 JSON envelopes.

Binary frames are media frames. The initial version treats binary frames as mic
PCM unless a preceding `media_mode` envelope switches to `camera_jpeg`.

## JSON Envelope

All JSON messages use:

```json
{
  "type": "agent_state",
  "request_id": "optional-correlation-id",
  "payload": {}
}
```

Required message types:

- `ready`: backend confirms session is attached to Gemini.
- `agent_state`: `idle`, `listening`, `thinking`, `speaking`, `error`,
  `disconnected`.
- `audio_start`: app is about to stream mic PCM.
- `audio_stop`: app stopped mic PCM.
- `media_mode`: app says following binary frames are `audio_pcm` or
  `camera_jpeg`.
- `tool_request`: backend asks the app to run a phone-local action.
- `tool_result`: app returns the result for a `tool_request`.
- `interrupt`: app/user interrupts model output.
- `error`: recoverable error with a user-safe message.
- `ping` / `pong`: keepalive.

## Phone-Local Tools

Gemini function calls are handled server-side. Backend tools still go through
`/api/internal/tool-call`. Phone-local tools go over the AI WebSocket:

```json
{
  "type": "tool_request",
  "request_id": "tool-uuid",
  "payload": {
    "name": "enable_camera",
    "params": {}
  }
}
```

The app must answer:

```json
{
  "type": "tool_result",
  "request_id": "tool-uuid",
  "payload": {
    "status": "ok"
  }
}
```

Initial phone-local tool names:

- `enable_camera`
- `disable_camera`
- `get_location`
- `open_maps`
- `schedule_reminder`
- `play_audio`
- `control_audio`

Tool requests must have timeouts. The model must receive a Gemini
`FunctionResponse` even when a phone-local tool fails or times out.

## Camera Flow

1. Gemini calls `activate_camera`.
2. Backend checks quota and sends `tool_request: enable_camera`.
3. App enables local camera, keeps the local preview, and starts guarded JPEG
   frame bursts.
4. App sends `media_mode: camera_jpeg`, then binary JPEG frames at max 1 FPS.
5. Gemini calls `deactivate_camera`.
6. Backend sends `tool_request: disable_camera`.
7. App stops camera frames and returns to audio-only mode.

## Wake Flow

Wake word remains local on the phone. After wake:

1. App stops wake listener.
2. App opens or reuses AI WebSocket.
3. App starts mic PCM streaming.
4. Backend sends realtime audio to Gemini.
5. Backend returns Gemini audio as binary PCM output frames.
6. App enters grace period after `agent_state: idle`.

## Non-Goals

- Do not move Google API keys into the app.
- Do not remove LiveKit call/video-call code.
- Do not route human calls over the AI WebSocket.
- Do not keep LiveKit RPC for AI phone-local tools after cutover.
