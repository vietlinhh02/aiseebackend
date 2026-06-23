"""Gemini function-call router — dispatches tool calls to backend or device.

Uses:
- contracts.py for all constant names (no string literals)
- AgentRuntimeState for camera/call/tool state tracking
- SafeToolExecutor for device commands (timeout, dedup, structured results)
"""

from __future__ import annotations

import asyncio
import logging
import time
import uuid
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any

from camera_activation_policy import validate_camera_activation_consent

from .agent_runtime_state import AgentRuntimeState
from .contracts import (
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
    navigation_client: object | None = None  # MapboxNavigationClient (lazy)
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
        # Fire-and-forget: don't block the tool call hot path on tracking HTTP.
        # The task is bounded by the per-process concurrency cap; failures are
        # silently dropped because tracking is observability, not correctness.
        asyncio.create_task(
            self._send_tool_call_tracking(backend, user_id, name, result)
        )

    async def _send_tool_call_tracking(
        self, backend, user_id: str, name: str, result: dict
    ) -> None:
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
                GeminiTool.START_WALKING_NAVIGATION: self._start_walking_navigation,
                GeminiTool.ANNOUNCE_NAVIGATION_STEP: self._announce_navigation_step,
                GeminiTool.STOP_NAVIGATION: self._stop_navigation,
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
        """Coerce a tool-provided call type to ``"video"`` or ``"voice"``.

        Default is **video** when the value is missing/empty/unknown — see
        ``_call_family_contact`` and ``_call_support_contact`` which assume
        the call should be a video call unless the caller explicitly opts
        out. This matches the product requirement: family and support calls
        should be video by default so the receiving party can see the user.
        """
        text = str(value or "").strip().lower()
        if text == "voice":
            return "voice"
        return "video"

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

    # ── In-app walking navigation (Mapbox Directions + Geocoding) ──

    def _get_navigation_client(self):
        """Lazy-init Mapbox client từ env. Cached trong self."""
        if self.navigation_client is None:
            from .mapbox_navigation import MapboxNavigationClient
            self.navigation_client = MapboxNavigationClient.from_env()
        return self.navigation_client

    def _get_nav_state(self) -> dict[str, Any]:
        """Lấy user_context["navigation_state"] (in-memory, per-session)."""
        if "navigation_state" not in self.user_context:
            self.user_context["navigation_state"] = None
        return self.user_context

    async def _start_walking_navigation(self, args: dict[str, Any]) -> dict[str, Any]:
        """Geocode destination + lấy walking route, lưu vào session state."""
        from .mapbox_navigation import GeoPoint, NavigationSession

        destination = (args.get("destination") or "").strip()
        if not destination:
            return tool_error(
                ToolErrorCode.UNKNOWN,
                "start_walking_navigation: thiếu destination",
            )

        client = self._get_navigation_client()
        if not client.is_configured:
            return tool_error(
                ToolErrorCode.DEVICE_UNAVAILABLE,
                "chỉ đường chưa được cấu hình (thiếu MAPBOX_NAVIGATION_TOKEN)",
                recoverable=False,
            )

        # Nếu đã có session active, dừng session cũ trước
        if self.user_context.get("navigation_state"):
            old = self.user_context["navigation_state"]
            old.outcome = "cancelled"

        # 1. Lấy vị trí hiện tại từ Flutter
        loc_result = await self._device_request(DeviceCommand.GET_LOCATION, {}, timeout=10.0)
        loc = loc_result.get("data", loc_result) if isinstance(loc_result, dict) else {}
        if not isinstance(loc, dict) or not loc.get("lat") or not loc.get("lng"):
            return tool_error(
                ToolErrorCode.DEVICE_UNAVAILABLE,
                "không lấy được vị trí hiện tại để bắt đầu chỉ đường",
            )
        origin = GeoPoint(lat=float(loc["lat"]), lng=float(loc["lng"]))

        # 2. Geocode destination
        geo = await client.geocode(destination, proximity=origin)
        if geo.get("status") != "ok":
            return tool_error(
                ToolErrorCode.UNKNOWN,
                f"không tìm được địa điểm: {destination}. {geo.get('error', '')}",
            )
        destination_point = GeoPoint(lat=float(geo["lat"]), lng=float(geo["lng"]))

        # 3. Lấy walking route
        route_resp = await client.walking_route(origin, destination_point)
        if route_resp.get("status") != "ok":
            return tool_error(
                ToolErrorCode.UNKNOWN,
                f"không tìm được đường đi bộ: {route_resp.get('error', '')}",
            )
        route = route_resp["route"]

        if not route.steps:
            return tool_error(
                ToolErrorCode.UNKNOWN,
                "Mapbox trả route không có bước chỉ đường",
            )

        # 3.5 Safety guardrail: cầm điện thoại đi bộ xa rất nguy hiểm cho người
        # khiếm thị/người già. Từ chối nếu quá xa và đề xuất phương án khác.
        from config import NAV_MAX_DISTANCE_M
        if route.total_distance_m > NAV_MAX_DISTANCE_M:
            km = route.total_distance_m / 1000.0
            max_km = NAV_MAX_DISTANCE_M / 1000.0
            return {
                "status": "refused_too_far",
                "total_distance_m": round(route.total_distance_m, 1),
                "max_distance_m": NAV_MAX_DISTANCE_M,
                "hint_for_agent": (
                    f"Quãng đường đi bộ là {km:.1f}km — quá xa để chỉ đường đi bộ "
                    f"trong app (max {max_km:.1f}km vì lý do an toàn). "
                    f"Đề xuất user: 1) gọi xe ôm/grab, 2) gọi con/cháu đưa đón, "
                    f"3) đi taxi với người nhà. Nếu user thật sự muốn đi bộ và xác "
                    f"nhận an toàn, có thể chia thành nhiều cuốc nhỏ hoặc tăng "
                    f"NAV_MAX_DISTANCE_M trong env."
                ),
            }

        # 4. Lưu session state
        import time
        session = NavigationSession(
            route=route,
            started_at=time.time(),
            destination_query=destination,
            origin_point=origin,
        )
        self.user_context["navigation_state"] = session

        # 5. Bật camera nếu chưa — để nhìn vật cản
        #    + bump FPS lên 600ms (~1.67 fps) để Gemini thấy rõ vật cản
        try:
            await self._device_request(
                DeviceCommand.SET_CAMERA_FPS,
                {"interval_ms": 600},
                timeout=4.0,
            )
        except Exception as exc:
            logger.warning("nav: cannot set camera FPS: %s", exc)

        if not self.runtime_state.camera.enabled:
            try:
                await self._device_request(
                    DeviceCommand.ENABLE_CAMERA, {}, timeout=6.0
                )
            except Exception as exc:
                logger.warning("nav: cannot enable camera automatically: %s", exc)

        # 6. Track quota
        try:
            await self.backend.check_quota(self._uid(), "navigation_sessions", 1)
            await self.backend.record_usage(
                self._uid(), "navigation_sessions", 1, "sessions", "agent",
                f"nav-{uuid.uuid4()}",
                {
                    "destination": destination,
                    "destination_name": geo.get("name", ""),
                    "total_distance_m": route.total_distance_m,
                    "total_steps": len(route.steps),
                    "mode": "in_app_walking",
                },
            )
        except Exception as exc:
            logger.warning("nav: quota tracking failed (non-blocking): %s", exc)

        first_step = route.steps[0]

        # Hiện navigation screen trên Flutter (UI overlay toàn màn hình
        # với destination + step hiện tại + nút dừng)
        try:
            await self._device_request(
                DeviceCommand.SHOW_NAVIGATION_SCREEN,
                {
                    "destination_name": geo.get("name", destination),
                    "destination_query": destination,
                    "destination_lat": destination_point.lat,
                    "destination_lng": destination_point.lng,
                    "origin_lat": origin.lat,
                    "origin_lng": origin.lng,
                    "total_distance_m": round(route.total_distance_m, 1),
                    "total_duration_s": round(route.total_duration_s, 1),
                    "total_steps": len(route.steps),
                    "first_step_name": first_step.name,
                    "first_step_instruction_english": first_step.instruction_text,
                    "first_step_voice_english": first_step.voice_text,
                    "first_step_distance_m": round(first_step.distance_m, 1),
                    "max_distance_m": NAV_MAX_DISTANCE_M,
                },
                timeout=4.0,
            )
        except Exception as exc:
            logger.warning("nav: cannot show navigation screen: %s", exc)

        return {
            "status": "ok",
            "destination_name": geo.get("name", destination),
            "total_distance_m": round(route.total_distance_m, 1),
            "total_duration_s": round(route.total_duration_s, 1),
            "total_steps": len(route.steps),
            "first_step_instruction_english": first_step.instruction_text,
            "first_step_voice_english": first_step.voice_text,
            "first_step_name": first_step.name,
            "first_step_distance_m": round(first_step.distance_m, 1),
            "origin_lat": origin.lat,
            "origin_lng": origin.lng,
            "destination_lat": destination_point.lat,
            "destination_lng": destination_point.lng,
            "hint_for_agent": (
                "Dịch 'first_step_voice_english' sang tiếng Việt tự nhiên và đọc cho user. "
                "KHÔNG đọc 'instruction_english' nguyên văn. "
                "Dùng 'total_distance_m' / 'total_duration_s' để thông báo tổng quãng đường + thời gian ước tính."
            ),
        }

    async def _announce_navigation_step(self, args: dict[str, Any]) -> dict[str, Any]:
        """Tính step hiện tại + deviation, trả cho Gemini narrate."""
        from .mapbox_navigation import GeoPoint, evaluate_progress

        from config import NAV_DEVIATION_METERS, NAV_DEVIATION_SECONDS

        session = self.user_context.get("navigation_state")
        if session is None:
            return tool_error(
                ToolErrorCode.UNKNOWN,
                "chưa có cuốc chỉ đường nào đang active. Gọi start_walking_navigation trước.",
            )

        try:
            current = GeoPoint(
                lat=float(args.get("current_lat") or 0),
                lng=float(args.get("current_lng") or 0),
            )
        except (TypeError, ValueError):
            return tool_error(
                ToolErrorCode.UNKNOWN,
                "current_lat / current_lng không hợp lệ",
            )

        if current.lat == 0 and current.lng == 0:
            return tool_error(
                ToolErrorCode.UNKNOWN,
                "vĩ độ/kinh độ bằng 0 — không xác định được vị trí",
            )

        progress = evaluate_progress(
            session,
            current,
            deviation_threshold_m=NAV_DEVIATION_METERS,
            deviation_cooldown_s=NAV_DEVIATION_SECONDS,
        )

        # Nếu đã đến đích → tự stop
        if progress["is_completed"] and session.outcome != "completed":
            session.outcome = "completed"
            self.user_context["navigation_state"] = None
            progress["auto_stopped"] = True
            # Ẩn nav screen trên Flutter
            try:
                await self._device_request(
                    DeviceCommand.HIDE_NAVIGATION_SCREEN,
                    {"outcome": "completed"},
                    timeout=3.0,
                )
            except Exception:
                pass

        progress["hint_for_agent"] = (
            "Nếu should_alert_deviation=true: nói ngắn 'Có vẻ bạn đang đi lệch, "
            "hãy quay lại hướng [tên đường hiện tại]'. "
            "Nếu is_completed=true: thông báo 'Bạn đã đến nơi' rồi dừng. "
            "Nếu KHÔNG: dịch current_step_voice_text sang tiếng Việt và đọc cho user. "
            "Khi đọc xong, KHÔNG cần gọi tool gì thêm — chờ user đi tiếp."
        )
        return {"status": "ok", **progress}

    async def _stop_navigation(self, args: dict[str, Any]) -> dict[str, Any]:
        """Kết thúc cuốc navigation hiện tại + lưu lịch sử."""
        import time


        session = self.user_context.get("navigation_state")
        if session is None:
            return {
                "status": "ok",
                "was_active": False,
                "message": "không có cuốc chỉ đường nào đang active",
            }

        outcome = (args.get("outcome") or "cancelled").strip()
        if outcome not in ("completed", "cancelled", "deviated", "error"):
            outcome = "cancelled"
        session.outcome = outcome
        now = time.time()
        duration_s = int(now - session.started_at)

        # Lưu record tracking (fire-and-forget)
        try:
            await self.backend.record_usage(
                self._uid(),
                "navigation_trips",
                1,
                "trips",
                "agent",
                f"navtrip-{uuid.uuid4()}",
                {
                    "destination_query": session.destination_query,
                    "origin_lat": session.origin_point.lat,
                    "origin_lng": session.origin_point.lng,
                    "destination_lat": session.route.destination_point.lat,
                    "destination_lng": session.route.destination_point.lng,
                    "total_distance_m": round(session.route.total_distance_m, 1),
                    "total_duration_estimated_s": round(session.route.total_duration_s, 1),
                    "actual_duration_s": duration_s,
                    "total_steps": len(session.route.steps),
                    "steps_completed": session.current_step_index,
                    "deviation_count": session.deviation_count,
                    "outcome": outcome,
                    "mode": "in_app_walking",
                },
            )
        except Exception as exc:
            logger.warning("nav: failed to record trip history: %s", exc)

        # Tắt camera nếu đang bật do navigation
        if self.runtime_state.camera.enabled:
            try:
                await self._device_request(DeviceCommand.DISABLE_CAMERA, {}, timeout=5.0)
            except Exception:
                pass

        # Khôi phục FPS mặc định nếu camera vẫn đang chạy cho việc khác
        # (ví dụ user mở camera để xem thứ khác sau khi dừng nav)
        try:
            await self._device_request(
                DeviceCommand.SET_CAMERA_FPS,
                {"interval_ms": 1000},
                timeout=4.0,
            )
        except Exception:
            pass

        # Ẩn navigation screen trên Flutter
        try:
            await self._device_request(
                DeviceCommand.HIDE_NAVIGATION_SCREEN,
                {"outcome": outcome},
                timeout=3.0,
            )
        except Exception:
            pass

        self.user_context["navigation_state"] = None

        return {
            "status": "ok",
            "was_active": True,
            "outcome": outcome,
            "duration_s": duration_s,
            "deviation_count": session.deviation_count,
            "steps_completed": session.current_step_index,
            "total_steps": len(session.route.steps),
            "total_distance_m": round(session.route.total_distance_m, 1),
        }

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
