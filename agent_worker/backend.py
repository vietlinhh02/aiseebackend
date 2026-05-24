"""HTTP client for Go backend internal API."""

from typing import Any
import logging

import httpx

from backend_models import (
    AICostRecordPayload,
    BackendJSONResponse,
    BackendPayload,
    QuotaCheckPayload,
    ToolCallPayload,
    TrackingEventPayload,
    UsageRecordPayload,
    UsageSessionCommitPayload,
    UsageSessionStartPayload,
)

logger = logging.getLogger("aisee-agent")


class BackendClient:
    """Calls Go backend via POST /api/internal/tool-call."""

    def __init__(
        self,
        base_url: str,
        api_key: str,
        client: httpx.AsyncClient | None = None,
    ):
        self.client = client or httpx.AsyncClient(
            base_url=base_url,
            timeout=10.0,
        )
        self.api_key = api_key
        self._owns_client = client is None

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
