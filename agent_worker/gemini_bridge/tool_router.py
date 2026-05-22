"""Gemini function-call router skeleton."""

from __future__ import annotations

import logging
import time
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any

from camera_activation_policy import validate_camera_activation_consent

from .transport import DeviceCommandChannel

logger = logging.getLogger("aisee-agent")
SUPPORT_CALL_BACKEND_TIMEOUT_SECONDS = 45.0
CALL_START_TIMEOUT_PADDING_SECONDS = 10.0

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
                "make_voice_call": self._family_call_alias("voice"),
                "make_video_call": self._family_call_alias("video"),
                "call_family_contact": self._call_family_contact,
                "call_support_contact": self._call_support_contact,
                "escalate_to_volunteer": self._call_support_contact_alias,
                "activate_camera": self._activate_camera,
                "deactivate_camera": self._deactivate_camera,
                "set_medication_reminder": self._schedule_handler(
                    "set_medication_reminder"
                ),
                "set_appointment_reminder": self._schedule_handler(
                    "set_appointment_reminder"
                ),
                "emergency_alert": self._emergency_alert,
                "escalate_to_family": self._escalate_to_family,
                "control_music": self._music_disabled,
                "navigate_to": self._navigate_to,
                "web_search": self._web_search,
                "get_location": self._get_location,
                "get_current_location": self._get_location,
                "update_location": self._update_location,
                "answer_incoming_call": self._answer_incoming_call,
                "decline_incoming_call": self._decline_incoming_call,
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

    def _family_call_alias(self, call_type: str) -> ToolHandler:
        async def handler(args: dict[str, Any]) -> dict[str, Any]:
            payload = dict(args)
            payload["call_type"] = call_type
            return await self._call_family_contact(payload)

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
        consent_phrase = str(args.get("consent_phrase") or "")
        consent_source = str(args.get("consent_source") or "")
        allowed, detail = validate_camera_activation_consent(
            consent_source=consent_source,
            consent_phrase=consent_phrase,
        )
        if not allowed:
            return {
                "status": "blocked",
                "error": detail,
                "message": (
                    "Camera chỉ được mở sau khi người dùng nói rõ mở/xem "
                    "hoặc đã đồng ý rõ ràng."
                ),
            }
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
            {
                "reason": reason,
                "consent_source": consent_source,
                "consent_phrase": consent_phrase,
            },
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

    async def _call_family_contact(self, args: dict[str, Any]) -> dict[str, Any]:
        result = await self.backend.tool_call(
            "call_family_contact",
            self._uid(),
            {
                "contact_name": str(args.get("contact_name") or ""),
                "call_type": self._normalize_call_type(args.get("call_type")),
            },
        )
        return await self._start_prepared_call(result)

    async def _call_support_contact(self, args: dict[str, Any]) -> dict[str, Any]:
        result = await self.backend.tool_call_with_timeout(
            "call_support_contact",
            self._uid(),
            {
                "contact_name": str(args.get("contact_name") or ""),
                "call_type": self._normalize_call_type(args.get("call_type")),
            },
            timeout=SUPPORT_CALL_BACKEND_TIMEOUT_SECONDS,
        )
        return await self._start_prepared_call(result)

    async def _call_support_contact_alias(self, args: dict[str, Any]) -> dict[str, Any]:
        payload = dict(args)
        payload.setdefault("contact_name", "")
        payload.setdefault("call_type", "voice")
        return await self._call_support_contact(payload)

    async def _start_prepared_call(self, result: dict[str, Any]) -> dict[str, Any]:
        status = str(result.get("status") or "")
        if status != "prepared":
            return dict(result)

        timeout_seconds = self._call_start_timeout_seconds(result)
        device_result = await self._device_request(
            "start_call",
            {
                "room_name": result.get("room_name", ""),
                "token": result.get("token", ""),
                "call_type": result.get("call_type", "voice"),
                "contact_id": result.get("contact_id", ""),
                "contact_name": result.get("contact_name", ""),
                "contact_role": result.get("contact_role", "family"),
                "avatar_url": result.get("avatar_url", ""),
                "answer_timeout_seconds": timeout_seconds,
            },
            timeout=timeout_seconds + CALL_START_TIMEOUT_PADDING_SECONDS,
        )
        merged = {
            key: value
            for key, value in result.items()
            if key not in {"token", "answer_timeout_seconds"}
        }
        merged.update(device_result)
        merged.setdefault("mode", result.get("mode", "family"))
        merged.setdefault("call_type", result.get("call_type", "voice"))
        merged.setdefault("contact_name", result.get("contact_name", ""))
        return merged

    @staticmethod
    def _normalize_call_type(value: Any) -> str:
        text = str(value or "").strip().lower()
        if text == "video":
            return "video"
        return "voice"

    @staticmethod
    def _call_start_timeout_seconds(result: dict[str, Any]) -> float:
        value = result.get("answer_timeout_seconds", 30)
        try:
            parsed = float(value)
        except (TypeError, ValueError):
            return 30.0
        return parsed if parsed > 0 else 30.0

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

    async def _escalate_to_family(self, args: dict[str, Any]) -> dict[str, Any]:
        params: dict[str, Any] = {}
        contact_name = str(args.get("contact_name") or "").strip()
        if contact_name:
            params["contact_name"] = contact_name
        skip = args.get("skip")
        if isinstance(skip, list):
            cleaned = [str(s) for s in skip if str(s).strip()]
            if cleaned:
                params["skip"] = cleaned
        result = await self.backend.tool_call(
            "escalate_to_family",
            self._uid(),
            params,
        )
        return await self._start_prepared_call(result)

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

    async def _answer_incoming_call(self, args: dict[str, Any]) -> dict[str, Any]:
        return await self._device_request("answer_incoming_call", {}, timeout=15.0)

    async def _decline_incoming_call(self, args: dict[str, Any]) -> dict[str, Any]:
        return await self._device_request("decline_incoming_call", {}, timeout=10.0)


BACKEND_TOOLS = {
    "send_chat_message",
    "read_unread_messages",
    "list_family_contacts",
    "list_medication_reminders",
    "mark_medication_taken",
    "mark_medication_skipped",
    "log_health_symptom",
    "log_health_metric",
    "play_music",
    "rate_volunteer",
}

BACKEND_TOOL_TIMEOUTS: dict[str, float] = {}
