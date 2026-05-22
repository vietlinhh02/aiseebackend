"""Top-level Google SDK bridge lifecycle.

This module is intentionally small in the first bridge PR. The old LiveKit
plugin path remains the default engine while this class becomes the stable
entrypoint for the Google SDK implementation.
"""

from __future__ import annotations

import asyncio
import json
import logging
from contextlib import suppress
from dataclasses import dataclass
from datetime import datetime, timezone
from logging import Logger

from backend import BackendClient
from config import (
    AI_COST_PRICING_VERSION,
    GEMINI_RESPONSE_AUDIO_SAMPLE_RATE,
    GEMINI_VIDEO_ENABLED,
    GEMINI_VIDEO_FPS_IDLE,
    GO_BACKEND_URL,
    INTERNAL_API_KEY,
)
from livekit import rtc
from prompts import select_prompt

from google.genai import types
from .core import GeminiEventHandler, GeminiLiveSettings, send_wake_greeting
from .exa_search import ExaSearchClient
from .livekit_io import (
    AgentAudioPublisher,
    LiveKitDeviceCommandChannel,
    forward_audio_track,
    forward_video_track,
)
from .nsfw_filter import NsfwFilter
from .state import AgentState, publish_agent_state
from .tool_router import ToolRouter

logger = logging.getLogger("aisee-agent")


