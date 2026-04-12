import asyncio
import json
import logging
import time

from livekit import rtc
from livekit.agents import AgentSession, WorkerOptions, cli, vad as vad_mod
from livekit.rtc.rpc import RpcInvocationData
from livekit.agents.voice.room_io import RoomOptions, AudioInputOptions
from livekit.plugins import google, noise_cancellation, silero

from google.genai import types as gemini_types

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

    # Chờ ngắn để agent thứ hai (nếu có) kịp hiện trong room trước khi thoát.
    await asyncio.sleep(0.35)

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

    # ── Session state ─────────────────────────────────────
    active_session: AgentSession | None = None
    first_start = True
    vad_instance = ctx.proc.userdata["vad"]
    last_resumption_handle: str | None = None

    async def _start_session():
        nonlocal active_session, first_start
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
                thinking_config=gemini_types.ThinkingConfig(
                    thinking_level=gemini_types.ThinkingLevel.LOW,
                ),
                session_resumption=gemini_types.SessionResumptionConfig(
                    handle=last_resumption_handle,
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
            user_away_timeout=30,
        )

        @active_session.on("user_state_changed")
        def _on_user_state(ev):
            if ev.new_state == "away":
                logger.info("user away — closing session (will resume on speech)")
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

        elapsed_ms = (time.monotonic() - t0) * 1000
        logger.info(
            "Gemini session ready in %.0fms", elapsed_ms,
        )

    async def _stop_session():
        nonlocal active_session, last_resumption_handle
        if active_session is None:
            return
        logger.info("closing Gemini session")

        # Capture resumption handle before closing.
        try:
            model = active_session.llm
            if model and hasattr(model, '_sessions'):
                for sess in model._sessions:
                    handle = sess.session_resumption_handle
                    if handle:
                        last_resumption_handle = handle
                        logger.info(
                            "captured session resumption handle:"
                            " %s…",
                            handle[:20],
                        )
                        break
        except Exception as exc:
            logger.warning(
                "failed to capture resumption handle: %s", exc,
            )

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
            logger.info("speech detected — resuming session")
            asyncio.create_task(_start_session())

    @local.register_rpc_method("wake_conversation_ready")
    async def _wake_conversation_ready(data: RpcInvocationData) -> str:
        logger.info(
            "wake_conversation_ready from identity=%s",
            data.caller_identity,
        )
        asyncio.create_task(_start_session())
        return json.dumps({"status": "ok"})

    # ── Lightweight VAD: only detect speech start to resume ──
    async def _run_vad(track: rtc.Track):
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
            asyncio.create_task(_run_vad(track))

    for participant in ctx.room.remote_participants.values():
        for pub in participant.track_publications.values():
            track = pub.track
            if track and track.kind == rtc.TrackKind.KIND_AUDIO:
                asyncio.create_task(_run_vad(track))

    # ── Auto-start first session (no greeting) ───────────
    await _start_session()

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
