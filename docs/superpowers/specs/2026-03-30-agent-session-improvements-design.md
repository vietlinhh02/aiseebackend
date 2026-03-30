# Agent Session Improvements: Auto-Greeting, Buffer Removal, Camera Fix

**Date:** 2026-03-30
**Status:** Draft

## Problem

Three issues with the current agent worker session management:

1. **No auto-greeting.** The agent uses a lazy session pattern — it only starts the Gemini session after VAD detects user speech. Users open the app and don't know the AI is available until they speak first.

2. **Broken audio buffer.** `BufferedAudioInput` is created and swapped onto `active_session.input.audio` *after* `session.start()`. The LiveKit `AgentInput.audio` setter triggers `_on_audio_input_changed()` which cancels the old forward task and creates a new one — but the original `AudioInput` (passed as `source` to `BufferedAudioInput`) has already been detached by the setter, so the wrapped source may stop producing frames. The net effect: buffered frames appear in logs but the AI never processes them.

3. **Camera toggle restarts session.** `activate_camera` / `deactivate_camera` call `_restart_session()` which tears down the entire Gemini WebSocket and rebuilds it. This takes 2-5 seconds during which the AI is unresponsive.

## Solution

### 1. Auto-start session on user connect

Start the Gemini session immediately when a user participant is detected in the room, instead of waiting for VAD speech detection.

- `_start_session()` is called directly in `entrypoint` after resolving `user_context`
- `first_start` / `wake_up` logic remains: first start uses `GREETING_SUFFIX`, subsequent restarts use `WAKEUP_SUFFIX`
- VAD listener is kept solely for restarting the session after `user_away_timeout` closes it

### 2. Remove broken buffer mechanism

- Delete `BufferedAudioInput` class
- Remove `audio_buffer`, `buffering` flag, and all buffer-related logic from `entrypoint`
- Remove the buffer swap after `session.start()`
- For wake-up restarts (after user_away), the session starts without buffered audio — Gemini's `START_SENSITIVITY_HIGH` + `prefix_padding_ms=100` handles speech onset detection

### 3. Always-on video input

- Start session with `video_input=True` always (instead of toggling via `video_enabled` flag)
- `_ParticipantVideoInputStream` auto-subscribes to `track_subscribed` events — when camera is off there are no video frames; when camera turns on, frames flow automatically
- `activate_camera` / `deactivate_camera` only send the RPC to the client; no session restart
- Remove: `video_enabled` flag, `_restart_session()`, `restarting` flag, `session_ctrl` dict, `restart_with_video` callback

## Files Changed

### `agent_worker/main.py`

- Delete `BufferedAudioInput` class (~25 lines)
- Delete `audio_buffer`, `buffering`, `video_enabled`, `restarting` variables
- Delete `_restart_session()` function
- Simplify `_start_session()`: remove buffer swap, always use `video_input=True`
- Remove `session_ctrl` from `AiSeeAgent` construction
- Call `_start_session()` directly after user context resolution (auto-start)
- Keep `_on_speech()` to call `_start_session()` for wake-up only
- Keep `_stop_session()` as-is (for user_away_timeout)

### `agent_worker/tools.py`

- Remove `session_ctrl` parameter from `AiSeeAgent.__init__`
- `activate_camera`: remove `asyncio.create_task(restart(...))` — just return RPC result
- `deactivate_camera`: same — just return RPC result

## Behavior After Changes

| Scenario | Before | After |
|---|---|---|
| User opens app | Silence until user speaks | AI greets immediately |
| User speaks first time | VAD → buffer → start session → greeting | Session already active, AI responds directly |
| Camera on | RPC + full session restart (2-5s) | RPC only, video flows instantly |
| Camera off | RPC + full session restart (2-5s) | RPC only, video stops instantly |
| User idle 30s | Session closes | Session closes (unchanged) |
| User speaks after idle | VAD → start session (wake_up) | VAD → start session (wake_up), same |

## Risks

- **Gemini session starts before user audio track is ready.** The `user_away_timeout=30` timer starts when the session starts. If the user's audio track hasn't been subscribed yet, the timer could fire before the user speaks. Mitigation: LiveKit's `RoomIO` handles `subscribed_fut` and skips the away timer until subscription completes (verified in SDK source).
- **Always-on video increases Gemini token usage.** Only if camera is actually on. When camera is off, no video track exists, so no frames are sent to Gemini.
