# LiveKit Agent Optimizations — Silero VAD, Noise Cancellation, Framework Features

**Date:** 2026-03-30
**Scope:** Python agent worker + Go backend (minor)
**Goal:** Replace custom implementations with LiveKit SDK built-in features, add noise cancellation, improve cold start, eliminate mutable global state.
**Constraint:** Preserve lazy session pattern (start Gemini on speech, stop after 30s idle) for cost savings.

---

## 1. Silero VAD replaces custom RMS detection

### Problem

Custom `_compute_rms()` + `_vad_loop()` in `main.py:42-56` uses a fixed RMS energy threshold (0.02) with consecutive frame counting. This is fragile in noisy environments and does not distinguish speech from non-speech noise. ~40 lines of manual PCM processing.

### Solution

Replace with Silero VAD, a neural-network-based voice activity detector shipped as a LiveKit plugin. Silero classifies speech vs noise at the model level — more accurate for Vietnamese in outdoor/noisy settings.

**Not using MultilingualModel** — that is a turn detector designed for the AgentSession STT pipeline. The lazy session only needs binary "is someone speaking?" detection to trigger session start. Gemini realtime handles its own turn detection once the session is active.

### Changes — `main.py`

Remove:
- `_compute_rms()` function
- `_vad_loop()` coroutine
- `ENERGY_THRESHOLD`, `SPEECH_FRAMES_REQUIRED` constants
- `struct`, `math` imports

Add:
```python
from livekit.agents import vad as vad_mod
from livekit.plugins import silero

# Pre-loaded VAD instance (see section 4 — prewarm)
# _vad is set from ctx.proc.userdata["vad"] in entrypoint

async def _start_vad(track: rtc.Track):
    audio_stream = rtc.AudioStream(track)
    vad_stream = _vad.stream()

    async def _push_frames():
        async for event in audio_stream:
            vad_stream.push_frame(event.frame)
        vad_stream.end_input()

    asyncio.create_task(_push_frames())

    async for event in vad_stream:
        if event.type == vad_mod.VADEventType.START_OF_SPEECH:
            _on_speech()
```

The `VAD.stream()` returns a `VADStream`. Audio frames are pushed via `push_frame(frame)` and events are consumed via async iteration. Two concurrent tasks: one pushes frames from the `rtc.AudioStream`, the other iterates `VADEvent` objects checking for `START_OF_SPEECH`.

Wire `_start_vad` into `track_subscribed` handler (same trigger point as current `_vad_loop`).

### Dependency

`pyproject.toml`: add `livekit-plugins-silero~=1.0`

---

## 2. BVC noise cancellation

### Problem

No noise filtering on user audio input. Visually impaired users often use the app outdoors (streets, markets, public transport). Background noise degrades Gemini speech recognition and causes hallucinations.

### Solution

Add BVC (Browser Voice Cancellation) noise cancellation to `RoomOptions` when starting each AgentSession. BVC runs server-side, filtering noise before audio reaches Gemini.

### Changes — `main.py`

Import path verified against installed SDK (`livekit.agents.voice.room_io`):
```python
from livekit.agents.voice.room_io import RoomOptions, AudioInputOptions
from livekit.plugins import noise_cancellation

# In _start_session():
await active_session.start(
    agent=agent,
    room=ctx.room,
    room_options=RoomOptions(
        video_input=True,
        audio_input=AudioInputOptions(
            noise_cancellation=noise_cancellation.BVC(),
        ),
    ),
)
```

### Dependency

`pyproject.toml`: add `livekit-plugins-noise-cancellation~=0.2`

---

## 3. `user_away_timeout` replaces custom idle monitor

### Problem

Custom `_idle_monitor()` in `main.py:276-283` polls every 5 seconds, checks `time.monotonic()` delta against `IDLE_TIMEOUT`. Manual `last_activity` bookkeeping on every speech event.

### Solution

Use `AgentSession`'s built-in `user_away_timeout` parameter. The framework emits a `user_state_changed` event when the user has been silent for the specified duration. Listen for `"away"` state to trigger `_stop_session()`.

### Changes — `main.py`

Remove:
- `last_activity` variable and all updates to it
- `_idle_monitor()` coroutine and its `asyncio.create_task`
- `IDLE_TIMEOUT` constant
- `time` import (if no other usage)

