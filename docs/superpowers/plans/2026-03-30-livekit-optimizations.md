# LiveKit Agent Optimizations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace custom VAD/idle implementations with LiveKit SDK features, add noise cancellation, eliminate global state in tools, cache Go clients.

**Architecture:** Swap components individually within the existing lazy session pattern. Agent worker changes are in `main.py` (lifecycle) and `tools.py` (refactor to class). Go backend change is a single file edit.

**Tech Stack:** Python 3.13, livekit-agents ~=1.0, livekit-plugins-silero ~=1.0, livekit-plugins-noise-cancellation ~=0.2, Go 1.22+, livekit server-sdk-go v2.16.1

**Spec:** `docs/superpowers/specs/2026-03-30-livekit-optimizations-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `agent_worker/pyproject.toml` | Edit | Add silero + noise-cancellation deps |
| `agent_worker/tools.py` | Rewrite | `AiSeeAgent` class with 22 tool methods |
| `agent_worker/main.py` | Edit | Silero VAD, noise cancellation, prewarm, user_away_timeout, import AiSeeAgent |
| `pkg/livekit/client.go` | Edit | Eager-init cached clients, comment on DeleteRoom |

---

### Task 1: Add Python dependencies

**Files:**
- Modify: `agent_worker/pyproject.toml`

- [ ] **Step 1: Update pyproject.toml**

```toml
[project]
name = "aisee-agent"
version = "0.1.0"
requires-python = ">=3.12"
dependencies = [
    "livekit-agents[images]~=1.0",
    "livekit-plugins-google~=1.0",
    "livekit-plugins-silero~=1.0",
    "livekit-plugins-noise-cancellation~=0.2",
    "httpx>=0.27.0",
]
```

- [ ] **Step 2: Install dependencies**

Run: `cd agent_worker && uv sync`
Expected: All packages install successfully, `uv.lock` updated.

- [ ] **Step 3: Verify imports work**

Run: `cd agent_worker && .venv/bin/python3 -c "from livekit.plugins import silero, noise_cancellation; print('ok')"`
Expected: `ok`

- [ ] **Step 4: Commit**

```bash
git add agent_worker/pyproject.toml agent_worker/uv.lock
git commit -m "deps: add silero VAD and noise cancellation plugins"
```

---

### Task 2: Refactor tools.py — AiSeeAgent class with tool methods

This is the largest change. Move all 22 tools from standalone functions with global state to instance methods on `AiSeeAgent`.

**Files:**
- Rewrite: `agent_worker/tools.py`
- Reference: `agent_worker/prompts.py` (for `select_prompt` import)
- Reference: `agent_worker/backend.py` (for `BackendClient` type)

- [ ] **Step 1: Rewrite tools.py**

Replace the entire file. The new structure:
- `AiSeeAgent(Agent)` class with `__init__` taking `backend`, `local_participant`, `user_context`, `locale`, `wake_up`
- `_uid()` and `_user_identity()` as instance methods
- All 22 `@function_tool()` decorated methods (identical logic, `self` instead of globals)
- No module-level mutable state

```python
"""AiSee agent with tool definitions — Phase 1-3."""

import json
import logging

from livekit import rtc
from livekit.agents import Agent, function_tool

from backend import BackendClient
from prompts import select_prompt