@dataclass(slots=True)
class GoogleSDKBridge:
    ctx: object
    logger: Logger
    settings: GeminiLiveSettings | None = None

    async def run(self) -> None:
        settings = self.settings or GeminiLiveSettings.from_env()
        room = self.ctx.room
        self.logger.info(
            "connecting to room %s with google_sdk bridge model=%s",
            room.name,
            settings.model,
        )
        await self.ctx.connect()
        local = room.local_participant
        await publish_agent_state(local, AgentState.INITIALIZING, self.logger)

        await asyncio.sleep(0.35)
        if self._has_duplicate_agent(room):
            self.logger.info("duplicate agent detected, exiting google_sdk bridge")
            return

        user_context = self._resolve_user_context(room)
        backend = BackendClient(GO_BACKEND_URL, INTERNAL_API_KEY)
        router = ToolRouter(
            backend=backend,
            device_channel=LiveKitDeviceCommandChannel(
                local_participant=local,
                destination_identity=user_context.get("user_identity") or user_context.get("user_id", ""),
            ),
            search_client=ExaSearchClient.from_env(),
            user_context=user_context,
        )
        audio_publisher = AgentAudioPublisher(
            local_participant=local,
            sample_rate=GEMINI_RESPONSE_AUDIO_SAMPLE_RATE,
        )
        await audio_publisher.start()

        nsfw_filter = _load_nsfw_filter(self.logger)

        disconnect_event = asyncio.Event()
        duplicate_event = asyncio.Event()
        wake_event = asyncio.Event()
        track_tasks: set[asyncio.Task] = set()
        ai_usage_started_at: str | None = None
        ai_usage_started_time: datetime | None = None
        ai_usage_source_id = room.name
        active_gemini_session = {"session": None}
        usage_metadata_events: list[dict] = []

        @local.register_rpc_method("wake_conversation_ready")
        async def _wake_conversation_ready(data) -> str:
            self.logger.info(
                "wake_conversation_ready from identity=%s",
                getattr(data, "caller_identity", ""),
            )
            wake_event.set()
            return json.dumps({"status": "ok", "action": "wake_greeting_queued"})

        @room.on("participant_connected")
        def _on_participant_connected(participant):
            if participant.kind == rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
                duplicate_event.set()
            else:
                self._read_user_attributes(user_context, participant)

        @room.on("participant_attributes_changed")
        def _on_attributes(changed_attributes, participant):
            if participant.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
                self._read_user_attributes(user_context, participant)

        @room.on("disconnected")
        def _on_disconnect():
            disconnect_event.set()

        user_was_speaking = False

        @room.on("active_speakers_changed")
        def _on_active_speakers_changed(speakers):
            nonlocal user_was_speaking
            user_speaking = any(
                p.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT
                for p in speakers
            )
            if user_speaking != user_was_speaking:
                user_was_speaking = user_speaking
                session = active_gemini_session.get("session")
                if session is not None:
                    if user_speaking:
                        self.logger.info("User started speaking (LiveKit active speaker VAD) -> sending activity_start")
                        asyncio.create_task(
                            session.send_realtime_input(activity_start=types.ActivityStart())
                        )
                    else:
                        self.logger.info("User stopped speaking (LiveKit active speaker VAD) -> sending activity_end")
                        asyncio.create_task(
                            session.send_realtime_input(activity_end=types.ActivityEnd())
                        )

        client = settings.client()
        instructions = select_prompt(
            user_context.get("locale", "vi"),
            False,
            user_context,
        )
        handler = GeminiEventHandler(tool_router=router, audio_publisher=audio_publisher)
        await publish_agent_state(local, AgentState.IDLE, self.logger)

        try:
            async with client.aio.live.connect(
                model=settings.model,
                config=settings.build_live_config_with_prompt(instructions),
            ) as gemini_session:
                active_gemini_session["session"] = gemini_session
                ai_usage_started_time = datetime.now(timezone.utc)
                ai_usage_started_at = ai_usage_started_time.isoformat()
                if user_context.get("user_id"):
                    try:
                        await backend.start_usage_session(
                            user_context["user_id"],
                            "ai_minutes",
                            "minutes",
                            "agent",
                            ai_usage_source_id,
                            ai_usage_started_at,
                            {"room_name": room.name, "engine": "google_sdk"},
                        )
                    except Exception as exc:
                        self.logger.warning("start ai usage session failed: %s", exc)
                self._wire_track_subscription(room, gemini_session, settings, track_tasks, nsfw_filter)
                receive_task = asyncio.create_task(
                    self._receive_loop(
                        gemini_session,
                        handler,
                        local,
                        usage_metadata_events,
                    )
                )
                wake_task = asyncio.create_task(
                    self._wake_greeting_loop(wake_event, active_gemini_session, local)
                )
                wait_tasks = [
                    asyncio.create_task(disconnect_event.wait()),
                    asyncio.create_task(duplicate_event.wait()),
                    receive_task,
                    wake_task,
                ]
                done, pending = await asyncio.wait(
                    wait_tasks,
                    return_when=asyncio.FIRST_COMPLETED,
                )
                for task in pending:
                    task.cancel()
                for task in pending:
                    with suppress(asyncio.CancelledError):
                        await task
                for task in done:
                    if task is receive_task:
                        task.result()
                    elif task is wake_task:
                        task.result()
        except Exception:
            await publish_agent_state(local, AgentState.ERROR, self.logger)
            raise
        finally:
            ended_at_time = datetime.now(timezone.utc)
            ended_at = ended_at_time.isoformat()
            active_gemini_session["session"] = None
            await cancel_track_tasks(track_tasks)
            if user_context.get("user_id") and ai_usage_started_at:
                try:
                    await backend.commit_usage_session(
                        user_context["user_id"],
                        "ai_minutes",
                        "minutes",
                        "agent",
                        ai_usage_source_id,
                        ai_usage_started_at,
                        ended_at,
                        {"room_name": room.name, "engine": "google_sdk"},
                    )
                except Exception as exc:
                    self.logger.warning("commit ai usage session failed: %s", exc)
                try:
                    await self._record_ai_cost_event(
                        backend=backend,
                        user_id=user_context["user_id"],
                        model=settings.model,
                        source_id=ai_usage_source_id,
                        started_at=ai_usage_started_time,
                        ended_at=ended_at_time,
                        occurred_at=ended_at,
                        usage_metadata_events=usage_metadata_events,
                    )
                except Exception as exc:
                    self.logger.warning("record ai cost event failed: %s", exc)
            await publish_agent_state(local, AgentState.DISCONNECTED, self.logger)
            await audio_publisher.close()
            await backend.close()

    async def _wake_greeting_loop(self, wake_event, active_gemini_session: dict, local) -> None:
        while True:
            await wake_event.wait()
            wake_event.clear()
            session = active_gemini_session.get("session")
            if session is None:
                wake_event.set()
                await asyncio.sleep(0.1)
                continue
            await publish_agent_state(local, AgentState.THINKING, self.logger)
            self.logger.info("sending wake greeting to Gemini")
            await send_wake_greeting(session)

    async def _receive_loop(
        self,
        gemini_session,
        handler: "GeminiEventHandler",
        local,
        usage_metadata_events: list[dict] | None = None,
    ) -> None:
        if usage_metadata_events is None:
            usage_metadata_events = []
        while True:
            saw_message = False
            async for message in gemini_session.receive():
                saw_message = True
                await handler.handle(gemini_session, message)
                metadata = _usage_metadata_from_message(message)
                if metadata:
                    usage_metadata_events.append(metadata)
                server_content = getattr(message, "server_content", None)
                if server_content and getattr(server_content, "turn_complete", False):
                    await publish_agent_state(local, AgentState.IDLE, self.logger)
            if not saw_message:
                return

    async def _record_ai_cost_event(
        self,
        *,
        backend: BackendClient,
        user_id: str,
        model: str,
        source_id: str,
        started_at: datetime | None,
        ended_at: datetime,
        occurred_at: str,
        usage_metadata_events: list[dict],
    ) -> None:
        duration_seconds = max(
            0.0,
            (ended_at - started_at).total_seconds() if started_at else 0.0,
        )
        token_totals = _sum_token_usage(usage_metadata_events)
        audio_input_seconds = _number_from_metadata(
            usage_metadata_events,
            "audio_input_seconds",
        ) or duration_seconds
        audio_output_seconds = _number_from_metadata(
            usage_metadata_events,
            "audio_output_seconds",
        )
        metadata = {
            "engine": "google_sdk",
            "room_name": source_id,
            "duration_seconds": duration_seconds,
            "raw_usage_metadata": usage_metadata_events,
        }
        await backend.record_ai_cost(
            user_id=user_id,
            provider="google",
            model=model,
            modality="live_audio",
            source="agent",
            source_id=source_id,
            input_tokens=token_totals["input_tokens"],
            output_tokens=token_totals["output_tokens"],
            audio_input_seconds=audio_input_seconds,
            audio_output_seconds=audio_output_seconds,
            cost_usd=0,
            cost_vnd=0,
            estimated=len(usage_metadata_events) == 0,
            pricing_version=AI_COST_PRICING_VERSION,
            occurred_at=occurred_at,
            metadata=metadata,
        )

    def _wire_track_subscription(self, room, gemini_session, settings, track_tasks: set[asyncio.Task], nsfw_filter: NsfwFilter | None = None) -> None:
        def _start_track(track, participant):
            if participant.kind == rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
                return
            if track.kind == rtc.TrackKind.KIND_AUDIO:
                task = asyncio.create_task(
                    forward_audio_track(track, gemini_session, settings, self.logger)
                )
                track_tasks.add(task)
                task.add_done_callback(track_tasks.discard)
            elif track.kind == rtc.TrackKind.KIND_VIDEO and GEMINI_VIDEO_ENABLED:
                task = asyncio.create_task(
                    forward_video_track(
                        track,
                        gemini_session,
                        GEMINI_VIDEO_FPS_IDLE,
                        self.logger,
                        nsfw_filter=nsfw_filter,
                    )
                )
                track_tasks.add(task)
                task.add_done_callback(track_tasks.discard)

        @room.on("track_subscribed")
        def _on_track(track, publication, participant):
            _start_track(track, participant)

        for participant in room.remote_participants.values():
            for pub in participant.track_publications.values():
                track = pub.track
                if track:
                    _start_track(track, participant)

    def _resolve_user_context(self, room) -> dict:
        user_id = ""
        user_identity = ""
        user_context = {
            "user_id": "",
            "user_name": "",
            "locale": "vi",
            "accessibility": "low_vision",
            "has_family_link": "false",
        }
        for participant in room.remote_participants.values():
            if participant.kind != rtc.ParticipantKind.PARTICIPANT_KIND_AGENT:
                user_id = participant.identity
                user_identity = participant.identity
                self._read_user_attributes(user_context, participant)
                break
        if not user_id and room.name.startswith("user-"):
            user_id = room.name.replace("user-", "")
            user_identity = user_id
        user_context["user_id"] = user_context.get("user_id") or user_id
        user_context["user_identity"] = user_identity or user_context["user_id"]
        return user_context

    def _read_user_attributes(self, user_context: dict, participant) -> None:
        for key in (
            "user_name",
            "user_id",
            "locale",
            "accessibility",
            "has_family_link",
        ):
            val = participant.attributes.get(key)
            if val:
                user_context[key] = val
        if participant.identity:
            user_context["user_identity"] = participant.identity

    def _has_duplicate_agent(self, room) -> bool:
        return any(
            participant.kind == rtc.ParticipantKind.PARTICIPANT_KIND_AGENT
            for participant in room.remote_participants.values()
        )


