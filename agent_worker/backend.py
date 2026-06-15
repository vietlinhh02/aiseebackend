"""HTTP client for Go backend internal API."""

from typing import Any
import logging

import httpx

from backend_models import (
    AICostRecordPayload,
    BackendJSONResponse,
    BackendPayload,
    QuotaCheckPayload,
    SessionMemorySavePayload,
    SessionMemoryTurnPayload,
    ToolCallPayload,
    TrackingEventPayload,
    UsageRecordPayload,
    UsageSessionCommitPayload,
    UsageSessionStartPayload,
)

logger = logging.getLogger("aisee-agent")


# ── Shared httpx client (singleton) ──
# Per-session httpx clients cause repeated TCP/TLS handshakes to Go backend.
# Sharing one client reuses the connection pool (max 100 keepalive conns).
# Created lazily on first use, closed on process shutdown.
_shared_async_client: httpx.AsyncClient | None = None


def _get_shared_client(base_url: str) -> httpx.AsyncClient:
    global _shared_async_client
    if _shared_async_client is None:
        limits = httpx.Limits(
            max_connections=200,
            max_keepalive_connections=100,
            keepalive_expiry=30.0,
        )
        _shared_async_client = httpx.AsyncClient(
            base_url=base_url,
            timeout=10.0,
            limits=limits,
        )
    return _shared_async_client


async def close_shared_client() -> None:
    global _shared_async_client
    if _shared_async_client is not None and not _shared_async_client.is_closed:
        await _shared_async_client.aclose()
    _shared_async_client = None