logger = logging.getLogger("aisee-agent")


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
        self._uctx = user_context
        super().__init__(instructions=select_prompt(locale, wake_up))

    def _uid(self) -> str:
        return self._uctx.get('user_id', '')

    def _user_identity(self) -> str:
        return self._uctx.get(
            'user_identity', self._uid(),
        )

    # --- Phase 1: Communication + Camera ---

    @function_tool()
    async def make_voice_call(self, contact_name: str) -> dict:
        """Call a family member by name or relationship (voice only)."""
        return await self._backend.tool_call(
            "make_voice_call", self._uid(),
            {"contact_name": contact_name},
        )

    @function_tool()
    async def make_video_call(self, contact_name: str) -> dict:
        """Video call a family member by name or relationship."""
        return await self._backend.tool_call(
            "make_video_call", self._uid(),
            {"contact_name": contact_name},
        )

    @function_tool()
    async def send_chat_message(
        self, contact_name: str, message: str,
    ) -> dict:
        """Send a text message to a family member."""
        return await self._backend.tool_call(
            "send_chat_message", self._uid(),
            {"contact_name": contact_name, "message": message},
        )

    @function_tool()
    async def read_unread_messages(
        self, contact_name: str = "",
    ) -> dict:
        """Read unread messages. Optionally filter by contact name."""
        params = {}
        if contact_name:
            params["contact_name"] = contact_name
        return await self._backend.tool_call(
            "read_unread_messages", self._uid(), params,
        )

    @function_tool()
    async def activate_camera(
        self, reason: str = "User requested",
    ) -> dict:
        """Enable camera to see through user's phone."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='enable_camera',
                payload='{}',
            )
            return json.loads(response)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    @function_tool()
    async def deactivate_camera(self) -> dict:
        """Disable camera, return to audio-only."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='disable_camera',
                payload='{}',
            )
            return json.loads(response)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    # --- Phase 2a: Health ---

    @function_tool()
    async def set_medication_reminder(
        self,
        medicine_name: str,
        dosage: str,
        times: list[str],
        frequency: str = "daily",
    ) -> dict:
        """Dat nhac nho uong thuoc. times la danh sach gio (VD: ["07:00", "21:00"])."""
        result = await self._backend.tool_call(
            "set_medication_reminder", self._uid(),
            {"medicine_name": medicine_name, "dosage": dosage,
             "times": times, "frequency": frequency},
        )
        reminder_action = result.pop("schedule_action", None)
        if reminder_action and isinstance(reminder_action, dict):
            try:
                await self._lp.perform_rpc(
                    destination_identity=self._user_identity(),
                    method='schedule_reminder',
                    payload=json.dumps(
                        reminder_action.get("params", {}),
                    ),
                )
            except Exception as exc:
                logger.warning(
                    "schedule_reminder RPC failed: %s", exc,
                )
        return result

    @function_tool()
    async def list_medication_reminders(self) -> dict:
        """Liet ke thuoc hom nay va trang thai da uong chua."""
        return await self._backend.tool_call(
            "list_medication_reminders", self._uid(), {},
        )

    @function_tool()
    async def mark_medication_taken(
        self, medicine_name: str,
    ) -> dict:
        """Xac nhan da uong thuoc."""
        return await self._backend.tool_call(
            "mark_medication_taken", self._uid(),
            {"medicine_name": medicine_name},
        )

    @function_tool()
    async def mark_medication_skipped(
        self, medicine_name: str,
    ) -> dict:
        """Xac nhan bo qua khong uong thuoc hom nay."""
        return await self._backend.tool_call(
            "mark_medication_skipped", self._uid(),
            {"medicine_name": medicine_name},
        )

    @function_tool()
    async def log_health_symptom(
        self,
        symptom: str,
        severity: str = "",
        notes: str = "",
    ) -> dict:
        """Ghi nhan trieu chung suc khoe (VD: dau dau, chong mat, met moi)."""
        params = {"symptom": symptom}
        if severity:
            params["severity"] = severity
        if notes:
            params["notes"] = notes
        return await self._backend.tool_call(
            "log_health_symptom", self._uid(), params,
        )

    @function_tool()
    async def log_health_metric(
        self,
        metric_type: str,
        value: str,
        unit: str = "",
    ) -> dict:
        """Ghi nhan chi so suc khoe (VD: huyet ap 140/90, nhiet do 37.5)."""
        params = {"metric_type": metric_type, "value": value}
        if unit:
            params["unit"] = unit
        return await self._backend.tool_call(
            "log_health_metric", self._uid(), params,
        )

    @function_tool()
    async def set_appointment_reminder(
        self,
        title: str,
        datetime: str,
        location: str = "",
        notes: str = "",
    ) -> dict:
        """Dat nhac nho lich hen bac si. datetime dang YYYY-MM-DD HH:MM."""
        result = await self._backend.tool_call(
            "set_appointment_reminder", self._uid(),
            {"title": title, "datetime": datetime,
             "location": location, "notes": notes},
        )
        reminder_action = result.pop("schedule_action", None)
        if reminder_action and isinstance(reminder_action, dict):
            try:
                await self._lp.perform_rpc(
                    destination_identity=self._user_identity(),
                    method='schedule_reminder',
                    payload=json.dumps(
                        reminder_action.get("params", {}),
                    ),
                )
            except Exception as exc:
                logger.warning(
                    "schedule_reminder RPC failed: %s", exc,
                )
        return result

    # --- Phase 2b: Emergency ---

    @function_tool()
    async def emergency_alert(self) -> dict:
        """SOS khan cap — gui thong bao den tat ca nguoi than kem vi tri GPS."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='get_location',
                payload='{}',
                response_timeout=15.0,
            )
            location = json.loads(response)
            lat = location.get("lat", 0)
            lng = location.get("lng", 0)
        except Exception:
            logger.warning(
                "get_location RPC failed, using unknown location",
            )
            lat, lng = 0, 0
        return await self._backend.tool_call_with_timeout(
            "emergency_alert", self._uid(),
            {"lat": lat, "lng": lng},
            timeout=20.0,
        )

    @function_tool()
    async def escalate_to_family(
        self,
        contact_name: str = "",
        skip: list[str] | None = None,
    ) -> dict:
        """Goi nguoi than ho tro khi AI khong giup duoc. Neu khong ai tra loi, goi lai voi skip de thu nguoi khac."""
        params: dict = {}
        if contact_name:
            params["contact_name"] = contact_name
        if skip:
            params["skip"] = skip
        return await self._backend.tool_call_with_timeout(
            "escalate_to_family", self._uid(), params,
            timeout=40.0,
        )

    # --- Phase 2c: Music ---

    @function_tool()
    async def play_music(self, query: str) -> dict:
        """Tim va phat nhac theo yeu cau."""
        result = await self._backend.tool_call(
            "play_music", self._uid(), {"query": query},
        )
        play_action = result.pop("play_action", None)
        if play_action and isinstance(play_action, dict):
            try:
                await self._lp.perform_rpc(
                    destination_identity=self._user_identity(),
                    method='play_audio',
                    payload=json.dumps(
                        play_action.get("params", {}),
                    ),
                )
            except Exception as exc:
                logger.warning(
                    "play_audio RPC failed: %s", exc,
                )
        return result

    @function_tool()
    async def control_music(self, action: str) -> dict:
        """Dieu khien nhac dang phat: pause, resume, stop."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='control_audio',
                payload=json.dumps({"command": action}),
            )
            return json.loads(response)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    # --- Phase 2d: Navigation ---

    @function_tool()
    async def navigate_to(self, destination: str) -> dict:
        """Mo ban do va chi duong den dia diem."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='open_maps',
                payload=json.dumps(
                    {"destination": destination},
                ),
            )
            return json.loads(response)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    # --- Phase 3: Volunteer + Location ---

    @function_tool()
    async def escalate_to_volunteer(self) -> dict:
        """Ket noi voi tinh nguyen vien khi gia dinh khong tra loi. Tra ve match_id de theo doi."""
        return await self._backend.tool_call(
            "escalate_to_volunteer", self._uid(), {},
        )

    @function_tool()
    async def rate_volunteer(
        self,
        volunteer_id: str,
        match_id: str,
        score: int,
    ) -> dict:
        """Danh gia tinh nguyen vien sau cuoc goi (1-5 sao)."""
        return await self._backend.tool_call(
            "rate_volunteer", self._uid(),
            {"volunteer_id": volunteer_id,
             "match_id": match_id, "score": score},
        )

    @function_tool()
    async def get_current_location(self) -> dict:
        """Lay vi tri hien tai — tra ve dia chi doc duoc va toa do GPS."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='get_location',
                payload='{}',
                response_timeout=15.0,
            )
            return json.loads(response)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}

    @function_tool()
    async def update_location(self) -> dict:
        """Cap nhat vi tri hien tai cua nguoi dung len he thong."""
        try:
            response = await self._lp.perform_rpc(
                destination_identity=self._user_identity(),
                method='get_location',
                payload='{}',
                response_timeout=15.0,
            )
            location = json.loads(response)
            lat = location.get("lat", 0)
            lng = location.get("lng", 0)
        except Exception as exc:
            return {"status": "error", "error": str(exc)}
        return await self._backend.tool_call(
            "update_location", self._uid(),
            {"lat": lat, "lng": lng},
        )
