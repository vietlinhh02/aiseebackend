"""Gemini function-call router — dispatches tool calls to backend or device.

Uses:
- contracts.py for all constant names (no string literals)
- AgentRuntimeState for camera/call/tool state tracking
- SafeToolExecutor for device commands (timeout, dedup, structured results)
"""

from __future__ import annotations

import logging
import time
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any

from camera_activation_policy import validate_camera_activation_consent

from .agent_runtime_state import AgentRuntimeState
from .contracts import (
    AUDIO_INTERRUPT_COMMANDS,
    BACKEND_ONLY_TOOLS,
    BACKEND_TOOL_TIMEOUTS,
    DeviceCommand,
    GeminiTool,
    ToolErrorCode,
    tool_error,
    tool_success,
    tool_unknown,
)
from .safe_tool_executor import SafeToolExecutor
from .transport import DeviceCommandChannel

logger = logging.getLogger("aisee-agent")
SUPPORT_CALL_BACKEND_TIMEOUT_SECONDS = 45.0
CALL_START_TIMEOUT_PADDING_SECONDS = 10.0

ToolHandler = Callable[[dict[str, Any]], Awaitable[dict[str, Any]]]


@dataclass(slots=True)
class ToolRouter:
    backend: object
    user_context: dict
    runtime_state: AgentRuntimeState = field(default_factory=AgentRuntimeState)
    device_channel: DeviceCommandChannel | None = None
    executor: SafeToolExecutor | None = None
    local_participant: object | None = None
    search_client: object | None = None
    on_camera_state_change: object | None = None
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
                result = tool_unknown(name)
            else:
                result = self._coerce_result(await handler(args))
            if "status" not in result:
                result["status"] = "ok"
            logger.info(
                "tool_call ok name=%s status=%s elapsed_ms=%d",
                name,
                result.get("status"),
                int((time.perf_counter() - started) * 1000),
            )
            await self._record_tool_call_tracking(name, result)
            return result
        except TimeoutError:
            logger.warning(
                "tool_call timeout name=%s elapsed_ms=%d",
                name,
                int((time.perf_counter() - started) * 1000),
            )
            result = tool_error(ToolErrorCode.TIMEOUT, f"tool '{name}' timed out")
            await self._record_tool_call_tracking(name, result)
            return result
        except Exception as exc:
            logger.warning(
                "tool_call failed name=%s elapsed_ms=%d error=%s",
                name,
                int((time.perf_counter() - started) * 1000),
                exc,
            )
            result = tool_error(ToolErrorCode.UNKNOWN, str(exc))
            await self._record_tool_call_tracking(name, result)
            return result

    async def _record_tool_call_tracking(self, name: str, result: dict) -> None:
        backend = getattr(self, "backend", None)
        if backend is None:
            return
        user_id = self._uid()
        if not user_id:
            return
        try:
            await backend.record_tracking_event(
                event_type="ai_tool_call",
                category="ai_ops",
                user_id=user_id,
                metadata={
                    "tool_name": name,
                    "status": result.get("status", ""),
                },
            )
        except Exception:
            pass

    def _build_handlers(self) -> dict[str, ToolHandler]:
        handlers: dict[str, ToolHandler] = {
            tool_name: self._backend_handler(tool_name)
            for tool_name in BACKEND_ONLY_TOOLS
        }
        handlers.update(
            {
                tool_name: self._backend_handler(tool_name, timeout)
                for tool_name, timeout in BACKEND_TOOL_TIMEOUTS.items()
            }
        )
        handlers.update(
            {
                GeminiTool.MAKE_VOICE_CALL: self._make_voice_call,
                GeminiTool.MAKE_VIDEO_CALL: self._make_video_call,
                GeminiTool.CALL_FAMILY_CONTACT: self._call_family_contact,
                GeminiTool.CALL_SUPPORT_CONTACT: self._call_support_contact,
                GeminiTool.ACTIVATE_CAMERA: self._activate_camera,
                GeminiTool.DEACTIVATE_CAMERA: self._deactivate_camera,
                GeminiTool.SET_MEDICATION_REMINDER: self._schedule_handler(
                    GeminiTool.SET_MEDICATION_REMINDER,
                ),
                GeminiTool.SET_APPOINTMENT_REMINDER: self._schedule_handler(
                    GeminiTool.SET_APPOINTMENT_REMINDER,
                ),
                GeminiTool.EMERGENCY_ALERT: self._emergency_alert,
                GeminiTool.ESCALATE_TO_FAMILY: self._escalate_to_family,
                GeminiTool.CONTROL_MUSIC: self._music_disabled,
                GeminiTool.NAVIGATE_TO: self._navigate_to,
                GeminiTool.ESCALATE_TO_VOLUNTEER: self._call_support_contact,
                GeminiTool.WEB_SEARCH: self._web_search,
                GeminiTool.GET_LOCATION: self._get_location,
                GeminiTool.GET_CURRENT_LOCATION: self._get_location,
                GeminiTool.UPDATE_LOCATION: self._update_location,
                GeminiTool.ANSWER_INCOMING_CALL: self._answer_incoming_call,
                GeminiTool.DECLINE_INCOMING_CALL: self._decline_incoming_call,
            }
        )
        return handlers

    def _backend_handler(self, name: str, timeout: float | None = None) -> ToolHandler:
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
        self, name: str, args: dict[str, Any], timeout: float | None = None,
    ) -> dict[str, Any]:
        if timeout is None:
            return await self.backend.tool_call(name, self._uid(), args)
        return await self.backend.tool_call_with_timeout(name, self._uid(), args, timeout=timeout)

    async def _device_request(
        self, method: str | DeviceCommand, payload: dict[str, Any], timeout: float | None = None,
    ) -> dict[str, Any]:
        """Execute device command via SafeToolExecutor."""
        request_id = f"tool-{uuid.uuid4()}"
        self.runtime_state.on_tool_request(request_id, str(method), payload)

        if self.executor is not None:
            result = await self.executor.execute(request_id, str(method), payload, timeout=timeout)
        elif self.device_channel is not None:
            result = await self.device_channel.request(str(method), payload, timeout=timeout)
        else:
            result = tool_error(
                ToolErrorCode.DEVICE_UNAVAILABLE,
                f"device command channel not configured for {method}",
                recoverable=False,
            )

        self.runtime_state.on_tool_result(request_id)
        return result

    # ── Camera ──

    async def _activate_camera(self, args: dict[str, Any]) -> dict[str, Any]:
        # State-aware: prevent duplicate activation
        if self.runtime_state.camera.enabled:
            return tool_success(message="camera already active")

        if not self.runtime_state.can_retry_camera():
            return tool_error(
                ToolErrorCode.SAFETY_BLOCKED,
                f"camera blocked: {self.runtime_state.camera.last_block_reason}",
                recoverable=False,
            )

        reason = args.get("reason", "User requested")
        consent_phrase = str(args.get("consent_phrase") or "")
        consent_source = str(args.get("consent_source") or "")

        allowed, detail = validate_camera_activation_consent(
            consent_source=consent_source,
            consent_phrase=consent_phrase,
            runtime_state=self.runtime_state,
        )
        if not allowed:
            return tool_error(
                ToolErrorCode.CAMERA_CONSENT_REQUIRED if "required" in detail
                else ToolErrorCode.CAMERA_CONSENT_INVALID,
                "Camera chỉ được mở sau khi người dùng nói rõ mở/xem "
                "hoặc đã đồng ý rõ ràng.",
            )

        self.runtime_state.on_camera_activate_requested()
        source_id = f"camera-{uuid.uuid4()}"

        try:
            await self.backend.check_quota(self._uid(), "camera_activations", 1)
        except Exception as exc:
            logger.warning("camera quota check failed (proceeding anyway): %s", exc)

        result = await self._device_request(DeviceCommand.ENABLE_CAMERA, {}, timeout=8.0)

        # Update state based on result
        success = result.get("success", result.get("status") == "ok")
        if success:
            self.runtime_state.on_camera_activated()
            if self.on_camera_state_change is not None:
                try:
                    await self.on_camera_state_change("activated")
                except Exception:
                    pass
        else:
            error_msg = ""
            if isinstance(result.get("error"), dict):
                error_msg = result["error"].get("message", "")
            elif "message" in result:
                error_msg = result["message"]
            self.runtime_state.on_camera_blocked(error_msg or "activation_failed")

        try:
            await self.backend.record_usage(
                self._uid(), "camera_activations", 1, "activations", "agent",
                source_id, {"reason": reason, "consent_source": consent_source, "consent_phrase": consent_phrase},
            )
        except Exception as exc:
            logger.warning("camera record_usage failed (camera still active): %s", exc)

        return result

    async def _deactivate_camera(self, args: dict[str, Any]) -> dict[str, Any]:
        result = await self._device_request(DeviceCommand.DISABLE_CAMERA, {}, timeout=5.0)
        self.runtime_state.on_camera_deactivated()
        if self.on_camera_state_change is not None:
            try:
                await self.on_camera_state_change("deactivated")
            except Exception:
                pass
        return result

    # ── Schedule ──

    async def _backend_with_schedule_action(self, name: str, args: dict[str, Any]) -> dict[str, Any]:
        result = await self.backend.tool_call(name, self._uid(), args)
        result = dict(result)
        reminder_action = result.pop("schedule_action", None)
        if reminder_action and isinstance(reminder_action, dict):
            try:
                await self._device_request(DeviceCommand.SCHEDULE_REMINDER, reminder_action.get("params", {}))
            except Exception as exc:
                logger.warning("schedule_reminder RPC failed: %s", exc)
        return result

    # ── Call ──

    async def _make_voice_call(self, args: dict[str, Any]) -> dict[str, Any]:
        params = dict(args)
        params["call_type"] = "voice"
        return await self._call_family_contact(params)

    async def _make_video_call(self, args: dict[str, Any]) -> dict[str, Any]:
        params = dict(args)
        params["call_type"] = "video"
        return await self._call_family_contact(params)

    async def _call_family_contact(self, args: dict[str, Any]) -> dict[str, Any]:
        # State-aware: prevent duplicate call
        if not self.runtime_state.can_start_call():
            return tool_error(
                ToolErrorCode.DUPLICATE_REQUEST,
                f"call already in progress: {self.runtime_state.call.flow_status}",
            )

        contact_name = str(args.get("contact_name") or "")
        call_type = self._normalize_call_type(args.get("call_type"))
        self.runtime_state.on_call_intent("family", contact_name)
        self.runtime_state.on_call_confirming()

        result = await self.backend.tool_call(
            GeminiTool.CALL_FAMILY_CONTACT,
            self._uid(),
            {"contact_name": contact_name, "call_type": call_type},
        )
        result["call_type"] = call_type
        return await self._start_prepared_call(result)

    async def _call_support_contact(self, args: dict[str, Any]) -> dict[str, Any]:
        if not self.runtime_state.can_start_call():
            return tool_error(
                ToolErrorCode.DUPLICATE_REQUEST,
                f"call already in progress: {self.runtime_state.call.flow_status}",
            )

        contact_name = str(args.get("contact_name") or "")
        # Support/volunteer calls are ALWAYS video calls.
        # The purpose is for the volunteer to see and help the user (e.g.,
        # read text, describe objects, guide navigation). Audio-only defeats
        # this purpose. If Gemini doesn't explicitly send call_type=video,
        # we force it here.
        call_type = "video"
        self.runtime_state.on_call_intent("support", contact_name)
        logger.info(
            "support call forced to video contact=%s", contact_name,
        )

        result = await self.backend.tool_call_with_timeout(
            GeminiTool.CALL_SUPPORT_CONTACT,
            self._uid(),
            {"contact_name": contact_name, "call_type": call_type},
            timeout=SUPPORT_CALL_BACKEND_TIMEOUT_SECONDS,
        )
        result["call_type"] = call_type
        return await self._start_prepared_call(result)

    async def _start_prepared_call(self, result: dict[str, Any]) -> dict[str, Any]:
        status = str(result.get("status") or "")
        if status != "prepared":
            self.runtime_state.on_call_ended(error=result.get("error", "not_prepared"))
            return dict(result)

        timeout_seconds = self._call_start_timeout_seconds(result)
        self.runtime_state.on_call_started()

        device_result = await self._device_request(
            DeviceCommand.START_CALL,
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

        # Update call state based on result
        device_status = device_result.get("status", "")
        if device_status in ("ringing", "connected"):
            if device_status == "ringing":
                self.runtime_state.on_call_ringing()
            else:
                self.runtime_state.on_call_connected()
        elif device_status == "error":
            error_msg = device_result.get("message", "call failed")
            self.runtime_state.on_call_ended(error=error_msg)
        else:
            self.runtime_state.on_call_ended()

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
        return "video" if text == "video" else "voice"

    @staticmethod
    def _call_start_timeout_seconds(result: dict[str, Any]) -> float:
        value = result.get("answer_timeout_seconds", 30)
        try:
            parsed = float(value)
        except (TypeError, ValueError):
            return 30.0
        return parsed if parsed > 0 else 30.0

    # ── Emergency ──

    async def _emergency_alert(self, args: dict[str, Any]) -> dict[str, Any]:
        self.runtime_state.on_emergency_detected()
        try:
            location = await self._device_request(DeviceCommand.GET_LOCATION, {}, timeout=15.0)
            lat = location.get("data", location).get("lat", 0) if isinstance(location.get("data"), dict) else location.get("lat", 0)
            lng = location.get("data", location).get("lng", 0) if isinstance(location.get("data"), dict) else location.get("lng", 0)
        except Exception:
            logger.warning("get_location RPC failed, using unknown location")
            lat, lng = 0, 0
        return await self.backend.tool_call_with_timeout(
            GeminiTool.EMERGENCY_ALERT, self._uid(), {"lat": lat, "lng": lng}, timeout=20.0,
        )

    async def _escalate_to_family(self, args: dict[str, Any]) -> dict[str, Any]:
        if not self.runtime_state.can_start_call():
            return tool_error(
                ToolErrorCode.DUPLICATE_REQUEST,
                f"call already in progress: {self.runtime_state.call.flow_status}",
            )
        params: dict[str, Any] = {}
        contact_name = str(args.get("contact_name") or "").strip()
        if contact_name:
            params["contact_name"] = contact_name
        skip = args.get("skip")
        if isinstance(skip, list):
            cleaned = [str(s) for s in skip if str(s).strip()]
            if cleaned:
                params["skip"] = cleaned
        self.runtime_state.on_call_intent("family", contact_name)
        result = await self.backend.tool_call(GeminiTool.ESCALATE_TO_FAMILY, self._uid(), params)
        return await self._start_prepared_call(result)

    # ── Music ──

    async def _music_disabled(self, args: dict[str, Any]) -> dict[str, Any]:
        return tool_error(
            ToolErrorCode.DEVICE_UNAVAILABLE,
            "music playback is temporarily disabled",
            recoverable=False,
        )

    # ── Navigation ──

    async def _navigate_to(self, args: dict[str, Any]) -> dict[str, Any]:
        destination = args.get("destination", "")
        source_id = f"navigation-{uuid.uuid4()}"
        await self.backend.check_quota(self._uid(), "navigation_sessions", 1)
        result = await self._device_request(DeviceCommand.OPEN_MAPS, {"destination": destination})
        await self.backend.record_usage(
            self._uid(), "navigation_sessions", 1, "sessions", "agent",
            source_id, {"destination": destination},
        )
        return result

    # ── Search ──

    async def _web_search(self, args: dict[str, Any]) -> dict[str, Any]:
        if self.search_client is None:
            return tool_error(ToolErrorCode.DEVICE_UNAVAILABLE, "web search is not configured", recoverable=False)
        query = str(args.get("query") or "")
        max_results = args.get("max_results")
        search = getattr(self.search_client, "search")
        return await search(query, max_results=max_results)

    # ── Location ──

    async def _get_location(self, args: dict[str, Any]) -> dict[str, Any]:
        return await self._device_request(DeviceCommand.GET_LOCATION, {}, timeout=15.0)

    async def _update_location(self, args: dict[str, Any]) -> dict[str, Any]:
        location = await self._device_request(DeviceCommand.GET_LOCATION, {}, timeout=15.0)
        lat = location.get("data", location).get("lat", 0) if isinstance(location.get("data"), dict) else location.get("lat", 0)
        lng = location.get("data", location).get("lng", 0) if isinstance(location.get("data"), dict) else location.get("lng", 0)
        return await self.backend.tool_call(
            GeminiTool.UPDATE_LOCATION, self._uid(), {"lat": lat, "lng": lng},
        )

    # ── Incoming call ──

    async def _answer_incoming_call(self, args: dict[str, Any]) -> dict[str, Any]:
        return await self._device_request(DeviceCommand.ANSWER_INCOMING_CALL, {}, timeout=15.0)

    async def _decline_incoming_call(self, args: dict[str, Any]) -> dict[str, Any]:
        return await self._device_request(DeviceCommand.DECLINE_INCOMING_CALL, {}, timeout=10.0)
