"""Internal websocket transport/service for LiveKit-free Gemini sessions."""

from __future__ import annotations

import asyncio
import json
import logging
from contextlib import suppress
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, AsyncContextManager, AsyncIterator, Callable, Protocol

from google.genai import errors as genai_errors
from websockets.asyncio.server import serve
from websockets.exceptions import ConnectionClosed

from config import AI_COST_PRICING_VERSION

from .core import GeminiLiveSettings, GeminiTransportSession
from .nsfw_filter import NsfwFilter
from .state import AgentState
from .tool_router import ToolRouter
from .transport import (
    GeminiTransport,
    InboundAudioEvent,
    InboundControlEvent,
    InboundTextEvent,
    InboundTransportEvent,
    InboundVideoEvent,
    ToolRequestEvent,
)

logger = logging.getLogger("aisee-agent")
MAX_WEBSOCKET_MESSAGE_BYTES = 8 * 1024 * 1024


class WebSocketConnection(Protocol):
    def __aiter__(self) -> AsyncIterator[str | bytes]: ...

    async def send(self, data: str | bytes) -> None: ...

    async def close(self) -> None: ...

    async def wait_closed(self) -> None: ...


GeminiSessionFactory = Callable[[GeminiLiveSettings, dict[str, Any]], AsyncContextManager[object]]
ToolRouterFactory = Callable[[GeminiTransport, dict[str, Any]], ToolRouter]
InstructionsFactory = Callable[[dict[str, Any]], str]


