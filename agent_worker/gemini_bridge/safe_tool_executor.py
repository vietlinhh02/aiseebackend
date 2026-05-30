"""Safe tool executor for device commands sent to Flutter via WS.

Provides:
- In-flight tracking per request_id
- Duplicate detection via processed_request_ids
- Default timeout (prevents infinite hangs)
- Structured result format (never raw exceptions)
- Cancel/interrupt on socket disconnect
- Late result logging (after timeout)
"""

from __future__ import annotations

import asyncio
import logging
import time
from dataclasses import dataclass, field
from typing import Any

from .contracts import ToolErrorCode, tool_error, tool_timeout

logger = logging.getLogger("aisee-agent")

DEFAULT_DEVICE_COMMAND_TIMEOUT = 20.0  # seconds


@dataclass
class InFlightRequest:
    """A device command waiting for tool_result from Flutter."""
    request_id: str
    command: str
    started_at: float = field(default_factory=time.monotonic)
    timeout: float = DEFAULT_DEVICE_COMMAND_TIMEOUT
    future: asyncio.Future[dict[str, Any]] = field(
        default_factory=lambda: asyncio.get_running_loop().create_future(),
    )


class SafeToolExecutor:
    """Wraps device command channel with safety guarantees.

    Usage:
        executor = SafeToolExecutor(device_channel=transport)
        result = await executor.execute(request_id, "enable_camera", {}, timeout=8.0)
    """

    def __init__(
        self,
        device_channel: Any,
        default_timeout: float = DEFAULT_DEVICE_COMMAND_TIMEOUT,
    ) -> None:
        self._channel = device_channel
        self._default_timeout = default_timeout
        self._in_flight: dict[str, InFlightRequest] = {}
        self._processed_ids: set[str] = set()

    async def execute(
        self,
        request_id: str,
        command: str,
        payload: dict[str, Any],
        timeout: float | None = None,
    ) -> dict[str, Any]:
        """Execute a device command with full safety guarantees.

        Returns a structured result dict — never raises.
        """
        # ── Duplicate check ──
        if request_id in self._processed_ids:
            logger.warning(
                "safe_executor duplicate request_id=%s command=%s",
                request_id, command,
            )
            return {
                "success": False,
                "status": "error",
                "error": {
                    "code": ToolErrorCode.DUPLICATE_REQUEST,
                    "message": f"duplicate request_id: {request_id}",
                    "recoverable": False,
                },
            }

        # ── Create in-flight entry ──
        effective_timeout = timeout if timeout is not None else self._default_timeout
        loop = asyncio.get_running_loop()
        entry = InFlightRequest(
            request_id=request_id,
            command=command,
            timeout=effective_timeout,
            future=loop.create_future(),
        )
        self._in_flight[request_id] = entry

        # ── Send to device channel ──
        try:
            if self._channel is None:
                return {
                    "success": False,
                    "status": "error",
                    "error": {
                        "code": ToolErrorCode.DEVICE_UNAVAILABLE,
                        "message": "device command channel not configured",
                        "recoverable": False,
                    },
                }

            # Send the tool_request to Flutter and use the returned result directly.
            # The transport's send_tool_request resolves when Flutter replies with
            # a tool_result frame. We no longer need the separate entry.future.
            transport_result = await self._channel.send_tool_request(
                _make_tool_request_event(request_id, command, payload),
                timeout=effective_timeout,
            )
            self._finalize(request_id)
            return transport_result
        except asyncio.TimeoutError:
            logger.warning(
                "safe_executor timeout request_id=%s command=%s timeout=%.1fs",
                request_id, command, effective_timeout,
            )
            self._finalize(request_id)
            return {
                "success": False,
                "status": "error",
                "error": {
                    "code": ToolErrorCode.TIMEOUT,
                    "message": f"device command '{command}' timed out after {effective_timeout:.0f}s",
                    "recoverable": True,
                },
            }
        except Exception as exc:
            logger.warning(
                "safe_executor error request_id=%s command=%s error=%s",
                request_id, command, exc,
            )
            self._finalize(request_id)
            return {
                "success": False,
                "status": "error",
                "error": {
                    "code": ToolErrorCode.UNKNOWN,
                    "message": str(exc),
                    "recoverable": True,
                },
            }


    def resolve(self, request_id: str, result: dict[str, Any]) -> bool:
        """Resolve a pending in-flight request with a result from Flutter.

        Returns True if resolved, False if request_id not found (late result).
        """
        entry = self._in_flight.get(request_id)
        if entry is None:
            # Late result after timeout or after disconnect
            logger.info(
                "safe_executor late_result request_id=%s (already resolved/timeout)",
                request_id,
            )
            return False

        if entry.future.done():
            return False

        # Coerce result to structured format
        structured = _coerce_to_structured(result)
        entry.future.set_result(structured)
        return True

    def cancel(self, request_id: str) -> bool:
        """Cancel a pending in-flight request."""
        entry = self._in_flight.get(request_id)
        if entry is None or entry.future.done():
            return False
        entry.future.set_result(
            tool_error(ToolErrorCode.CANCELLED, f"tool '{entry.command}' cancelled"),
        )
        self._finalize(request_id)
        return True

    def cancel_all(self) -> int:
        """Cancel all pending in-flight requests. Returns count cancelled."""
        count = 0
        for request_id in list(self._in_flight):
            if self.cancel(request_id):
                count += 1
        return count

    def on_disconnect(self) -> int:
        """Handle socket disconnect — mark all pending as interrupted."""
        count = 0
        for request_id, entry in list(self._in_flight.items()):
            if not entry.future.done():
                entry.future.set_result(
                    tool_error(
                        ToolErrorCode.DEVICE_UNAVAILABLE,
                        f"tool '{entry.command}' interrupted by disconnect",
                    ),
                )
                self._finalize(request_id)
                count += 1
        return count

    def has_pending(self) -> bool:
        return len(self._in_flight) > 0

    def get_pending_command(self) -> str | None:
        """Get the command name of the first pending request."""
        for entry in self._in_flight.values():
            if not entry.future.done():
                return entry.command
        return None

    def _finalize(self, request_id: str) -> None:
        """Move request from in-flight to processed."""
        self._in_flight.pop(request_id, None)
        self._processed_ids.add(request_id)
        # Cap processed_ids to prevent memory growth
        if len(self._processed_ids) > 1000:
            # Keep most recent 500
            keep = list(self._processed_ids)[-500:]
            self._processed_ids = set(keep)


