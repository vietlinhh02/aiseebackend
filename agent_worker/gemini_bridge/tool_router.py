"""Gemini function-call router skeleton."""

from __future__ import annotations

import logging
import time
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any

from .transport import DeviceCommandChannel

logger = logging.getLogger("aisee-agent")

MUSIC_DISABLED_RESULT = {
    "status": "disabled",
    "message": "music playback is temporarily disabled",
}
ToolHandler = Callable[[dict[str, Any]], Awaitable[dict[str, Any]]]


@dataclass(slots=True)
class ToolRouter:
    backend: object
    user_context: dict
    device_channel: DeviceCommandChannel | None = None
    local_participant: object | None = None
    search_client: object | None = None
    _handlers: dict[str, ToolHandler] = field(init=False, repr=False)

    def __post_init__(self) -> None:
        if self.device_channel is None and self.local_participant is not None:
            from .livekit_io import LiveKitDeviceCommandChannel

            self.device_channel = LiveKitDeviceCommandChannel(
                local_participant=self.local_participant,
                destination_identity=self._user_identity(),
            )
        self._handlers = self._build_handlers()

    async def call(self, name: str, args: dict | None) -> dict[str, Any]:
        args = self._normalize_args(args)
        started = time.perf_counter()
        logger.info("tool_call start name=%s arg_keys=%s", name, sorted(args.keys()))
        try:
            handler = self._handlers.get(name)
            if handler is None:
                result = {"status": "error", "error": f"unknown tool: {name}"}
            else:
                result = self._coerce_result(await handler(args))
            logger.info(
                "tool_call ok name=%s status=%s elapsed_ms=%d",
                name,
                result.get("status"),
                int((time.perf_counter() - started) * 1000),
            )
            return result
        except Exception as exc:
            logger.warning(
                "tool_call failed name=%s elapsed_ms=%d error=%s",
                name,
                int((time.perf_counter() - started) * 1000),
                exc,
            )
            return {"status": "error", "error": str(exc)}

    def _build_handlers(self) -> dict[str, ToolHandler]:
        handlers: dict[str, ToolHandler] = {
            tool_name: self._backend_handler(tool_name)
            for tool_name in BACKEND_TOOLS
        }
        handlers.update(
            {
                tool_name: self._backend_handler(tool_name, timeout)
                for tool_name, timeout in BACKEND_TOOL_TIMEOUTS.items()
            }
        )
        handlers.update(
            {
                "activate_camera": self._activate_camera,
                "deactivate_camera": self._deactivate_camera,
                "set_medication_reminder": self._schedule_handler(
                    "set_medication_reminder"
                ),
                "set_appointment_reminder": self._schedule_handler(
                    "set_appointment_reminder"
                ),
                "emergency_alert": self._emergency_alert,
                "control_music": self._music_disabled,
                "navigate_to": self._navigate_to,
                "web_search": self._web_search,
                "get_location": self._get_location,
                "get_current_location": self._get_location,
                "update_location": self._update_location,
            }
        )
        return handlers

    def _backend_handler(
        self,
        name: str,
        timeout: float | None = None,
    ) -> ToolHandler:
        async def handler(args: dict[str, Any]) -> dict[str, Any]:
            return await self._backend_tool(name, args, timeout)

        return handler

    def _schedule_handler(self, name: str) -> ToolHandler:
        async def handler(args: dict[str, Any]) -> dict[str, Any]:
            return await self._backend_with_schedule_action(name, args)

        return handler

    @staticmethod
    def _normalize_args(args: dict | None) -> dict[str, Any]:
        if isinstance(args, dict):
            return dict(args)
        return {}

    @staticmethod
    def _coerce_result(result: Any) -> dict[str, Any]:
        if isinstance(result, dict):
            return result
        return {"status": "ok", "result": result}

    def _uid(self) -> str:
        return self.user_context.get("user_id", "")

    def _user_identity(self) -> str:
        return self.user_context.get("user_identity") or self._uid()

    async def _backend_tool(
        self,
        name: str,
        args: dict[str, Any],
        timeout: float | None = None,
    ) -> dict[str, Any]:
        if timeout is None:
            return await self.backend.tool_call(name, self._uid(), args)
        return await self.backend.tool_call_with_timeout(name, self._uid(), args, timeout=timeout)

    async def _device_request(
        self,
        method: str,
        payload: dict[str, Any],
        timeout: float | None = None,
    ) -> dict[str, Any]:
        if self.device_channel is None:
            raise RuntimeError(f"device command channel is not configured for {method}")
        return await self.device_channel.request(method, payload, timeout=timeout)

    async def _activate_camera(self, args: dict[str, Any]) -> dict[str, Any]:
        reason = args.get("reason", "User requested")
        source_id = f"camera-{uuid.uuid4()}"
        await self.backend.check_quota(self._uid(), "camera_activations", 1)
        result = await self._device_request("enable_camera", {}, timeout=8.0)
        await self.backend.record_usage(
            self._uid(),
            "camera_activations",
            1,
            "activations",
            "agent",
            source_id,
            {"reason": reason},
        )
        return result

    async def _deactivate_camera(self, args: dict[str, Any]) -> dict[str, Any]:
        return await self._device_request("disable_camera", {}, timeout=5.0)

    async def _backend_with_schedule_action(
        self,
        name: str,
        args: dict[str, Any],
    ) -> dict[str, Any]:
        result = await self.backend.tool_call(name, self._uid(), args)
        result = dict(result)
        reminder_action = result.pop("schedule_action", None)
        if reminder_action and isinstance(reminder_action, dict):
            try:
                await self._device_request("schedule_reminder", reminder_action.get("params", {}))
            except Exception as exc:
                logger.warning("schedule_reminder RPC failed: %s", exc)
        return result

    async def _emergency_alert(self, args: dict[str, Any]) -> dict[str, Any]:
        try:
            location = await self._device_request("get_location", {}, timeout=15.0)
            lat = location.get("lat", 0)
            lng = location.get("lng", 0)
        except Exception:
            logger.warning("get_location RPC failed, using unknown location")
            lat, lng = 0, 0
        return await self.backend.tool_call_with_timeout(
            "emergency_alert",
            self._uid(),
            {"lat": lat, "lng": lng},
            timeout=20.0,
        )

    async def _music_disabled(self, args: dict[str, Any]) -> dict[str, Any]:
        return dict(MUSIC_DISABLED_RESULT)

    async def _navigate_to(self, args: dict[str, Any]) -> dict[str, Any]:
        destination = args.get("destination", "")
        source_id = f"navigation-{uuid.uuid4()}"
        await self.backend.check_quota(self._uid(), "navigation_sessions", 1)
        result = await self._device_request("open_maps", {"destination": destination})
        await self.backend.record_usage(
            self._uid(),
            "navigation_sessions",
            1,
            "sessions",
            "agent",
            source_id,
            {"destination": destination},
        )
        return result

    async def _web_search(self, args: dict[str, Any]) -> dict[str, Any]:
        if self.search_client is None:
            return {"status": "error", "error": "web search is not configured"}
        query = str(args.get("query") or "")
        max_results = args.get("max_results")
        search = getattr(self.search_client, "search")
        return await search(query, max_results=max_results)

    async def _get_location(self, args: dict[str, Any]) -> dict[str, Any]:
        return await self._device_request("get_location", {}, timeout=15.0)

    async def _update_location(self, args: dict[str, Any]) -> dict[str, Any]:
        location = await self._device_request("get_location", {}, timeout=15.0)
        return await self.backend.tool_call(
            "update_location",
            self._uid(),
            {"lat": location.get("lat", 0), "lng": location.get("lng", 0)},
        )


BACKEND_TOOLS = {
    "make_voice_call",
    "make_video_call",
    "send_chat_message",
    "read_unread_messages",
    "list_medication_reminders",
    "mark_medication_taken",
    "mark_medication_skipped",
    "log_health_symptom",
    "log_health_metric",
    "play_music",
    "escalate_to_volunteer",
    "rate_volunteer",
}

BACKEND_TOOL_TIMEOUTS = {
    "escalate_to_family": 40.0,
}