@dataclass(slots=True)
class GeminiBridgeWebSocketTransport:
    websocket: WebSocketConnection
    log: logging.Logger = field(default_factory=lambda: logger)
    media_mode: str = "audio"
    on_audio_start: Callable[[], Any] | None = None
    on_audio_stop: Callable[[], Any] | None = None
    session_context: dict[str, Any] = field(default_factory=dict, init=False)
    _prefetched: asyncio.Queue[str | bytes] = field(default_factory=asyncio.Queue, init=False)
    _pending_tool_results: dict[str, asyncio.Future[dict[str, Any]]] = field(
        default_factory=dict,
        init=False,
    )

    async def emit_ready(self) -> None:
        await self._send_json({"type": "ready"})

    async def emit_state(self, state: AgentState) -> None:
        await self._send_json({"type": "agent_state", "state": state.value})

    async def emit_audio(self, pcm: bytes) -> None:
        if pcm:
            await self.websocket.send(pcm)

    async def emit_audio_interrupted(self) -> None:
        await self._send_json({"type": "assistant_audio_interrupted"})

    async def emit_error(
        self,
        code: str,
        message: str,
        *,
        request_id: str = "",
    ) -> None:
        payload = {
            "type": "error",
            "code": code,
            "message": message,
        }
        if request_id:
            payload["request_id"] = request_id
        await self._send_json(payload)

    async def send_tool_request(
        self,
        event: ToolRequestEvent,
        timeout: float | None = None,
    ) -> dict:
        future: asyncio.Future[dict[str, Any]] = asyncio.get_running_loop().create_future()
        self._pending_tool_results[event.request_id] = future
        payload = {
            "type": event.type,
            "request_id": event.request_id,
            **event.payload,
        }
        if timeout is not None:
            payload["timeout_ms"] = int(timeout * 1000)
        await self._send_json(payload)
        try:
            return await future
        finally:
            self._pending_tool_results.pop(event.request_id, None)

    async def receive(self) -> AsyncIterator[InboundTransportEvent]:
        try:
            while not self._prefetched.empty():
                event = await self._decode_inbound(await self._prefetched.get())
                if event is not None:
                    yield event
            async for raw in self.websocket:
                event = await self._decode_inbound(raw)
                if event is not None:
                    yield event
        finally:
            self._fail_pending_tool_results(
                ConnectionError("assistant websocket closed before tool_result was received")
            )

    async def read_session_init(self, timeout: float = 5.0) -> dict[str, Any]:
        try:
            raw = await asyncio.wait_for(anext(self.websocket.__aiter__()), timeout=timeout)
        except (ConnectionClosed, StopAsyncIteration, TimeoutError):
            return {}

        context = await self._decode_session_init(raw)
        if context is None:
            await self._prefetched.put(raw)
            return {}
        self.session_context = context
        return context

    async def close(self) -> None:
        self._fail_pending_tool_results(ConnectionError("assistant websocket closed"))
        with suppress(Exception):
            await self.websocket.close()
        with suppress(Exception):
            await self.websocket.wait_closed()

    async def _decode_inbound(self, raw: str | bytes) -> InboundTransportEvent | None:
        if isinstance(raw, str):
            return await self._decode_inbound_json(raw)
        if isinstance(raw, (bytes, bytearray, memoryview)):
            payload = bytes(raw)
            if self.media_mode == "camera":
                return InboundVideoEvent(jpeg=payload)
            return InboundAudioEvent(pcm=payload)
        await self.emit_error("invalid_frame", "expected text json or binary media frame")
        return None

    async def _decode_inbound_json(self, raw: str) -> InboundTransportEvent | None:
        try:
            decoded = json.loads(raw)
        except json.JSONDecodeError:
            await self.emit_error("invalid_json", "failed to decode json websocket frame")
            return None
        if not isinstance(decoded, dict):
            await self.emit_error("invalid_json", "json websocket frame must be an object")
            return None

        payload = decoded.get("payload")
        payload_map = payload if isinstance(payload, dict) else {}
        message_type = self._string_value(decoded.get("type"))

        if message_type == "text":
            text = self._string_value(decoded.get("text") or payload_map.get("text"))
            if not text:
                await self.emit_error("invalid_text", "text frame requires a non-empty text value")
                return None
            return InboundTextEvent(text=text)

        if message_type == "control":
            action = self._string_value(decoded.get("action") or payload_map.get("action"))
            if not action:
                await self.emit_error("invalid_control", "control frame requires an action")
                return None
            return InboundControlEvent(action=action, payload=payload_map)

        if message_type == "audio_start":
            await self._call_audio_hook(self.on_audio_start)
            return None

        if message_type == "audio_stop":
            await self._call_audio_hook(self.on_audio_stop)
            return None

        if message_type == "interrupt":
            return InboundControlEvent(action="interrupt", payload=payload_map)

        if message_type == "session_init":
            self.session_context = self._session_context_from_payload(payload_map)
            return None

        if message_type == "media_mode":
            mode = self._normalize_media_mode(decoded.get("mode") or payload_map.get("mode"))
            if mode is None:
                await self.emit_error("invalid_media_mode", "media_mode must be audio or camera")
                return None
            self.media_mode = mode
            return None

        if message_type == "tool_result":
            await self._resolve_tool_result(decoded, payload_map)
            return None

        if message_type == "ping":
            await self._send_json(
                {
                    "type": "pong",
                    "request_id": self._string_value(decoded.get("request_id")),
                }
            )
            return None

        await self.emit_error(
            "unsupported_message_type", f"unsupported message type: {message_type}"
        )
        return None

    async def _decode_session_init(self, raw: str | bytes) -> dict[str, Any] | None:
        if not isinstance(raw, str):
            return None
        try:
            decoded = json.loads(raw)
        except json.JSONDecodeError:
            return None
        if not isinstance(decoded, dict) or decoded.get("type") != "session_init":
            return None
        payload = decoded.get("payload")
        if not isinstance(payload, dict):
            return {}
        return self._session_context_from_payload(payload)

    def _session_context_from_payload(self, payload: dict[str, Any]) -> dict[str, Any]:
        context = dict(payload)
        mode = self._normalize_media_mode(context.get("media_mode"))
        if mode is not None:
            self.media_mode = mode
        return context

    async def _resolve_tool_result(
        self, decoded: dict[str, Any], payload_map: dict[str, Any]
    ) -> None:
        request_id = self._string_value(decoded.get("request_id"))
        if not request_id:
            await self.emit_error("invalid_tool_result", "tool_result requires request_id")
            return

        future = self._pending_tool_results.get(request_id)
        if future is None:
            await self.emit_error(
                "unknown_tool_request",
                "received tool_result for unknown request_id",
                request_id=request_id,
            )
            return

        if future.done():
            return
        future.set_result(self._coerce_tool_result(decoded, payload_map))

    def _coerce_tool_result(
        self,
        decoded: dict[str, Any],
        payload_map: dict[str, Any],
    ) -> dict[str, Any]:
        if payload_map:
            return dict(payload_map)

        if decoded.get("ok") is False:
            message = decoded.get("error") or "tool request failed"
            return {"status": "error", "error": str(message)}

        result = decoded.get("result")
        if isinstance(result, dict):
            return result
        if result is not None:
            return {"status": "ok", "result": result}
        return {"status": "ok"}

    async def _send_json(self, payload: dict[str, Any]) -> None:
        await self.websocket.send(json.dumps(payload))

    def _fail_pending_tool_results(self, exc: Exception) -> None:
        for future in list(self._pending_tool_results.values()):
            if not future.done():
                future.set_exception(exc)
        self._pending_tool_results.clear()

    @staticmethod
    def _normalize_media_mode(mode: object) -> str | None:
        normalized = GeminiBridgeWebSocketTransport._string_value(mode).lower()
        if normalized in {"audio", "audio_pcm"}:
            return "audio"
        if normalized in {"camera", "camera_jpeg"}:
            return "camera"
        return None

    @staticmethod
    def _string_value(value: object) -> str:
        return value.strip() if isinstance(value, str) else ""

    async def _call_audio_hook(self, callback: Callable[[], Any] | None) -> None:
        if callback is None:
            return
        result = callback()
        if asyncio.iscoroutine(result):
            await result


