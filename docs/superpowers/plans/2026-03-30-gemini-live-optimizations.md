# Gemini Live API Optimizations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Optimize agent_worker for Phase 4 demo — inject user context, video-on-demand, session resumption, VAD tuning, and thinking level.

**Architecture:** All changes are in `agent_worker/`. Prompt changes in `prompts.py`, session/model config in `main.py`, tool callbacks in `tools.py`. No Go backend or DB changes.

**Tech Stack:** Python 3.13, livekit-agents, livekit-plugins-google (`google.realtime.RealtimeModel`), google.genai.types

**Spec:** `docs/superpowers/specs/2026-03-30-gemini-live-optimizations-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `agent_worker/prompts.py` | Modify | Add user context block to `select_prompt()` |
| `agent_worker/tools.py` | Modify | Pass user_context to prompt, add session_ctrl for camera restart |
| `agent_worker/main.py` | Modify | Video-on-demand, session resumption, VAD config, thinking config |

---

### Task 1: User context injection into system prompt

**Files:**
- Modify: `agent_worker/prompts.py:234-242` (select_prompt function)
- Modify: `agent_worker/tools.py:15-27` (AiSeeAgent.__init__)

- [ ] **Step 1: Add user context formatting to `select_prompt()`**

In `agent_worker/prompts.py`, change `select_prompt` to accept `user_context` and append a context block:

```python
def select_prompt(
    locale: str,
    wake_up: bool,
    user_context: dict | None = None,
) -> str:
    """Select system prompt based on locale and wake_up state."""
    if locale.startswith('vi'):
        base = SYSTEM_PROMPT_VI
        suffix = WAKEUP_SUFFIX_VI if wake_up else GREETING_SUFFIX_VI
    else:
        base = SYSTEM_PROMPT_EN
        suffix = WAKEUP_SUFFIX_EN if wake_up else GREETING_SUFFIX_EN

    prompt = base + suffix

    if user_context:
        prompt += _format_user_context(locale, user_context)

    return prompt


def _format_user_context(
    locale: str, ctx: dict,
) -> str:
    """Append user info block so the model knows who it's talking to."""
    name = ctx.get('user_name', '')
    access = ctx.get('accessibility', 'low_vision')
    family = ctx.get('has_family_link', 'false')

    if locale.startswith('vi'):
        return (
            "\n=== THÔNG TIN NGƯỜI DÙNG ===\n"
            f"- Tên: {name}\n"
            f"- Khả năng thị lực: {access}\n"
            f"- Đã liên kết người thân: {family}\n\n"
            "Dùng tên người dùng khi trò chuyện. "
            "Điều chỉnh mô tả theo khả năng thị lực.\n"
            "Nếu chưa liên kết người thân, "
            "không đề xuất gọi người thân hay SOS.\n"
        )
    return (
        "\n=== USER INFO ===\n"
        f"- Name: {name}\n"
        f"- Vision level: {access}\n"
        f"- Family linked: {family}\n\n"
        "Use the user's name in conversation. "
        "Adjust descriptions to their vision level.\n"
        "If no family is linked, "
        "do not suggest calling family or SOS.\n"
    )
```

- [ ] **Step 2: Pass user_context from AiSeeAgent to select_prompt**

In `agent_worker/tools.py:27`, change the `super().__init__` call:

```python
super().__init__(
    instructions=select_prompt(locale, wake_up, user_context),
)
```

- [ ] **Step 3: Verify locally**

Run: `cd agent_worker && python -c "from prompts import select_prompt; p = select_prompt('vi', False, {'user_name': 'Eddie', 'accessibility': 'low_vision', 'has_family_link': 'true'}); print(p[-300:])"`

Expected: The last ~300 chars should contain the `=== THÔNG TIN NGƯỜI DÙNG ===` block with Eddie's info.

- [ ] **Step 4: Commit**

```bash
git add agent_worker/prompts.py agent_worker/tools.py
git commit -m "feat: inject user context into Gemini system prompt