Modify `_start_session()`:
```python
active_session = AgentSession(
    llm=google.realtime.RealtimeModel(...),
    user_away_timeout=30,
)

@active_session.on("user_state_changed")
def _on_user_state(ev):
    if ev.new_state == "away":
        asyncio.create_task(_stop_session())
```

The `user_away_timeout` uses a simple `asyncio.TimerHandle` internally (verified in SDK source). It fires when both user and agent are in "listening" state for 30s — no VAD dependency. The timer resets whenever user or agent state changes (speech, thinking, tool use). This is compatible with the lazy session pattern: the Gemini realtime model handles audio processing and state transitions internally.

Modify `_on_speech()`: remove `last_activity = time.monotonic()`, keep only the session-start trigger.

---

## 4. Prewarm hook for Silero VAD

### Problem

First speech detection incurs cold-start latency (~200-500ms) while the Silero VAD model loads into memory.

### Solution

Pre-load the Silero model when the worker process starts, before any job is dispatched.

### Changes — `main.py`

Store the loaded model in `proc.userdata` so the entrypoint reuses it instead of loading again:

```python
def prewarm(proc):
    proc.userdata["vad"] = silero.VAD.load()

if __name__ == "__main__":
    cli.run_app(
        WorkerOptions(
            entrypoint_fnc=entrypoint,
            agent_name="aisee-agent",
            prewarm_fnc=prewarm,
        ),
    )
```

In `entrypoint()`, retrieve via: `_vad = ctx.proc.userdata["vad"]`

Note: `prewarm_fnc` is a sync function (not async). The `WorkerOptions` + `cli.run_app` pattern is still supported in the installed SDK version alongside the newer `AgentServer` pattern.

---

## 5. Cache RoomServiceClient and DispatchClient (Go)

### Problem

`pkg/livekit/client.go` creates a new `RoomServiceClient` and `AgentDispatchClient` on every method call. Each instantiation sets up a new HTTP client internally.

### Solution

Lazy-initialize and cache both clients on the `Client` struct.

### Changes — `pkg/livekit/client.go`

Initialize both clients eagerly in `NewClient()`. The constructors are cheap (just store config; HTTP client is created on first use). This avoids data races — multiple goroutines call `RoomClient()` concurrently from different HTTP handlers.

```go
type Client struct {
    apiKey    string
    apiSecret string
    host      string
    roomSvc   *lksdk.RoomServiceClient
    dispSvc   *lksdk.AgentDispatchClient
}

func NewClient(host, apiKey, apiSecret string) *Client {
    return &Client{
        host:      host,
        apiKey:    apiKey,
        apiSecret: apiSecret,
        roomSvc: lksdk.NewRoomServiceClient(
            host, apiKey, apiSecret,
        ),
        dispSvc: lksdk.NewAgentDispatchServiceClient(
            host, apiKey, apiSecret,
        ),
    }
}

func (c *Client) RoomClient() *lksdk.RoomServiceClient {
    return c.roomSvc
}

func (c *Client) DispatchClient() *lksdk.AgentDispatchClient {
    return c.dispSvc
}
```

---

## 6. Tools as Agent class methods

### Problem

`tools.py` uses mutable global state (`_backend`, `_local_participant`, `_user_context`) initialized via `init_tools()`. Not thread-safe, not testable, couples all tools to module-level singletons.

### Solution

Move tool functions to methods on `AiSeeAgent` class. Pass dependencies via constructor. Tools access state through `self`.

Verified: `@function_tool()` works on instance methods. The SDK's `_BaseFunctionTool` implements `__get__` (descriptor protocol) which binds `self` and strips it from the function signature exposed to the LLM. See `tool_context.py:201-206`.

### Changes — `tools.py`

Remove:
- `_backend`, `_local_participant`, `_user_context` globals
- `init_tools()` function
- `_user_identity()`, `_uid()` module-level helpers
- `ALL_TOOLS` list