@dataclass(slots=True)
class AIUsageSessionTracker:
    backend: Any
    session_context: dict[str, Any]
    log: logging.Logger
    started_at: str | None = None
    active_source_id: str = ""
    sequence: int = 0

    async def start(self) -> None:
        if self.started_at is not None:
            return
        user_id = self._user_id()
        if not user_id:
            return
        started_at = datetime.now(timezone.utc).isoformat()
        self.sequence += 1
        source_id = self._source_id()
        try:
            await self.backend.start_usage_session(
                user_id,
                "ai_minutes",
                "minutes",
                "assistant_ws",
                source_id,
                started_at,
                self._metadata(),
            )
        except Exception as exc:
            self.log.warning("start websocket ai usage session failed: %s", exc)
            return
        self.started_at = started_at
        self.active_source_id = source_id

    async def commit(self) -> None:
        if self.started_at is None:
            return
        user_id = self._user_id()
        started_at = self.started_at
        source_id = self.active_source_id or self._source_id()
        self.started_at = None
        self.active_source_id = ""
        if not user_id:
            return
        ended_at = datetime.now(timezone.utc).isoformat()
        try:
            await self.backend.commit_usage_session(
                user_id,
                "ai_minutes",
                "minutes",
                "assistant_ws",
                source_id,
                started_at,
                ended_at,
                self._metadata(),
            )
        except Exception as exc:
            self.log.warning("commit websocket ai usage session failed: %s", exc)

    def _user_id(self) -> str:
        value = self.session_context.get("user_id")
        return value.strip() if isinstance(value, str) else ""

    def _source_id(self) -> str:
        value = self.session_context.get("session_id")
        if isinstance(value, str) and value.strip():
            base = value.strip()
        else:
            base = f"assistant-ws-{id(self)}"
        return f"{base}:audio:{self.sequence}"

    def _metadata(self) -> dict[str, Any]:
        return {
            "engine": "websocket",
            "websocket_session_id": self.session_context.get("session_id", ""),
            "audio_session_index": self.sequence,
            "media_mode": self.session_context.get("media_mode", ""),
        }