class BackendClient:
    """Calls Go backend via POST /api/internal/tool-call."""

    def __init__(
        self,
        base_url: str,
        api_key: str,
        client: httpx.AsyncClient | None = None,
    ):
        # Pass an explicit client to take ownership; otherwise use shared pool.
        if client is not None:
            self.client = client
            self._owns_client = True
        else:
            self.client = _get_shared_client(base_url)
            self._owns_client = False
        self.api_key = api_key

    async def _post_json(
        self,
        path: str,
        payload: BackendPayload | dict[str, Any],
        *,
        timeout: float | None = None,
        log_name: str,
    ) -> dict[str, Any]:
        json_payload = (
            payload.model_dump(exclude_none=True)
            if isinstance(payload, BackendPayload)
            else payload
        )
        resp = await self.client.post(
            path,
            json=json_payload,
            headers={"X-Internal-Key": self.api_key},
            timeout=timeout,
        )
        if resp.status_code >= 400:
            logger.error(
                "%s failed: %d %s",
                log_name,
                resp.status_code,
                resp.text,
            )
        resp.raise_for_status()
        return BackendJSONResponse.model_validate(resp.json()).root

    async def tool_call(
        self,
        tool: str,
        user_id: str,
        params: dict[str, Any],
    ) -> dict[str, Any]:
        """Send a tool call to Go backend and return the result."""
        return await self._post_json(
            "/tool-call",
            ToolCallPayload(tool=tool, user_id=user_id, params=params),
            log_name=f"tool_call {tool}",
        )

    async def tool_call_with_timeout(
        self,
        tool: str,
        user_id: str,
        params: dict[str, Any],
        timeout: float = 35.0,
    ) -> dict[str, Any]:
        """Send a tool call with custom timeout (for long-running tools)."""
        return await self._post_json(
            "/tool-call",
            ToolCallPayload(tool=tool, user_id=user_id, params=params),
            timeout=timeout,
            log_name=f"tool_call {tool}",
        )

    async def check_quota(
        self,
        user_id: str,
        usage_type: str,
        quantity: float,
    ) -> dict[str, Any]:
        """Check quota before a billable action."""
        return await self._post_json(
            "/usage/check",
            QuotaCheckPayload(
                user_id=user_id,
                usage_type=usage_type,
                quantity=quantity,
            ),
            log_name=f"check_quota {usage_type}",
        )

    async def record_usage(
        self,
        user_id: str,
        usage_type: str,
        quantity: float,
        unit: str,
        source: str,
        source_id: str,
        metadata: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Record trusted billable usage."""
        return await self._post_json(
            "/usage/record",
            UsageRecordPayload(
                user_id=user_id,
                usage_type=usage_type,
                quantity=quantity,
                unit=unit,
                source=source,
                source_id=source_id,
                metadata=metadata or {},
            ),
            log_name=f"record_usage {usage_type}",
        )

    async def start_usage_session(
        self,
        user_id: str,
        usage_type: str,
        unit: str,
        source: str,
        source_id: str,
        started_at: str,
        metadata: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Start a trusted usage session."""
        return await self._post_json(
            "/usage/session/start",
            UsageSessionStartPayload(
                user_id=user_id,
                usage_type=usage_type,
                unit=unit,
                source=source,
                source_id=source_id,
                started_at=started_at,
                metadata=metadata or {},
            ),
            log_name=f"start_usage_session {usage_type}",
        )

    async def commit_usage_session(
        self,
        user_id: str,
        usage_type: str,
        unit: str,
        source: str,
        source_id: str,
        started_at: str,
        ended_at: str,
        metadata: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        """Commit a trusted usage session."""
        return await self._post_json(
            "/usage/session/commit",
            UsageSessionCommitPayload(
                user_id=user_id,
                usage_type=usage_type,
                unit=unit,
                source=source,
                source_id=source_id,
                started_at=started_at,
                ended_at=ended_at,
                metadata=metadata or {},
            ),
            log_name=f"commit_usage_session {usage_type}",
        )

    async def record_ai_cost(
        self,
        user_id: str,
        provider: str,
        model: str,
        modality: str,
        source: str,
        source_id: str,
        cost_usd: float,
        cost_vnd: float,
        estimated: bool,
        pricing_version: str,
        occurred_at: str,
        metadata: dict[str, Any] | None = None,
        input_tokens: int = 0,
        output_tokens: int = 0,
        audio_input_seconds: float = 0,
        audio_output_seconds: float = 0,
        image_count: int = 0,
        video_seconds: float = 0,
    ) -> dict[str, Any]:
        """Record internal AI cost telemetry for admin analytics."""
        return await self._post_json(
            "/ai-cost/record",
            AICostRecordPayload(
                user_id=user_id,
                provider=provider,
                model=model,
                modality=modality,
                source=source,
                source_id=source_id,
                input_tokens=input_tokens,
                output_tokens=output_tokens,
                audio_input_seconds=audio_input_seconds,
                audio_output_seconds=audio_output_seconds,
                image_count=image_count,
                video_seconds=video_seconds,
                cost_usd=cost_usd,
                cost_vnd=cost_vnd,
                currency="USD",
                estimated=estimated,
                pricing_version=pricing_version,
                occurred_at=occurred_at,
                metadata=metadata or {},
            ),
            log_name=f"record_ai_cost {model}",
        )

    async def record_tracking_event(
        self,
        event_type: str,
        category: str,
        user_id: str,
        metadata: dict[str, Any] | None = None,
        *,
        severity: str = "",
        source: str = "agent_worker",
        source_id: str = "unknown",
    ) -> dict[str, Any]:
        """Record a tracking event for admin observability."""
        return await self._post_json(
            "/tracking/events",
            TrackingEventPayload(
                event_type=event_type,
                category=category,
                user_id=user_id,
                severity=severity,
                source=source,
                source_id=source_id,
                metadata=metadata or {},
            ),
            log_name=f"record_tracking_event {event_type}",
        )

    async def close(self):
        if self._owns_client:
            await self.client.aclose()

    # ── Session memory persistence ──

    async def save_session_memory(
        self,
        user_id: str,
        session_id: str,
        summary: str = "",
        recent_turns: list[dict[str, str]] | None = None,
        user_facts: list[str] | None = None,
        last_intent: str = "",
        pending_tasks: list[str] | None = None,
        last_mode: str = "normal",
    ) -> dict[str, Any]:
        """Persist session memory to Go backend."""
        turns = [
            SessionMemoryTurnPayload(role=t.get("role", ""), text=t.get("text", ""))
            for t in (recent_turns or [])
        ]
        return await self._post_json(
            "/session-memory/save",
            SessionMemorySavePayload(
                user_id=user_id,
                session_id=session_id,
                summary=summary,
                recent_turns=turns,
                user_facts=user_facts or [],
                last_intent=last_intent,
                pending_tasks=pending_tasks or [],
                last_mode=last_mode,
            ),
            log_name="save_session_memory",
        )

    async def load_session_memory(
        self,
        user_id: str,
        session_id: str = "",
    ) -> dict[str, Any]:
        """Load most recent session memory from Go backend."""
        params: dict[str, str] = {"user_id": user_id}
        if session_id:
            params["session_id"] = session_id
        resp = await self.client.get(
            "/session-memory/load",
            params=params,
            headers={"X-Internal-Key": self.api_key},
            timeout=10.0,
        )
        resp.raise_for_status()
        return BackendJSONResponse.model_validate(resp.json()).root
