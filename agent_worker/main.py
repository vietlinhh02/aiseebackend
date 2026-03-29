import asyncio
import collections
import logging
import time

from livekit import rtc
from livekit.agents import AgentSession, WorkerOptions, cli, vad as vad_mod
from livekit.agents.voice.io import AudioInput
from livekit.agents.voice.room_io import RoomOptions, AudioInputOptions
from livekit.plugins import google, noise_cancellation, silero

from backend import BackendClient
from config import GEMINI_API_KEY, GEMINI_MODEL, GO_BACKEND_URL, INTERNAL_API_KEY
from tools import AiSeeAgent

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("aisee-agent")

# Max seconds of audio to buffer during session startup.
AUDIO_BUFFER_MAX_S = 5.0


class BufferedAudioInput(AudioInput):
    """Yield buffered frames first, then delegate to the room's audio."""

    def __init__(
        self,
        buffered: list[rtc.AudioFrame],
        source: AudioInput,
    ):
        super().__init__(label="buffered-audio", source=source)
        self._buffered = collections.deque(buffered)
        self._done_buffered = False
        logger.info(
            "BufferedAudioInput: replaying %d buffered frames "
            "before live audio",
            len(buffered),
        )

    async def __anext__(self) -> rtc.AudioFrame:
        if not self._done_buffered:
            if self._buffered:
                return self._buffered.popleft()
            self._done_buffered = True
            logger.info("BufferedAudioInput: buffered frames done, "
                        "switching to live audio")
        return await super().__anext__()


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

    # Audio buffer: collects frames between START_OF_SPEECH and
    # session ready so the agent hears the user's first words.
    audio_buffer: list[rtc.AudioFrame] = []
    buffering = False

    async def _start_session():
        nonlocal active_session, first_start, buffering
        if active_session is not None:
            return

        wake_up = not first_start
        first_start = False
        t0 = time.monotonic()

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

        # Stop buffering — session is now receiving live audio.
        buffering = False
        elapsed_ms = (time.monotonic() - t0) * 1000
        logger.info(
            "Gemini session ready in %.0fms, "
            "buffered %d audio frames",
            elapsed_ms, len(audio_buffer),
        )

        # Swap the session's audio input with a wrapper that
        # replays buffered frames before live audio.
        if audio_buffer and active_session.input.audio:
            original_audio = active_session.input.audio
            active_session.input.audio = BufferedAudioInput(
                buffered=list(audio_buffer),
                source=original_audio,
            )
        audio_buffer.clear()

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
        nonlocal buffering
        if active_session is None:
            buffering = True
            audio_buffer.clear()
            logger.info("speech detected — buffering audio, "
                        "starting session")
            asyncio.create_task(_start_session())

    # ── Silero VAD on user audio track ──────────────────────
    async def _start_vad(track: rtc.Track):
        audio_stream = rtc.AudioStream(track)
        vad_stream = vad_instance.stream()

        async def _push_frames():
            async for event in audio_stream:
                # Buffer frames while session is starting up.
                if buffering:
                    audio_buffer.append(event.frame)
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