def _make_tool_request_event(
    request_id: str,
    command: str,
    payload: dict[str, Any],
) -> Any:
    """Create a ToolRequestEvent-like object for the transport channel."""
    from .transport import ToolRequestEvent
    return ToolRequestEvent(
        request_id=request_id,
        name=command,
        params=payload,
    )


def _coerce_to_structured(result: dict[str, Any]) -> dict[str, Any]:
    """Ensure result follows the structured format.

    Input may be in various formats from Flutter:
    - {"status": "ok", ...}
    - {"status": "error", "message": "...", "recoverable": true}
    - {"success": true, ...}

    Output is always structured.
    """
    if "success" in result:
        return result  # Already structured

    status = result.get("status", "ok")
    if status == "ok":
        return {
            "success": True,
            "status": "ok",
            "data": {k: v for k, v in result.items() if k != "status"},
            "message": result.get("message", ""),
        }
    if status == "blocked":
        return {
            "success": False,
            "status": "blocked",
            "error": {
                "code": ToolErrorCode.SAFETY_BLOCKED,
                "message": result.get("message", result.get("error", "blocked")),
                "recoverable": False,
            },
        }
    if status == "error":
        return {
            "success": False,
            "status": "error",
            "error": {
                "code": result.get("code", ToolErrorCode.UNKNOWN),
                "message": result.get("message", result.get("error", "unknown error")),
                "recoverable": result.get("recoverable", True),
            },
        }
    # Unknown status — treat as success with data
    return {
        "success": True,
        "status": status,
        "data": {k: v for k, v in result.items() if k != "status"},
        "message": result.get("message", ""),
    }