async def cancel_track_tasks(track_tasks: set[asyncio.Task]) -> None:
    tasks = list(track_tasks)
    for task in tasks:
        task.cancel()
    for task in tasks:
        with suppress(asyncio.CancelledError):
            await task


def _usage_metadata_from_message(message) -> dict:
    for attr in ("usage_metadata", "response_usage_metadata"):
        metadata = getattr(message, attr, None)
        if metadata:
            return _jsonable_metadata(metadata)
    server_content = getattr(message, "server_content", None)
    if server_content is not None:
        metadata = getattr(server_content, "usage_metadata", None)
        if metadata:
            return _jsonable_metadata(metadata)
    return {}


def _jsonable_metadata(value) -> dict:
    if isinstance(value, dict):
        return dict(value)
    for method in ("to_json_dict", "model_dump"):
        fn = getattr(value, method, None)
        if callable(fn):
            try:
                result = fn()
                return result if isinstance(result, dict) else {}
            except Exception:
                return {}
    try:
        encoded = json.loads(json.dumps(value, default=lambda obj: getattr(obj, "__dict__", str(obj))))
        return encoded if isinstance(encoded, dict) else {}
    except Exception:
        return {}


def _sum_token_usage(events: list[dict]) -> dict[str, int]:
    input_tokens = 0
    output_tokens = 0
    for event in events:
        input_tokens += int(
            event.get("prompt_token_count")
            or event.get("input_token_count")
            or event.get("input_tokens")
            or 0
        )
        output_tokens += int(
            event.get("candidates_token_count")
            or event.get("output_token_count")
            or event.get("output_tokens")
            or 0
        )
    return {"input_tokens": input_tokens, "output_tokens": output_tokens}


def _load_nsfw_filter(log: Logger) -> NsfwFilter | None:
    try:
        return NsfwFilter()
    except Exception as exc:
        log.warning("nsfw filter unavailable, skipping server-side guard: %s", exc)
        return None


def _number_from_metadata(events: list[dict], key: str) -> float:
    total = 0.0
    for event in events:
        value = event.get(key)
        if isinstance(value, (int, float)):
            total += float(value)
    return total