select_prompt() now accepts user_context dict and appends
user name, accessibility level, and family link status so the
model can personalize responses."
```

---

### Task 2: thinkingLevel and VAD sensitivity config

**Files:**
- Modify: `agent_worker/main.py:208-213` (RealtimeModel constructor)

- [ ] **Step 1: Add google.genai.types import**

At the top of `agent_worker/main.py`, add:

```python
from google.genai import types as gemini_types
```

- [ ] **Step 2: Update RealtimeModel constructor with thinking + VAD config**

Replace the `RealtimeModel(...)` block in `_start_session()` (lines 209-213):

```python
llm=google.realtime.RealtimeModel(
    model=GEMINI_MODEL,
    api_key=GEMINI_API_KEY,
    voice="Aoede",
    thinking_config=gemini_types.ThinkingConfig(
        thinking_level=gemini_types.ThinkingLevel.LOW,
    ),
    realtime_input_config=gemini_types.RealtimeInputConfig(
        automatic_activity_detection=(
            gemini_types.AutomaticActivityDetection(
                start_of_speech_sensitivity=(
                    gemini_types.StartSensitivity
                    .START_SENSITIVITY_HIGH
                ),
                end_of_speech_sensitivity=(
                    gemini_types.EndSensitivity
                    .END_SENSITIVITY_LOW
                ),
                prefix_padding_ms=100,
                silence_duration_ms=500,
            )
        ),
    ),
),
```

- [ ] **Step 3: Verify import resolves**

Run: `cd agent_worker && python -c "from google.genai import types as gemini_types; print(gemini_types.ThinkingLevel.LOW); print(gemini_types.StartSensitivity.START_SENSITIVITY_HIGH); print(gemini_types.EndSensitivity.END_SENSITIVITY_LOW)"`

Expected: Prints three enum values without errors.

- [ ] **Step 4: Commit**

```bash
git add agent_worker/main.py
git commit -m "feat: add thinking_config LOW and VAD sensitivity tuning

Set ThinkingLevel.LOW for better image descriptions and multi-step
reasoning. Configure VAD with END_SENSITIVITY_LOW and 500ms silence
duration to avoid cutting off hesitant speakers."
```

---

### Task 3: Video-on-demand — session restart on camera toggle

**Files:**
- Modify: `agent_worker/main.py:175-270` (session lifecycle)
- Modify: `agent_worker/tools.py:15-27,77-103` (AiSeeAgent init + camera tools)

- [ ] **Step 1: Add `video_enabled` flag and `_restart_session()` in main.py**

In the lazy session state block (after line 183), add:

```python
video_enabled = False
```

Add `_restart_session()` function after `_stop_session()`:

```python
async def _restart_session():
    """Stop and restart session (e.g. after video mode change)."""
    logger.info(
        "restarting session (video_enabled=%s)",
        video_enabled,
    )
    await _stop_session()
    await _start_session()
```

- [ ] **Step 2: Use `video_enabled` in `_start_session()` RoomOptions**

Change `RoomOptions(video_input=True,` to:

```python
room_options=RoomOptions(
    video_input=video_enabled,
    audio_input=AudioInputOptions(
        noise_cancellation=noise_cancellation.BVC(),
    ),
),
```

- [ ] **Step 3: Add `session_ctrl` parameter to AiSeeAgent**

In `agent_worker/tools.py`, update `__init__`:

```python
class AiSeeAgent(Agent):
    def __init__(
        self,
        backend: BackendClient,
        local_participant: rtc.LocalParticipant,
        user_context: dict,
        locale: str = 'vi',
        wake_up: bool = False,
        session_ctrl: dict | None = None,
    ):
        self._backend = backend
        self._lp = local_participant
        self._uctx = user_context
        self._session_ctrl = session_ctrl or {}
        super().__init__(
            instructions=select_prompt(locale, wake_up, user_context),
        )
```

- [ ] **Step 4: Update `activate_camera` to trigger session restart**

Replace `activate_camera` in `tools.py`:

```python
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
        result = json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}

    restart = self._session_ctrl.get('restart_with_video')
    if restart:
        await restart(True)

    return result
```

- [ ] **Step 5: Update `deactivate_camera` to trigger session restart**

Replace `deactivate_camera` in `tools.py`:

```python
@function_tool()
async def deactivate_camera(self) -> dict:
    """Disable camera, return to audio-only."""
    try:
        response = await self._lp.perform_rpc(
            destination_identity=self._user_identity(),
            method='disable_camera',
            payload='{}',
        )
        result = json.loads(response)
    except Exception as exc:
        return {"status": "error", "error": str(exc)}

    restart = self._session_ctrl.get('restart_with_video')
    if restart:
        await restart(False)

    return result
```

- [ ] **Step 6: Create and pass session_ctrl callback in main.py**

In `_start_session()`, define the callback and pass it to `AiSeeAgent`. Replace the `agent = AiSeeAgent(...)` block:

```python
async def _restart_with_video(enabled: bool):
    nonlocal video_enabled
    video_enabled = enabled
    await _restart_session()

session_ctrl = {
    'restart_with_video': _restart_with_video,
}