```

- [ ] **Step 2: Verify syntax**

Run: `cd agent_worker && .venv/bin/python3 -c "from tools import AiSeeAgent; print('22 tools:', len([m for m in dir(AiSeeAgent) if hasattr(getattr(AiSeeAgent, m, None), '__livekit_tool_info')]))"`
Expected: `22 tools: 22`

- [ ] **Step 3: Commit**

```bash
git add agent_worker/tools.py
git commit -m "refactor: move tools to AiSeeAgent class methods, remove global state"
```

---

### Task 3: Rewrite main.py — Silero VAD, noise cancellation, prewarm, user_away_timeout

**Files:**
- Modify: `agent_worker/main.py`
- Reference: `agent_worker/tools.py` (new `AiSeeAgent` class)

- [ ] **Step 1: Rewrite main.py**

Replace the entire file with:

```python
import asyncio
import logging

from livekit import rtc
from livekit.agents import AgentSession, WorkerOptions, cli, vad as vad_mod
from livekit.agents.voice.room_io import RoomOptions, AudioInputOptions
from livekit.plugins import google, noise_cancellation, silero

from backend import BackendClient
from config import GEMINI_API_KEY, GEMINI_MODEL, GO_BACKEND_URL, INTERNAL_API_KEY
from tools import AiSeeAgent

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("aisee-agent")


