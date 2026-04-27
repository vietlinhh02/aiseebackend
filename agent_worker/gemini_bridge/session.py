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
    GEMINI_RESPONSE_AUDIO_SAMPLE_RATE,
    GEMINI_VIDEO_ENABLED,
    GEMINI_VIDEO_FPS_IDLE,
    GO_BACKEND_URL,
    INTERNAL_API_KEY,
)
from livekit import rtc
from prompts import select_prompt

from .core import GeminiEventHandler, GeminiLiveSettings, send_wake_greeting
from .exa_search import ExaSearchClient
from .livekit_io import (
    AgentAudioPublisher,
    LiveKitDeviceCommandChannel,
    forward_audio_track,
    forward_video_track,
)
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

        disconnect_event = asyncio.Event()
        duplicate_event = asyncio.Event()
        wake_event = asyncio.Event()
        track_tasks: set[asyncio.Task] = set()
        ai_usage_started_at: str | None = None
        ai_usage_source_id = room.name
        active_gemini_session = {"session": None}

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
                ai_usage_started_at = datetime.now(timezone.utc).isoformat()
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
                self._wire_track_subscription(room, gemini_session, settings, track_tasks)
                receive_task = asyncio.create_task(
                    self._receive_loop(gemini_session, handler, local)
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
            ended_at = datetime.now(timezone.utc).isoformat()
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

    async def _receive_loop(self, gemini_session, handler: "GeminiEventHandler", local) -> None:
        while True:
            saw_message = False
            async for message in gemini_session.receive():
                saw_message = True
                await handler.handle(gemini_session, message)
                server_content = getattr(message, "server_content", None)
                if server_content and getattr(server_content, "turn_complete", False):
                    await publish_agent_state(local, AgentState.IDLE, self.logger)
            if not saw_message:
                return

    def _wire_track_subscription(self, room, gemini_session, settings, track_tasks: set[asyncio.Task]) -> None:
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
                    forward_video_track(track, gemini_session, GEMINI_VIDEO_FPS_IDLE, self.logger)
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
