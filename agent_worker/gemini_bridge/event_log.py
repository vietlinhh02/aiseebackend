"""Conversation event log for long-running AI sessions.

Records structured events for:
- Debugging and observability
- Rolling summary generation
- Context injection on reconnect

Events are kept in memory (not persisted to DB).  For DB persistence,
emit events to an external sink (e.g., tracking API).
"""

from __future__ import annotations

import time
from dataclasses import dataclass, field
from enum import StrEnum
from typing import Any


class EventType(StrEnum):
    USER_SPEECH = "user_speech"
    ASSISTANT_RESPONSE = "assistant_response"
    TOOL_REQUEST = "tool_request"
    TOOL_RESULT = "tool_result"
    DEVICE_EVENT = "device_event"
    ERROR_EVENT = "error_event"
    SAFETY_EVENT = "safety_event"
    CALL_EVENT = "call_event"
    RECONNECT_EVENT = "reconnect_event"
    STATE_CHANGE = "state_change"
    CONTEXT_INJECTION = "context_injection"


@dataclass(frozen=True, slots=True)
class SessionEvent:
    event_type: EventType
    data: dict[str, Any]
    timestamp: float = field(default_factory=time.monotonic)
    wall_time: str = ""  # ISO format for logging

    def to_dict(self) -> dict[str, Any]:
        return {
            "type": self.event_type,
            "data": self.data,
            "ts": self.wall_time or str(self.timestamp),
        }


class ConversationEventLog:
    """In-memory event log for one AI session.

    Keeps last N events and generates rolling summaries.
    """

    def __init__(self, max_events: int = 200, summary_interval: int = 10) -> None:
        self._events: list[SessionEvent] = []
        self._max_events = max_events
        self._summary_interval = summary_interval
        self._turns_since_summary = 0
        self._rolling_summary = ""

    def record(self, event_type: EventType, data: dict[str, Any]) -> SessionEvent:
        """Record an event."""
        from datetime import datetime, timezone
        event = SessionEvent(
            event_type=event_type,
            data=data,
            wall_time=datetime.now(timezone.utc).isoformat(),
        )
        self._events.append(event)
        if len(self._events) > self._max_events:
            self._events = self._events[-self._max_events:]

        # Count turns for summary generation
        if event_type in (EventType.USER_SPEECH, EventType.ASSISTANT_RESPONSE):
            self._turns_since_summary += 1

        return event

    def should_generate_summary(self) -> bool:
        return self._turns_since_summary >= self._summary_interval

    def get_pending_summary_events(self) -> list[SessionEvent]:
        """Get events since last summary for summary generation."""
        return self._events[-self._turns_since_summary:] if self._turns_since_summary > 0 else []

    def commit_summary(self, summary: str) -> None:
        """Store generated summary and reset turn counter."""
        self._rolling_summary = summary
        self._turns_since_summary = 0

    @property
    def rolling_summary(self) -> str:
        return self._rolling_summary

    def get_last_turns(self, n: int = 10) -> list[dict[str, str]]:
        """Get last N user/assistant turns for context injection."""
        turns: list[dict[str, str]] = []
        for event in reversed(self._events):
            if event.event_type == EventType.USER_SPEECH:
                turns.append({"role": "user", "text": event.data.get("text", "")})
            elif event.event_type == EventType.ASSISTANT_RESPONSE:
                turns.append({"role": "assistant", "text": event.data.get("text", "")})
            if len(turns) >= n:
                break
        turns.reverse()
        return turns

    def get_recent_errors(self, n: int = 3) -> list[dict[str, Any]]:
        """Get last N error events."""
        errors: list[dict[str, Any]] = []
        for event in reversed(self._events):
            if event.event_type in (EventType.ERROR_EVENT, EventType.SAFETY_EVENT):
                errors.append(event.data)
            if len(errors) >= n:
                break
        errors.reverse()
        return errors

    def get_recent_tool_calls(self, n: int = 5) -> list[dict[str, Any]]:
        """Get last N tool call results."""
        tools: list[dict[str, Any]] = []
        for event in reversed(self._events):
            if event.event_type == EventType.TOOL_RESULT:
                tools.append(event.data)
            if len(tools) >= n:
                break
        tools.reverse()
        return tools

    def get_unresolved_tasks(self) -> list[str]:
        """Identify tasks that started but didn't complete."""
        pending: dict[str, str] = {}  # request_id -> tool_name
        resolved: set[str] = set()

        for event in self._events:
            if event.event_type == EventType.TOOL_REQUEST:
                rid = event.data.get("request_id", "")
                name = event.data.get("name", "")
                if rid:
                    pending[rid] = name
            elif event.event_type == EventType.TOOL_RESULT:
                rid = event.data.get("request_id", "")
                if rid:
                    resolved.add(rid)
                    pending.pop(rid, None)

        return list(pending.values())

    def build_context_for_reconnect(self) -> str:
        """Build a concise context string for Gemini session restore.

        This is injected as a system/user message when reconnecting.
        """
        parts: list[str] = []

        if self._rolling_summary:
            parts.append(f"Tóm tắt cuộc trò chuyện trước đó: {self._rolling_summary}")

        recent_errors = self.get_recent_errors(2)
        if recent_errors:
            error_texts = [e.get("message", "") for e in recent_errors if e.get("message")]
            if error_texts:
                parts.append(f"Lỗi gần đây: {'; '.join(error_texts)}")

        unresolved = self.get_unresolved_tasks()
        if unresolved:
            parts.append(f"Task chưa hoàn thành: {', '.join(unresolved)}")

        recent_turns = self.get_last_turns(5)
        if recent_turns:
            turn_texts = []
            for t in recent_turns:
                role = "User" if t["role"] == "user" else "Assistant"
                text = t["text"][:100] + "..." if len(t["text"]) > 100 else t["text"]
                turn_texts.append(f"{role}: {text}")
            parts.append("Cuộc trò chuyện gần đây:\n" + "\n".join(turn_texts))

        if not parts:
            return ""

        return (
            "=== PHIÊN TRƯỚC ĐÃ ĐƯỢC PHỤC HỒI ===\n"
            + "\n".join(parts)
            + "\n=== HẾT THÔNG TIN PHIÊN TRƯỚC ==="
        )

    @property
    def event_count(self) -> int:
        return len(self._events)

    def clear(self) -> None:
        self._events.clear()
        self._rolling_summary = ""
        self._turns_since_summary = 0