agent = AiSeeAgent(
    backend=backend,
    local_participant=ctx.room.local_participant,
    user_context=user_context,
    locale=user_context.get('locale', 'vi'),
    wake_up=wake_up,
    session_ctrl=session_ctrl,
)
```

- [ ] **Step 7: Verify syntax**

Run: `cd agent_worker && python -c "import main; print('OK')"`

Expected: No import errors.

- [ ] **Step 8: Commit**

```bash
git add agent_worker/main.py agent_worker/tools.py
git commit -m "feat: video-on-demand — start audio-only, restart on camera toggle

Sessions start with video_input=False (15-min limit) instead of
True (2-min limit). activate_camera/deactivate_camera trigger
session restart with the new video setting via session_ctrl callback."
```

---

### Task 4: Session resumption via native SessionResumptionConfig

**Files:**
- Modify: `agent_worker/main.py` (session lifecycle, RealtimeModel constructor)

- [ ] **Step 1: Investigate how to access session_resumption_handle from AgentSession**

Run: `cd agent_worker && python -c "
from livekit.agents import AgentSession
import inspect
# Check if AgentSession exposes llm session
members = [m for m in dir(AgentSession) if not m.startswith('__')]
print('AgentSession members:', [m for m in members if 'llm' in m.lower() or 'session' in m.lower() or 'realtime' in m.lower()])
"`

Then explore further based on output. The goal: find a path from `active_session` (AgentSession) to the underlying `RealtimeSession.session_resumption_handle`.

Possible paths:
- `active_session.llm` → RealtimeModel (not session)
- `active_session._llm_session` or similar private attr
- Listen for an event that exposes the handle

Document what you find before proceeding.

- [ ] **Step 2: Add `last_resumption_handle` variable**

In the lazy session state block in `main.py`, add:

```python
last_resumption_handle: str | None = None
```

- [ ] **Step 3: Pass SessionResumptionConfig to RealtimeModel**

Update the `RealtimeModel(...)` constructor in `_start_session()` to include:

```python
session_resumption=gemini_types.SessionResumptionConfig(
    handle=last_resumption_handle,
),
```

Add this after the existing `realtime_input_config` parameter.

- [ ] **Step 4: Capture handle before stopping session**

In `_stop_session()`, before `active_session.aclose()`, capture the handle. The exact code depends on Step 1 findings. Expected pattern:

```python
async def _stop_session():
    nonlocal active_session, last_resumption_handle
    if active_session is None:
        return
    logger.info("idle timeout — closing Gemini session")

    # Capture resumption handle before closing.
    try:
        # Path depends on Step 1 investigation.
        # Example: active_session._realtime_session.session_resumption_handle
        handle = <path_found_in_step_1>
        if handle:
            last_resumption_handle = handle
            logger.info(
                "captured session resumption handle: %s…",
                handle[:20],
            )
    except Exception as exc:
        logger.warning(
            "failed to capture resumption handle: %s", exc,
        )

    await _publish_state(local, 'idle')
    await active_session.aclose()
    active_session = None
    # ... rest of existing cleanup
```

- [ ] **Step 5: Test session resumption flow**

Start the agent, speak to it, wait for idle timeout (30s), speak again. Check logs for:
1. `"captured session resumption handle: ..."` on stop
2. `SessionResumptionConfig(handle=...)` being passed on restart
3. Agent remembering context from previous session (ask "what did I just say?")

If the handle path from Step 1 doesn't work or the handle is always None, this is acceptable degradation — session starts fresh. Log a warning and move on.

- [ ] **Step 6: Commit**

```bash
git add agent_worker/main.py
git commit -m "feat: session resumption via native SessionResumptionConfig

Capture session_resumption_handle before closing and pass it to the
next RealtimeModel instance. Preserves full conversation context
across idle timeouts and video mode restarts."
```

---

### Task 5: Smoke test and final verification

- [ ] **Step 1: Full import check**

Run: `cd agent_worker && python -c "import main; import tools; import prompts; import config; print('All imports OK')"`

Expected: No errors.

- [ ] **Step 2: Run Docker build**

Run: `docker compose build agent_worker`

Expected: Build succeeds.

- [ ] **Step 3: Review all changes**

Run: `git diff main..HEAD --stat` and `git log main..HEAD --oneline`

Verify:
- 4-5 commits, each focused on one optimization
- Only `agent_worker/` files modified (plus docs)
- No unrelated changes

- [ ] **Step 4: Manual test checklist (during demo prep)**

Test each optimization:
1. **User context**: Start session → agent greets user by name
2. **Thinking**: Ask agent to describe a scene → response quality improved
3. **VAD**: Speak slowly with pauses → agent doesn't cut off mid-sentence
4. **Video-on-demand**: Start audio-only → ask to turn on camera → session restarts with video
5. **Session resumption**: Talk, wait 30s, talk again → agent remembers context