async def _publish_state(local_participant, state: str):
    """Publish agent state as participant attribute for frontend UI."""
    try:
        await local_participant.set_attributes(
            {'lk.agent.state': state},
        )
        logger.info("published state: %s", state)
    except Exception as exc:
        logger.warning("failed to publish state %s: %s", state, exc)


async def entrypoint(ctx):
    logger.info("connecting to room %s", ctx.room.name)
    await ctx.connect()
    logger.info("connected to room %s", ctx.room.name)

    local = ctx.room.local_participant
    await _publish_state(local, 'initializing')
    logger.info(
        "local_participant: identity=%s sid=%s",
        local.identity, local.sid,
    )
    for sid, p in ctx.room.remote_participants.items():
        logger.info(
            "remote_participant: identity=%s sid=%s kind=%s",
            p.identity, sid, p.kind,
        )

    # Wait briefly so concurrent agents can see each other.
    await asyncio.sleep(1)

    # Exit if another agent is already in the room.
    for p in ctx.room.remote_participants.values():
        if p.kind == rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
            logger.info(
                "duplicate agent detected: %s, exiting",
                p.identity,
            )
            return

    await _publish_state(local, 'idle')

    # Log participant join/leave for debugging.
    @ctx.room.on("participant_connected")
    def _on_join(participant):
        logger.info(
            "participant_connected: identity=%s kind=%s",
            participant.identity, participant.kind,
        )

    @ctx.room.on("participant_disconnected")
    def _on_leave(participant):
        logger.info(
            "participant_disconnected: identity=%s kind=%s",
            participant.identity, participant.kind,
        )

    # Exit if a second agent joins later.
    duplicate_event = asyncio.Event()

    @ctx.room.on("participant_connected")
    def _on_duplicate(participant):
        if participant.kind == rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
            logger.info("another agent joined, exiting")
            duplicate_event.set()

    backend = BackendClient(GO_BACKEND_URL, INTERNAL_API_KEY)

    user_id = ""
    for participant in ctx.room.remote_participants.values():
        if participant.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
            user_id = participant.identity
            break

    if not user_id and ctx.room.name.startswith("user-"):
        user_id = ctx.room.name.replace("user-", "")

    logger.info("resolved user_id=%s", user_id)

    user_context = {
        'user_id': user_id,
        'user_name': '',
        'locale': 'vi',
        'accessibility': 'low_vision',
        'has_family_link': 'false',
    }

    def _read_user_attributes(participant):
        for key in (
            'user_name', 'user_id', 'locale',
            'accessibility', 'has_family_link',
        ):
            val = participant.attributes.get(key)
            if val:
                user_context[key] = val
        logger.info("user_context updated: %s", user_context)

    for p in ctx.room.remote_participants.values():
        if p.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
            _read_user_attributes(p)

    @ctx.room.on("participant_attributes_changed")
    def _on_attributes(changed_attributes, participant):
        if participant.kind == rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
            return
        _read_user_attributes(participant)

    user_identity = ""
    for p in ctx.room.remote_participants.values():
        if p.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
            user_identity = p.identity
            break
    if not user_identity:
        user_identity = user_id
    user_context['user_identity'] = user_identity
    logger.info("resolved user_identity=%s (for RPC)", user_identity)

    @ctx.room.on("participant_connected")
    def _on_user_identity_update(participant):
        if participant.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
            user_context['user_identity'] = participant.identity
            logger.info(
                "updated user_identity=%s", participant.identity,
            )

    # ── Lazy session state ──────────────────────────────────
    active_session: AgentSession | None = None
    first_start = True
    vad_instance = ctx.proc.userdata["vad"]

    async def _start_session():
        nonlocal active_session, first_start
        if active_session is not None:
            return

        wake_up = not first_start
        first_start = False

        logger.info(
            "starting Gemini session (wake_up=%s) "
            "current_tracks=%d",
            wake_up,
            len(ctx.room.local_participant.track_publications),
        )

        agent = AiSeeAgent(
            backend=backend,
            local_participant=ctx.room.local_participant,
            user_context=user_context,
            locale=user_context.get('locale', 'vi'),
            wake_up=wake_up,
        )
        active_session = AgentSession(
            llm=google.realtime.RealtimeModel(
                model=GEMINI_MODEL,
                api_key=GEMINI_API_KEY,
                voice="Aoede",
            ),
            user_away_timeout=30,
        )

        @active_session.on("user_state_changed")
        def _on_user_state(ev):
            if ev.new_state == "away":
                asyncio.create_task(_stop_session())

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

    async def _stop_session():
        nonlocal active_session
        if active_session is None:
            return
        logger.info("idle timeout — closing Gemini session")
        await _publish_state(local, 'idle')
        await active_session.aclose()
        active_session = None

        for pub in list(local.track_publications.values()):
            if pub.track and pub.track.kind == rtc.TrackKind.KIND_AUDIO:
                logger.info(
                    "unpublishing stale audio track: %s", pub.sid,
                )
                await local.unpublish_track(pub.sid)

        logger.info(
            "Gemini session closed, tracks=%d, idle mode",
            len(local.track_publications),
        )

    def _on_speech():
        if active_session is None:
            asyncio.create_task(_start_session())

    # ── Silero VAD on user audio track ──────────────────────
    async def _start_vad(track: rtc.Track):
        audio_stream = rtc.AudioStream(track)
        vad_stream = vad_instance.stream()

        async def _push_frames():
            async for event in audio_stream:
                vad_stream.push_frame(event.frame)
            vad_stream.end_input()

        asyncio.create_task(_push_frames())

        async for event in vad_stream:
            if event.type == vad_mod.VADEventType.START_OF_SPEECH:
                _on_speech()

    @ctx.room.on("track_subscribed")
    def on_track(track, publication, participant):
        logger.info(
            "track_subscribed: kind=%s participant=%s",
            track.kind, participant.identity,
        )
        if track.kind == rtc.TrackKind.KIND_AUDIO:
            asyncio.create_task(_start_vad(track))

    for participant in ctx.room.remote_participants.values():
        for pub in participant.track_publications.values():
            track = pub.track
            if track and track.kind == rtc.TrackKind.KIND_AUDIO:
                asyncio.create_task(_start_vad(track))

    # ── Keep entrypoint alive until room disconnects ────────
    disconnect_event = asyncio.Event()

    @ctx.room.on("disconnected")
    def on_disconnect():
        disconnect_event.set()

    done, _ = await asyncio.wait(
        [
            asyncio.create_task(disconnect_event.wait()),
            asyncio.create_task(duplicate_event.wait()),
        ],
        return_when=asyncio.FIRST_COMPLETED,
    )
    await _publish_state(local, 'disconnected')
    await _stop_session()
    logger.info("entrypoint exiting")


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

