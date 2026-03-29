import asyncio
import logging
import math
import struct
import time

from livekit import rtc
from livekit.agents import AgentSession, Agent, WorkerOptions, cli
from livekit.agents.voice.room_io import RoomOptions
from livekit.plugins import google

from backend import BackendClient
from config import GEMINI_API_KEY, GEMINI_MODEL, GO_BACKEND_URL, INTERNAL_API_KEY
from data_channel import DataChannelBridge
from prompts import select_prompt
from tools import ALL_TOOLS, init_tools

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("aisee-agent")

IDLE_TIMEOUT = 30
ENERGY_THRESHOLD = 0.02
SPEECH_FRAMES_REQUIRED = 5


async def _publish_state(local_participant, state: str):
    """Publish agent state as participant attribute for frontend UI."""
    try:
        await local_participant.set_attributes({'lk.agent.state': state})
        logger.info("published state: %s", state)
    except Exception as exc:
        logger.warning("failed to publish state %s: %s", state, exc)


class AiSeeAgent(Agent):
    def __init__(self, tools, locale: str = 'vi', wake_up: bool = False):
        super().__init__(
            instructions=select_prompt(locale, wake_up),
            tools=tools,
        )


def _compute_rms(data: bytes) -> float:
    """Compute RMS energy of 16-bit PCM audio."""
    n_samples = len(data) // 2
    if n_samples == 0:
        return 0.0
    samples = struct.unpack(f"<{n_samples}h", data[: n_samples * 2])
    return math.sqrt(sum(s * s for s in samples) / n_samples) / 32768.0


async def entrypoint(ctx: AgentSession):
    logger.info("connecting to room %s", ctx.room.name)
    await ctx.connect()
    logger.info("connected to room %s", ctx.room.name)

    # ── Debug: log all participants ────────────────────────
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
    logger.info(
        "total participants: %d remote + 1 local",
        len(ctx.room.remote_participants),
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

    bridge = DataChannelBridge()
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
        """Read user attributes from participant into user_context."""
        for key in ('user_name', 'user_id', 'locale', 'accessibility', 'has_family_link'):
            val = participant.attributes.get(key)
            if val:
                user_context[key] = val
        logger.info("user_context updated: %s", user_context)

    # Read from existing participants (user may have connected first)
    for p in ctx.room.remote_participants.values():
        if p.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
            _read_user_attributes(p)

    @ctx.room.on("participant_attributes_changed")
    def _on_attributes(changed_attributes, participant):
        if participant.kind == rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
            return
        _read_user_attributes(participant)

    @ctx.room.on("data_received")
    def on_data(data_packet, *_args, **_kwargs):
        raw = (
            data_packet.data
            if hasattr(data_packet, "data")
            else data_packet
        )
        bridge.on_data_received(raw)

    init_tools(
        backend=backend,
        bridge=bridge,
        local_participant=ctx.room.local_participant,
        user_id=user_id,
    )

    # ── Lazy session state ──────────────────────────────────
    active_session: AgentSession | None = None
    last_activity = 0.0
    first_start = True

    async def _start_session():
        nonlocal active_session, last_activity, first_start
        if active_session is not None:
            return

        wake_up = not first_start
        first_start = False
        last_activity = time.monotonic()

        logger.info(
            "starting Gemini session (wake_up=%s) "
            "current_tracks=%d",
            wake_up,
            len(ctx.room.local_participant.track_publications),
        )

        agent = AiSeeAgent(
            tools=ALL_TOOLS,
            locale=user_context.get('locale', 'vi'),
            wake_up=wake_up,
        )
        active_session = AgentSession(
            llm=google.realtime.RealtimeModel(
                model=GEMINI_MODEL,
                api_key=GEMINI_API_KEY,
                voice="Aoede",
            ),
        )

        await active_session.start(
            agent=agent,
            room=ctx.room,
            room_options=RoomOptions(video_input=True),
        )

    async def _stop_session():
        nonlocal active_session
        if active_session is None:
            return
        logger.info("idle timeout — closing Gemini session")
        await _publish_state(local, 'idle')
        await active_session.aclose()
        active_session = None

        # Unpublish all agent audio tracks to prevent ghost speakers.
        local = ctx.room.local_participant
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
        nonlocal last_activity
        last_activity = time.monotonic()
        if active_session is None:
            asyncio.create_task(_start_session())

    # ── VAD on user audio track ─────────────────────────────
    async def _vad_loop(track: rtc.Track):
        stream = rtc.AudioStream(track)
        consecutive = 0
        async for event in stream:
            rms = _compute_rms(event.frame.data.tobytes())
            if rms > ENERGY_THRESHOLD:
                consecutive += 1
                if consecutive >= SPEECH_FRAMES_REQUIRED:
                    _on_speech()
                    consecutive = 0
            else:
                consecutive = max(0, consecutive - 1)

    @ctx.room.on("track_subscribed")
    def on_track(track, publication, participant):
        logger.info(
            "track_subscribed: kind=%s participant=%s",
            track.kind, participant.identity,
        )
        if track.kind == rtc.TrackKind.KIND_AUDIO:
            asyncio.create_task(_vad_loop(track))

    # Handle tracks already subscribed before handler registered
    for participant in ctx.room.remote_participants.values():
        for pub in participant.track_publications.values():
            track = pub.track
            if track and track.kind == rtc.TrackKind.KIND_AUDIO:
                asyncio.create_task(_vad_loop(track))

    # ── Idle monitor ────────────────────────────────────────
    async def _idle_monitor():
        while True:
            await asyncio.sleep(5)
            if active_session and last_activity:
                elapsed = time.monotonic() - last_activity
                if elapsed > IDLE_TIMEOUT:
                    await _stop_session()

    asyncio.create_task(_idle_monitor())

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


if __name__ == "__main__":
    cli.run_app(
        WorkerOptions(
            entrypoint_fnc=entrypoint,
            agent_name="aisee-agent",
        )
    )