Restructure:
```python
class AiSeeAgent(Agent):
    def __init__(
        self,
        backend: BackendClient,
        local_participant: rtc.LocalParticipant,
        user_context: dict,
        locale: str = 'vi',
        wake_up: bool = False,
    ):
        self._backend = backend
        self._lp = local_participant
        self._ctx = user_context
        super().__init__(instructions=select_prompt(locale, wake_up))

    def _uid(self) -> str:
        return self._ctx.get('user_id', '')

    def _user_identity(self) -> str:
        return self._ctx.get('user_identity', self._uid())

    @function_tool()
    async def make_voice_call(self, contact_name: str) -> dict:
        """Call a family member by name or relationship (voice only)."""
        return await self._backend.tool_call(
            "make_voice_call", self._uid(),
            {"contact_name": contact_name},
        )

    # ... all other tools as methods ...
```

### Changes — `main.py`

Remove:
- `from tools import ALL_TOOLS, init_tools`
- `init_tools(...)` call

Import `AiSeeAgent` from `tools.py` (move class there, remove from `main.py`).

Modify `_start_session()`:
```python
agent = AiSeeAgent(
    backend=backend,
    local_participant=ctx.room.local_participant,
    user_context=user_context,
    locale=user_context.get('locale', 'vi'),
    wake_up=wake_up,
)
```

---

## 7. Transcription stream verification

### Problem

`AgentSession.start()` auto-publishes transcriptions via `lk.transcription` text stream topic. Need to verify this works with the lazy session pattern.

### Solution

No code changes needed on agent side — the framework handles this. Document that Flutter app should subscribe to `lk.transcription` topic for subtitle display. Verify in testing that transcriptions are published when Gemini session is active.

### Notes

- Transcriptions only flow while `active_session` is running (expected with lazy session)
- Flutter side: `room.registerTextStreamHandler('lk.transcription', handler)` (separate task, not in this spec)

---

## 8. Keep stale room deletion — add comment explaining why

### Problem

`CreateRoomWithAgent()` in `client.go:106` calls `DeleteRoom` before `CreateRoom`. This was originally thought to be removable, but `CreateDispatch` idempotency is unverified — calling it multiple times for the same `(agent_name, room)` pair may accumulate duplicate dispatches, each spawning an agent.

### Solution

Keep the current `DeleteRoom` call. The existing duplicate-agent detection in `main.py:77-83` provides a safety net, but preventing duplicate dispatches at the source is better than relying on the agent to self-terminate.

Add a comment explaining the intent:

```go
// Delete stale room to prevent dispatch accumulation.
// CreateDispatch is not idempotent — calling it on an existing
// room adds another dispatch, spawning a duplicate agent.
_ = c.DeleteRoom(ctx, roomName)
```

No code changes — only add the comment.

---

## Dependencies summary

### New Python dependencies (`pyproject.toml`)

```toml
dependencies = [
    "livekit-agents[images]~=1.0",
    "livekit-plugins-google~=1.0",
    "livekit-plugins-silero~=1.0",
    "livekit-plugins-noise-cancellation~=0.2",
    "httpx>=0.27.0",
]
```

### Go — no new dependencies

---

## Files changed

| File | Type | Summary |
|------|------|---------|
| `agent_worker/pyproject.toml` | Edit | Add silero + noise-cancellation deps |
| `agent_worker/main.py` | Edit | Silero VAD, noise cancellation, user_away_timeout, prewarm, remove custom VAD/idle |
| `agent_worker/tools.py` | Rewrite | Tools as AiSeeAgent class methods, remove global state |
| `pkg/livekit/client.go` | Edit | Eager-init cached clients, add comment on DeleteRoom |

---

## Risk assessment

| Risk | Mitigation |
|------|------------|
| Silero VAD sensitivity differs from custom RMS | Test with Vietnamese speech samples; Silero is more accurate by design |
| Noise cancellation adds latency | BVC is optimized for realtime; measure round-trip in staging |
| `user_away_timeout` event timing | Verify event fires correctly with lazy session pattern |
| Tools refactor breaks tool registration | Verify all 22 tools register correctly via agent framework |
| Dual VAD (standalone + AgentSession internal) | Acceptable cost; standalone VAD is lightweight inference only |

---

## Out of scope

- Flutter app changes for transcription subscription
- MultilingualModel turn detection (requires always-on STT, incompatible with lazy session)
- Gemini model version upgrade (separate evaluation needed)
- `delete_room_on_close` on AgentSession (only works for user rooms, needs more analysis)
- Migration to `AgentServer` / `@server.rtc_session()` pattern (current `WorkerOptions` + `cli.run_app` still works)
