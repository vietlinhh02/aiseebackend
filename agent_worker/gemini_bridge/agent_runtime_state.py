"""Centralized runtime state for the AI agent session.

This is the single source of truth for camera, tool, call, and conversation
state during a live session.  It is NOT persisted to DB — it lives in RAM
for the duration of one Gemini Live session.

When WebSocket reconnects, a snapshot can be restored via from_snapshot().
"""

from __future__ import annotations

import asyncio
import logging
import time
from dataclasses import dataclass, field
from typing import Any, Callable, Awaitable

from .event_log import ConversationEventLog, EventType
from .memory_manager import MemoryManager, PersistedMemory

logger = logging.getLogger("aisee-agent")

# Camera auto-deactivation safety net (seconds).
# Gemini Live video sessions have a 2-minute limit.  This watchdog ensures
# the camera is always turned off even if Gemini "forgets" to call the tool.
CAMERA_WATCHDOG_TIMEOUT_SECONDS = 120.0


# ─────────────────────────────────────────────────────────────────────
# Sub-states
# ─────────────────────────────────────────────────────────────────────

@dataclass
class CameraState:
    """Tracks camera lifecycle."""
    enabled: bool = False
    streaming: bool = False
    last_block_reason: str | None = None  # "nsfw_detected", "permission_denied", "system_error"
    awaiting_reenable_confirmation: bool = False
    retry_count: int = 0
    last_event_at: float | None = None

    def reset(self) -> None:
        self.enabled = False
        self.streaming = False
        self.last_block_reason = None
        self.awaiting_reenable_confirmation = False
        self.retry_count = 0

    def to_dict(self) -> dict[str, Any]:
        return {
            "enabled": self.enabled,
            "streaming": self.streaming,
            "last_block_reason": self.last_block_reason,
            "awaiting_reenable_confirmation": self.awaiting_reenable_confirmation,
            "retry_count": self.retry_count,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> CameraState:
        return cls(
            enabled=data.get("enabled", False),
            streaming=data.get("streaming", False),
            last_block_reason=data.get("last_block_reason"),
            awaiting_reenable_confirmation=data.get("awaiting_reenable_confirmation", False),
            retry_count=data.get("retry_count", 0),
        )


@dataclass
class PendingTool:
    """A tool request that is waiting for a result."""
    request_id: str
    name: str
    started_at: float = field(default_factory=time.monotonic)
    params: dict[str, Any] = field(default_factory=dict)


@dataclass
class ToolState:
    """Tracks in-flight and completed tool requests."""
    active_tool_request_id: str | None = None
    active_tool_name: str | None = None
    pending_tools: dict[str, PendingTool] = field(default_factory=dict)
    processed_request_ids: set[str] = field(default_factory=set)
    last_tool_error: str | None = None

    def on_request(self, request_id: str, name: str, params: dict | None = None) -> None:
        self.active_tool_request_id = request_id
        self.active_tool_name = name
        self.pending_tools[request_id] = PendingTool(
            request_id=request_id, name=name, params=params or {},
        )

    def on_result(self, request_id: str) -> PendingTool | None:
        pending = self.pending_tools.pop(request_id, None)
        self.processed_request_ids.add(request_id)
        if self.active_tool_request_id == request_id:
            self.active_tool_request_id = None
            self.active_tool_name = None
        return pending

    def is_duplicate(self, request_id: str) -> bool:
        return request_id in self.processed_request_ids

    def has_active_tool(self) -> bool:
        return self.active_tool_request_id is not None

    def mark_timeout(self, request_id: str) -> PendingTool | None:
        pending = self.pending_tools.pop(request_id, None)
        self.processed_request_ids.add(request_id)
        if self.active_tool_request_id == request_id:
            self.active_tool_request_id = None
            self.active_tool_name = None
        self.last_tool_error = "timeout"
        return pending

    def cancel_all(self) -> list[PendingTool]:
        cancelled = list(self.pending_tools.values())
        for p in cancelled:
            self.processed_request_ids.add(p.request_id)
        self.pending_tools.clear()
        self.active_tool_request_id = None
        self.active_tool_name = None
        return cancelled

    def on_disconnect(self) -> list[PendingTool]:
        """Mark all pending tools as interrupted (socket disconnected)."""
        return self.cancel_all()

    def to_dict(self) -> dict[str, Any]:
        return {
            "active_tool_request_id": self.active_tool_request_id,
            "active_tool_name": self.active_tool_name,
            "pending_count": len(self.pending_tools),
            "processed_count": len(self.processed_request_ids),
            "last_tool_error": self.last_tool_error,
        }


@dataclass
class CallState:
    """Tracks call flow state machine."""
    # none / intent_detected / selecting / confirming / calling / ringing / in_call / ended / failed / cancelled
    flow_status: str = "none"
    call_type: str | None = None  # family / support / emergency
    selected_contact_name: str | None = None
    selected_contact_id: str | None = None
    active_tool_request_id: str | None = None
    last_call_error: str | None = None
    started_at: float | None = None
    ended_at: float | None = None

    @property
    def is_active(self) -> bool:
        return self.flow_status in {
            "intent_detected", "selecting", "confirming",
            "calling", "ringing", "in_call",
        }

    @property
    def is_terminal(self) -> bool:
        return self.flow_status in {"ended", "failed", "cancelled"}

    def reset(self) -> None:
        self.flow_status = "none"
        self.call_type = None
        self.selected_contact_name = None
        self.selected_contact_id = None
        self.active_tool_request_id = None
        self.last_call_error = None
        self.started_at = None
        self.ended_at = None

    def to_dict(self) -> dict[str, Any]:
        return {
            "flow_status": self.flow_status,
            "call_type": self.call_type,
            "selected_contact_name": self.selected_contact_name,
            "is_active": self.is_active,
        }


@dataclass
class MemoryState:
    """Rolling conversation memory for long sessions."""
    rolling_summary: str = ""
    last_user_intent: str | None = None
    recent_tool_names: list[str] = field(default_factory=list)
    unresolved_tasks: list[str] = field(default_factory=list)
    last_n_turns: list[dict[str, str]] = field(default_factory=list)
    _max_turns: int = 10

    def add_turn(self, role: str, text: str) -> None:
        self.last_n_turns.append({"role": role, "text": text})
        if len(self.last_n_turns) > self._max_turns:
            self.last_n_turns = self.last_n_turns[-self._max_turns:]

    def add_tool_call(self, tool_name: str) -> None:
        self.recent_tool_names.append(tool_name)
        if len(self.recent_tool_names) > 20:
            self.recent_tool_names = self.recent_tool_names[-20:]

    def update_summary(self, summary: str) -> None:
        self.rolling_summary = summary

    def to_dict(self) -> dict[str, Any]:
        return {
            "rolling_summary": self.rolling_summary,
            "last_user_intent": self.last_user_intent,
            "recent_tool_count": len(self.recent_tool_names),
            "unresolved_count": len(self.unresolved_tasks),
            "turn_count": len(self.last_n_turns),
        }


# ─────────────────────────────────────────────────────────────────────
# Main runtime state
# ─────────────────────────────────────────────────────────────────────

class AgentRuntimeState:
    """Centralized state for one AI assistant session.

    This is the single source of truth for:
    - Conversation mode (normal, camera_assist, call_flow, emergency_flow)
    - Camera lifecycle (enabled, blocked, retry)
    - Tool execution (in-flight, dedup, timeout)
    - Call flow (state machine)
    - Conversation memory (rolling summary, recent turns)
    - Cross-session memory (via MemoryManager)
    """

    def __init__(self, session_id: str = "", user_id: str = "", backend: Any = None) -> None:
        self.session_id = session_id
        self.user_id = user_id
        self.conversation_mode = "normal"  # normal / camera_assist / call_flow / emergency_flow / navigation_flow
        self.camera = CameraState()
        self.tool = ToolState()
        self.call = CallState()
        self.memory = MemoryState()
        self.event_log = ConversationEventLog()
        self.created_at = time.monotonic()
        self.updated_at = time.monotonic()
        self._camera_watchdog_task: asyncio.Task | None = None
        self._camera_deactivate_callback: Callable[[], Awaitable[None]] | None = None

        # Cross-session memory manager (optional, wired up by service layer)
        self.memory_manager: MemoryManager | None = None
        if backend and user_id:
            self.memory_manager = MemoryManager(
                backend=backend,
                user_id=user_id,
                session_id=session_id,
            )

    # ── Camera lifecycle ──

    def on_camera_activate_requested(self) -> None:
        self.camera.awaiting_reenable_confirmation = False
        self.updated_at = time.monotonic()

    def on_camera_activated(self) -> None:
        self.camera.enabled = True
        self.camera.streaming = True
        self.camera.last_block_reason = None
        self.camera.awaiting_reenable_confirmation = False
        self.camera.retry_count = 0
        self.camera.last_event_at = time.monotonic()
        self.conversation_mode = "camera_assist"
        self.updated_at = time.monotonic()
        self._start_camera_watchdog()

    def on_camera_deactivated(self) -> None:
        self.camera.enabled = False
        self.camera.streaming = False
        self.camera.last_event_at = time.monotonic()
        if self.conversation_mode == "camera_assist":
            self.conversation_mode = "normal"
        self.updated_at = time.monotonic()
        self._cancel_camera_watchdog()

    def on_camera_blocked(self, reason: str) -> None:
        self.camera.enabled = False
        self.camera.streaming = False
        self.camera.last_block_reason = reason
        self.camera.awaiting_reenable_confirmation = True
        self.camera.retry_count += 1
        self.camera.last_event_at = time.monotonic()
        self.event_log.record(EventType.SAFETY_EVENT, {
            "event": "camera_blocked",
            "reason": reason,
            "retry_count": self.camera.retry_count,
        })
        self.updated_at = time.monotonic()
        self._cancel_camera_watchdog()

    # ── Camera watchdog ──

    def set_camera_deactivate_callback(
        self, callback: Callable[[], Awaitable[None]] | None,
    ) -> None:
        """Register async callback invoked when watchdog fires.

        The callback should call ToolRouter._deactivate_camera or equivalent.
        """
        self._camera_deactivate_callback = callback

    def _start_camera_watchdog(self) -> None:
        """Start (or restart) the camera auto-deactivation watchdog."""
        self._cancel_camera_watchdog()
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            logger.debug("camera watchdog: no running event loop, skipping")
            return
        self._camera_watchdog_task = loop.create_task(
            self._camera_watchdog_coro(),
        )
        logger.info(
            "camera watchdog started: will auto-deactivate in %.0fs",
            CAMERA_WATCHDOG_TIMEOUT_SECONDS,
        )

    def _cancel_camera_watchdog(self) -> None:
        """Cancel the watchdog if running."""
        task = self._camera_watchdog_task
        if task is not None and not task.done():
            task.cancel()
            logger.info("camera watchdog cancelled")
        self._camera_watchdog_task = None

    async def _camera_watchdog_coro(self) -> None:
        """Wait for timeout, then force camera deactivation."""
        try:
            await asyncio.sleep(CAMERA_WATCHDOG_TIMEOUT_SECONDS)
        except asyncio.CancelledError:
            return

        if not self.camera.enabled:
            return  # Already deactivated by normal flow

        logger.warning(
            "camera watchdog fired after %.0fs — forcing deactivation",
            CAMERA_WATCHDOG_TIMEOUT_SECONDS,
        )
        self.event_log.record(EventType.SAFETY_EVENT, {
            "event": "camera_watchdog_timeout",
            "timeout_seconds": CAMERA_WATCHDOG_TIMEOUT_SECONDS,
        })

        callback = self._camera_deactivate_callback
        if callback is not None:
            try:
                await callback()
            except Exception as exc:
                logger.error("camera watchdog callback failed: %s", exc)
                # Fallback: update state directly
                self.on_camera_deactivated()
        else:
            # No callback registered — update state directly
            self.on_camera_deactivated()

    def can_retry_camera(self) -> bool:
        """Whether camera can be re-activated after a block."""
        if self.camera.last_block_reason is None:
            return True  # Never blocked
        if self.camera.last_block_reason == "permission_denied":
            return False  # Need user to grant permission in settings
        if self.camera.retry_count >= 5:
            return False  # Too many retries
        return True

    def is_camera_context(self) -> bool:
        """Whether the current conversation is in camera/visual context."""
        return (
            self.conversation_mode == "camera_assist"
            or self.camera.enabled
            or self.camera.last_block_reason is not None
        )

    # ── Tool lifecycle ──

    def on_tool_request(self, request_id: str, name: str, params: dict | None = None) -> None:
        self.tool.on_request(request_id, name, params)
        self.memory.add_tool_call(name)
        self.event_log.record(EventType.TOOL_REQUEST, {
            "request_id": request_id,
            "name": name,
            "params_keys": sorted((params or {}).keys()),
        })
        self.updated_at = time.monotonic()

    def on_tool_result(self, request_id: str) -> PendingTool | None:
        result = self.tool.on_result(request_id)
        self.event_log.record(EventType.TOOL_RESULT, {
            "request_id": request_id,
            "name": result.name if result else "unknown",
        })
        self.updated_at = time.monotonic()
        return result

    def on_tool_timeout(self, request_id: str) -> PendingTool | None:
        result = self.tool.mark_timeout(request_id)
        self.event_log.record(EventType.ERROR_EVENT, {
            "event": "tool_timeout",
            "request_id": request_id,
            "name": result.name if result else "unknown",
            "message": f"tool timed out",
        })
        self.updated_at = time.monotonic()
        return result

    def is_duplicate_tool(self, request_id: str) -> bool:
        return self.tool.is_duplicate(request_id)

    # ── Call lifecycle ──

    def on_call_intent(self, call_type: str, contact_name: str = "") -> None:
        self.call.flow_status = "intent_detected"
        self.call.call_type = call_type
        self.call.selected_contact_name = contact_name or None
        self.conversation_mode = "call_flow" if call_type != "emergency" else "emergency_flow"
        self.event_log.record(EventType.CALL_EVENT, {
            "event": "call_intent",
            "call_type": call_type,
            "contact": contact_name,
        })
        self.updated_at = time.monotonic()

    def on_call_confirming(self) -> None:
        self.call.flow_status = "confirming"
        self.updated_at = time.monotonic()

    def on_call_started(self, request_id: str = "") -> None:
        self.call.flow_status = "calling"
        self.call.active_tool_request_id = request_id
        self.call.started_at = time.monotonic()
        self.updated_at = time.monotonic()

    def on_call_ringing(self) -> None:
        self.call.flow_status = "ringing"
        self.updated_at = time.monotonic()

    def on_call_connected(self) -> None:
        self.call.flow_status = "in_call"
        self.updated_at = time.monotonic()

    def on_call_ended(self, error: str | None = None) -> None:
        if error:
            self.call.flow_status = "failed"
            self.call.last_call_error = error
        else:
            self.call.flow_status = "ended"
        self.call.ended_at = time.monotonic()
        self.call.active_tool_request_id = None
        self.event_log.record(EventType.CALL_EVENT, {
            "event": "call_ended",
            "status": self.call.flow_status,
            "error": error,
            "contact": self.call.selected_contact_name,
        })
        if self.conversation_mode in {"call_flow", "emergency_flow"}:
            self.conversation_mode = "normal"
        self.updated_at = time.monotonic()

    def on_call_cancelled(self) -> None:
        self.call.flow_status = "cancelled"
        self.call.active_tool_request_id = None
        self.event_log.record(EventType.CALL_EVENT, {
            "event": "call_cancelled",
            "contact": self.call.selected_contact_name,
        })
        if self.conversation_mode in {"call_flow", "emergency_flow"}:
            self.conversation_mode = "normal"
        self.updated_at = time.monotonic()

    def can_start_call(self) -> bool:
        """Whether a new call can be started (no active call)."""
        return not self.call.is_active

    # ── Emergency ──

    def on_emergency_detected(self) -> None:
        self.conversation_mode = "emergency_flow"
        self.updated_at = time.monotonic()

    # ── Disconnect / Reconnect ──

    def on_disconnect(self) -> list[PendingTool]:
        """Handle socket disconnect — mark all pending tools as interrupted."""
        cancelled = self.tool.on_disconnect()
        self.event_log.record(EventType.RECONNECT_EVENT, {
            "event": "disconnect",
            "pending_tools_cancelled": len(cancelled),
            "conversation_mode": self.conversation_mode,
            "camera_enabled": self.camera.enabled,
            "call_status": self.call.flow_status,
        })
        self.updated_at = time.monotonic()
        return cancelled

    # ── Cross-session memory lifecycle ──

    async def load_persisted_memory(self) -> PersistedMemory:
        """Load cross-session memory from backend. Returns loaded memory."""
        if not self.memory_manager:
            return PersistedMemory()
        return await self.memory_manager.load_from_backend()

    def start_memory_periodic_save(self) -> None:
        """Start background periodic memory persistence."""
        if self.memory_manager:
            self.memory_manager.start_periodic_save()

    async def shutdown_memory(self) -> None:
        """Shutdown memory manager and do final save."""
        if self.memory_manager:
            await self.memory_manager.shutdown()

    # ── Memory ──

    def add_user_turn(self, text: str) -> None:
        self.memory.add_turn("user", text)
        self.event_log.record(EventType.USER_SPEECH, {"text": text})
        if self.memory_manager:
            self.memory_manager.add_user_turn(text)
        self.updated_at = time.monotonic()

    def add_assistant_turn(self, text: str) -> None:
        self.memory.add_turn("assistant", text)
        self.event_log.record(EventType.ASSISTANT_RESPONSE, {"text": text})
        if self.memory_manager:
            self.memory_manager.add_assistant_turn(text)
        self.updated_at = time.monotonic()

    def get_reconnect_context(self) -> str:
        """Build context string for Gemini session restore.

        Uses memory_manager (cross-session) first, then falls back to
        event_log and in-memory state for current session context.
        """
        # Prefer memory manager's context (includes cross-session data)
        if self.memory_manager:
            mm_context = self.memory_manager.build_context_for_gemini()
            if mm_context:
                return mm_context

        # Fallback: use event_log + in-memory state
        summary = self.event_log.rolling_summary or self.memory.rolling_summary
        if not summary and not self.event_log.event_count:
            return ""

        parts: list[str] = []

        if summary:
            parts.append(f"Tom tat cuoc tro chuyen truoc do: {summary}")

        # Current state context
        if self.conversation_mode != "normal":
            parts.append(f"Che do hien tai: {self.conversation_mode}")
        if self.camera.last_block_reason:
            parts.append(f"Camera bi block: {self.camera.last_block_reason}")
        if self.call.is_active:
            parts.append(f"Dang co cuoc goi: {self.call.flow_status} voi {self.call.selected_contact_name or 'unknown'}")

        recent_errors = self.event_log.get_recent_errors(2)
        if recent_errors:
            error_texts = [e.get("message", "") for e in recent_errors if e.get("message")]
            if error_texts:
                parts.append(f"Loi gan day: {'; '.join(error_texts)}")

        unresolved = self.event_log.get_unresolved_tasks()
        if unresolved:
            parts.append(f"Task chua hoan thanh: {', '.join(unresolved)}")

        recent_turns = self.event_log.get_last_turns(5)
        if recent_turns:
            turn_texts = []
            for t in recent_turns:
                role = "User" if t["role"] == "user" else "Assistant"
                text = t["text"][:100] + "..." if len(t["text"]) > 100 else t["text"]
                turn_texts.append(f"{role}: {text}")
            parts.append("Cuoc tro chuyen gan day:\n" + "\n".join(turn_texts))

        if not parts:
            return ""

        return (
            "=== PHIEN TRUOC DA DUOC PHUC HOI ===\n"
            + "\n".join(parts)
            + "\n=== HET THONG TIN PHIEN TRUOC ==="
        )

    # ── Context for Gemini system instruction ──

    def get_context_summary(self) -> str:
        """Generate a brief context summary to inject into Gemini session."""
        parts: list[str] = []

        if self.conversation_mode != "normal":
            parts.append(f"Mode: {self.conversation_mode}")

        if self.camera.enabled:
            parts.append("Camera: active")
        elif self.camera.last_block_reason:
            parts.append(f"Camera: blocked ({self.camera.last_block_reason})")

        if self.call.is_active:
            parts.append(f"Call: {self.call.flow_status} with {self.call.selected_contact_name or 'unknown'}")

        if self.tool.has_active_tool():
            parts.append(f"Tool in progress: {self.tool.active_tool_name}")

        if self.memory.rolling_summary:
            parts.append(f"Context: {self.memory.rolling_summary}")

        if self.memory.unresolved_tasks:
            parts.append(f"Pending: {', '.join(self.memory.unresolved_tasks[:3])}")

        return " | ".join(parts) if parts else ""

    # ── Snapshot for reconnect ──

    def to_snapshot(self) -> dict[str, Any]:
        # Sync event_log summary into memory for snapshot persistence
        if self.event_log.rolling_summary:
            self.memory.rolling_summary = self.event_log.rolling_summary
        snapshot = {
            "session_id": self.session_id,
            "user_id": self.user_id,
            "conversation_mode": self.conversation_mode,
            "camera": self.camera.to_dict(),
            "tool": self.tool.to_dict(),
            "call": self.call.to_dict(),
            "memory": self.memory.to_dict(),
        }
        # Include memory manager snapshot for client-side fallback
        if self.memory_manager:
            snapshot["memory_manager"] = self.memory_manager.get_snapshot()
        return snapshot

    @classmethod
    def from_snapshot(cls, data: dict[str, Any], backend: Any = None) -> AgentRuntimeState:
        state = cls(
            session_id=data.get("session_id", ""),
            user_id=data.get("user_id", ""),
            backend=backend,
        )
        state.conversation_mode = data.get("conversation_mode", "normal")
        if "camera" in data:
            state.camera = CameraState.from_dict(data["camera"])
        # Note: tool state is NOT restored (pending tools are dead after reconnect)
        # Call state is partially restored for context
        if "call" in data:
            call_data = data["call"]
            state.call.flow_status = call_data.get("flow_status", "none")
            state.call.call_type = call_data.get("call_type")
            state.call.selected_contact_name = call_data.get("selected_contact_name")
        if "memory" in data:
            mem_data = data["memory"]
            state.memory.rolling_summary = mem_data.get("rolling_summary", "")
            state.memory.last_user_intent = mem_data.get("last_user_intent")
        # Restore memory manager data from client-side snapshot (fallback)
        if "memory_manager" in data and state.memory_manager:
            mm_data = data["memory_manager"]
            state.memory_manager.rolling_summary = mm_data.get("summary", "")
            state.memory_manager.recent_turns = mm_data.get("recent_turns", [])
            state.memory_manager.user_facts = mm_data.get("user_facts", [])
            state.memory_manager.last_intent = mm_data.get("last_intent", "")
            state.memory_manager.pending_tasks = mm_data.get("pending_tasks", [])
        return state