@dataclass(slots=True)
class GeminiBridgeWebSocketService:
    settings: GeminiLiveSettings
    gemini_session_factory: GeminiSessionFactory | None = None
    tool_router_factory: ToolRouterFactory | None = None
    instructions_factory: InstructionsFactory | None = None
    instructions: str = ""
    log: logging.Logger = field(default_factory=lambda: logger)

    async def handle(self, websocket: WebSocketConnection) -> None:
        transport = GeminiBridgeWebSocketTransport(websocket=websocket, log=self.log)
        session_context = await transport.read_session_init()
        tool_router = self._build_tool_router(transport, session_context)
        backend = tool_router.backend
        usage_tracker = AIUsageSessionTracker(
            backend=backend,
            session_context=session_context,
            log=self.log,
        )
        transport.on_audio_start = usage_tracker.start
        transport.on_audio_stop = usage_tracker.commit
        live_session = GeminiTransportSession(
            settings=self.settings,
            transport=transport,
            tool_router=tool_router,
            nsfw_filter=_load_nsfw_filter_for_service(self.log),
        )
        user_id = str(session_context.get("user_id") or "")
        source_id = str(session_context.get("session_id") or "")
        session_started_at: datetime | None = None
        session_failed = False

        async def _emit_tracking(backend_client, event_type: str) -> None:
            if not user_id:
                return
            try:
                await backend_client.record_tracking_event(
                    event_type=event_type,
                    category="ai_ops",
                    user_id=user_id,
                    metadata={
                        "session_id": source_id,
                        "engine": "websocket",
                        "model": self.settings.model,
                    },
                )
            except Exception:
                pass

        try:
            try:
                session_started_at = datetime.now(timezone.utc)
                await _emit_tracking(backend, "ai_session_started")
                await self._run_gemini_session(
                    live_session,
                    session_context,
                    enable_google_search=self.settings.google_search_enabled,
                )
            except Exception as exc:
                if not self._should_retry_without_google_search(exc):
                    raise
                self.log.warning(
                    "gemini google_search unavailable; retrying without search error=%s",
                    exc,
                )
                await _emit_tracking(backend, "ai_quota_fallback")
                with suppress(Exception):
                    await transport.emit_error(
                        "gemini_search_unavailable",
                        (
                            "Gemini Live Google Search quota is unavailable. "
                            "Continuing the assistant session without web search."
                        ),
                    )
                await self._run_gemini_session(
                    live_session,
                    session_context,
                    enable_google_search=False,
                )
        except Exception as exc:
            session_failed = True
            code = _session_error_code(exc)
            self.log.warning("assistant session failed code=%s error=%s", code, exc)
            await _emit_tracking(backend, "ai_session_failed")
            with suppress(Exception):
                await transport.emit_state(AgentState.ERROR)
            with suppress(Exception):
                await transport.emit_error(code, _session_error_message(exc))
        else:
            await _emit_tracking(backend, "ai_session_ended")
        finally:
            with suppress(Exception):
                await transport.emit_state(AgentState.DISCONNECTED)
            await usage_tracker.commit()
            if user_id and source_id and session_started_at:
                await self._record_ws_ai_cost(
                    backend=backend,
                    user_id=user_id,
                    source_id=source_id,
                    started_at=session_started_at,
                )
            await self._close_tool_router_backend(tool_router)
            await transport.close()

    async def serve(self, host: str, port: int, **kwargs):
        kwargs.setdefault("max_size", MAX_WEBSOCKET_MESSAGE_BYTES)
        return await serve(self.handle, host, port, **kwargs)

    async def _run_gemini_session(
        self,
        live_session: GeminiTransportSession,
        session_context: dict[str, Any],
        *,
        enable_google_search: bool,
    ) -> None:
        async with self._connect_gemini(
            session_context,
            enable_google_search=enable_google_search,
        ) as gemini_session:
            await live_session.run(gemini_session)

    async def _record_ws_ai_cost(
        self,
        *,
        backend,
        user_id: str,
        source_id: str,
        started_at: datetime,
    ) -> None:
        ended_at = datetime.now(timezone.utc)
        duration_seconds = max(
            0.0,
            (ended_at - started_at).total_seconds(),
        )
        try:
            await backend.record_ai_cost(
                user_id=user_id,
                provider="google",
                model=self.settings.model,
                modality="live_audio",
                source="agent",
                source_id=source_id,
                cost_usd=0,
                cost_vnd=0,
                estimated=True,
                pricing_version=AI_COST_PRICING_VERSION,
                occurred_at=ended_at.isoformat(),
                audio_input_seconds=duration_seconds,
                metadata={
                    "engine": "websocket",
                    "session_id": source_id,
                    "duration_seconds": duration_seconds,
                },
            )
        except Exception as exc:
            self.log.warning("record_ws_ai_cost failed: %s", exc)

    def _build_tool_router(
        self,
        transport: GeminiTransport,
        session_context: dict[str, Any],
    ) -> ToolRouter:
        if self.tool_router_factory is None:
            raise RuntimeError("tool_router_factory is required for websocket service")
        return self.tool_router_factory(transport, session_context)

    def _connect_gemini(
        self,
        session_context: dict[str, Any],
        *,
        enable_google_search: bool,
    ) -> AsyncContextManager[object]:
        if self.gemini_session_factory is not None:
            session_context = dict(session_context)
            session_context["enable_google_search"] = enable_google_search
            return self.gemini_session_factory(self.settings, session_context)
        config = self.settings.build_live_config(enable_google_search=enable_google_search)
        instructions = self.instructions
        if self.instructions_factory is not None:
            instructions = self.instructions_factory(session_context)
        if instructions:
            config = self.settings.build_live_config_with_prompt(
                instructions,
                enable_google_search=enable_google_search,
            )
        return self.settings.client().aio.live.connect(
            model=self.settings.model,
            config=config,
        )

    def _should_retry_without_google_search(self, exc: Exception) -> bool:
        return self.settings.google_search_enabled and _is_quota_error(exc)

    async def _close_tool_router_backend(self, tool_router: ToolRouter) -> None:
        backend = getattr(tool_router, "backend", None)
        close = getattr(backend, "close", None)
        if close is None:
            return
        result = close()
        if asyncio.iscoroutine(result):
            await result


def _session_error_code(exc: Exception) -> str:
    if isinstance(exc, genai_errors.APIError):
        if _is_quota_error(exc):
            return "gemini_quota_exceeded"
        return "gemini_api_error"
    if _is_quota_error(exc):
        return "gemini_quota_exceeded"
    return "session_error"


def _session_error_message(exc: Exception) -> str:
    if _session_error_code(exc) == "gemini_quota_exceeded":
        return (
            "Gemini Live API quota exceeded. "
            "Check the Gemini API key billing/quota or switch to a key/model with Live API quota."
        )
    message = str(exc).strip()
    return message or "assistant session failed"


def _session_error_text(exc: Exception) -> str:
    parts = [str(exc)]
    for attr in ("message", "status", "details"):
        value = getattr(exc, attr, None)
        if value:
            parts.append(str(value))
    return " ".join(parts)


def _load_nsfw_filter_for_service(log: logging.Logger) -> NsfwFilter | None:
    try:
        return NsfwFilter()
    except Exception as exc:
        log.warning("nsfw filter unavailable, skipping server-side guard: %s", exc)
        return None


def _is_quota_error(exc: Exception) -> bool:
    message = _session_error_text(exc).lower()
    return "quota" in message or "rate limit" in message or "resource exhausted" in message
