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
from livekit.plugins import silero

# In _start_vad(track):
async def _start_vad(track: rtc.Track):
    vad = silero.VAD.load()
    stream = vad.stream(track)
    async for event in stream:
        if event.type == silero.VADEventType.START_OF_SPEECH:
            _on_speech()
```

Wire `_start_vad` into `track_subscribed` handler (same trigger point as current `_vad_loop`).

### Dependency

`pyproject.toml`: add `livekit-plugins-silero>=1.0.0`

---

## 2. BVC noise cancellation

### Problem

No noise filtering on user audio input. Visually impaired users often use the app outdoors (streets, markets, public transport). Background noise degrades Gemini speech recognition and causes hallucinations.

### Solution

Add BVC (Browser Voice Cancellation) noise cancellation to `RoomOptions` when starting each AgentSession. BVC runs server-side, filtering noise before audio reaches Gemini.

### Changes — `main.py`

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

`pyproject.toml`: add `livekit-plugins-noise-cancellation>=0.2.0`

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
def _on_user_state(state):
    if state == "away":
        asyncio.create_task(_stop_session())
```

Modify `_on_speech()`: remove `last_activity = time.monotonic()`, keep only the session-start trigger.

---

## 4. Prewarm hook for Silero VAD

### Problem

First speech detection incurs cold-start latency (~200-500ms) while the Silero VAD model loads into memory.

### Solution

Pre-load the Silero model when the worker process starts, before any job is dispatched.

### Changes — `main.py`

```python
async def prewarm(proc):
    silero.VAD.load()

if __name__ == "__main__":
    cli.run_app(
        WorkerOptions(
            entrypoint_fnc=entrypoint,
            agent_name="aisee-agent",
            prewarm_fnc=prewarm,
        ),
    )
```

---

## 5. Cache RoomServiceClient and DispatchClient (Go)

### Problem

`pkg/livekit/client.go` creates a new `RoomServiceClient` and `AgentDispatchClient` on every method call. Each instantiation sets up a new HTTP client internally.

### Solution

Lazy-initialize and cache both clients on the `Client` struct.

### Changes — `pkg/livekit/client.go`

```go
type Client struct {
    apiKey    string
    apiSecret string
    host      string
    roomSvc   *lksdk.RoomServiceClient
    dispSvc   *lksdk.AgentDispatchClient
}

func (c *Client) RoomClient() *lksdk.RoomServiceClient {
    if c.roomSvc == nil {
        c.roomSvc = lksdk.NewRoomServiceClient(
            c.host, c.apiKey, c.apiSecret,
        )
    }
    return c.roomSvc
}

func (c *Client) DispatchClient() *lksdk.AgentDispatchClient {
    if c.dispSvc == nil {
        c.dispSvc = lksdk.NewAgentDispatchServiceClient(
            c.host, c.apiKey, c.apiSecret,
        )
    }
    return c.dispSvc
}
```

No sync needed — `Client` is created once at server startup and used from a single goroutine context per request (the HTTP handler already serializes access).

---

## 6. Tools as Agent class methods

### Problem

`tools.py` uses mutable global state (`_backend`, `_local_participant`, `_user_context`) initialized via `init_tools()`. Not thread-safe, not testable, couples all tools to module-level singletons.

### Solution

Move tool functions to methods on `AiSeeAgent` class. Pass dependencies via constructor. Tools access state through `self`.

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

## 8. Remove stale room deletion from CreateRoomWithAgent

### Problem

`CreateRoomWithAgent()` in `client.go:106` calls `DeleteRoom` before `CreateRoom` to clear stale auto-dispatch config. This is a workaround, not a fix — it causes a brief window where the room does not exist.

### Solution

Remove the preemptive `DeleteRoom` call. LiveKit Cloud handles room recreation correctly — `CreateRoom` with the same name either returns the existing room or creates a new one. The dispatch is per-room, not per-room-name.

### Changes — `pkg/livekit/client.go`

```go
func (c *Client) CreateRoomWithAgent(
    ctx context.Context, roomName string,
) error {
    _, err := c.RoomClient().CreateRoom(
        ctx, &lkproto.CreateRoomRequest{
            Name:             roomName,
            EmptyTimeout:     300,
            DepartureTimeout: 30,
        },
    )
    if err != nil {
        return fmt.Errorf("create room %s: %w", roomName, err)
    }

    _, err = c.DispatchClient().CreateDispatch(
        ctx, &lkproto.CreateAgentDispatchRequest{
            AgentName: "aisee-agent",
            Room:      roomName,
        },
    )
    if err != nil {
        return fmt.Errorf("dispatch agent to %s: %w", roomName, err)
    }
    return nil
}
```

---

## Dependencies summary

### New Python dependencies (`pyproject.toml`)

```toml
dependencies = [
    "livekit-agents[images]>=1.0.0",
    "livekit-plugins-google>=1.0.0",
    "livekit-plugins-silero>=1.0.0",
    "livekit-plugins-noise-cancellation>=0.2.0",
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
| `pkg/livekit/client.go` | Edit | Cache clients, remove stale room deletion |

---

## Risk assessment

| Risk | Mitigation |
|------|------------|
| Silero VAD sensitivity differs from custom RMS | Test with Vietnamese speech samples; Silero is more accurate by design |
| Noise cancellation adds latency | BVC is optimized for realtime; measure round-trip in staging |
| `user_away_timeout` event timing | Verify event fires correctly with lazy session pattern |
| Tools refactor breaks tool registration | Verify all 23 tools register correctly via agent framework |
| Stale room deletion removal | Test that CreateRoom is idempotent on LiveKit Cloud |

---

## Out of scope

- Flutter app changes for transcription subscription
- MultilingualModel turn detection (requires always-on STT, incompatible with lazy session)
- Gemini model version upgrade (separate evaluation needed)
- `delete_room_on_close` on AgentSession (only works for user rooms, needs more analysis)