- [ ] **Step 2: Verify syntax**

Run: `cd agent_worker && .venv/bin/python3 -c "import main; print('ok')"`
Expected: `ok` (or Silero model download on first run)

- [ ] **Step 3: Commit**

```bash
git add agent_worker/main.py
git commit -m "feat: silero VAD, noise cancellation, user_away_timeout, prewarm"
```

---

### Task 4: Go backend — cache clients, add DeleteRoom comment

**Files:**
- Modify: `pkg/livekit/client.go`

- [ ] **Step 1: Update Client struct and NewClient**

In `pkg/livekit/client.go`, replace the `Client` struct and `NewClient` function:

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
		roomSvc:   lksdk.NewRoomServiceClient(host, apiKey, apiSecret),
		dispSvc:   lksdk.NewAgentDispatchServiceClient(host, apiKey, apiSecret),
	}
}
```

- [ ] **Step 2: Simplify RoomClient and DispatchClient methods**

Replace both methods to return the cached field:

```go
func (c *Client) RoomClient() *lksdk.RoomServiceClient {
	return c.roomSvc
}

func (c *Client) DispatchClient() *lksdk.AgentDispatchClient {
	return c.dispSvc
}
```

- [ ] **Step 3: Add comment on DeleteRoom in CreateRoomWithAgent**

Add explanatory comment before the `DeleteRoom` call in `CreateRoomWithAgent`:

```go
// Delete stale room to prevent dispatch accumulation.
// CreateDispatch is not idempotent — calling it on an existing
// room adds another dispatch, spawning a duplicate agent.
_ = c.DeleteRoom(ctx, roomName)
```

- [ ] **Step 4: Verify Go compiles**

Run: `cd /home/eddiesngu/Desktop/Dang/aiseebackend && go build ./...`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add pkg/livekit/client.go
git commit -m "perf: eager-init LiveKit clients, document DeleteRoom intent"
```

---

### Task 5: Smoke test — Docker build

**Files:** None (verification only)

- [ ] **Step 1: Build agent worker Docker image**

Run: `cd /home/eddiesngu/Desktop/Dang/aiseebackend && docker compose build agent`
Expected: Build succeeds, all pip dependencies install.

- [ ] **Step 2: Build Go backend Docker image**

Run: `docker compose build backend`
Expected: Build succeeds, `go build` completes.

- [ ] **Step 3: Verify services start**

Run: `docker compose up -d && docker compose logs agent --tail=20`
Expected: Agent worker starts, prewarm loads Silero model, registers with LiveKit.
Look for: `"published state: initializing"` or similar startup log.

Run: `docker compose down` after verification.
