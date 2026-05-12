"""HTTP client for Go backend internal API."""

import logging

import httpx

logger = logging.getLogger("aisee-agent")


class BackendClient:
    """Calls Go backend via POST /api/internal/tool-call."""

    def __init__(self, base_url: str, api_key: str):
        self.client = httpx.AsyncClient(
            base_url=base_url,
            timeout=10.0,
        )
        self.api_key = api_key

    async def tool_call(self, tool: str, user_id: str, params: dict) -> dict:
        """Send a tool call to Go backend and return the result."""
        resp = await self.client.post(
            "/tool-call",
            json={"tool": tool, "user_id": user_id, "params": params},
            headers={"X-Internal-Key": self.api_key},
        )
        if resp.status_code >= 400:
            logger.error(
                "tool_call %s failed: %d %s",
                tool, resp.status_code, resp.text,
            )
        resp.raise_for_status()
        return resp.json()

    async def tool_call_with_timeout(
        self,
        tool: str,
        user_id: str,
        params: dict,
        timeout: float = 35.0,
    ) -> dict:
        """Send a tool call with custom timeout (for long-running tools)."""
        resp = await self.client.post(
            "/tool-call",
            json={"tool": tool, "user_id": user_id, "params": params},
            headers={"X-Internal-Key": self.api_key},
            timeout=timeout,
        )
        if resp.status_code >= 400:
            logger.error(
                "tool_call %s failed: %d %s",
                tool, resp.status_code, resp.text,
            )
        resp.raise_for_status()
        return resp.json()

    async def check_quota(
        self,
        user_id: str,
        usage_type: str,
        quantity: float,
    ) -> dict:
        """Check quota before a billable action."""
        resp = await self.client.post(
            "/usage/check",
            json={
                "user_id": user_id,
                "usage_type": usage_type,
                "quantity": quantity,
            },
            headers={"X-Internal-Key": self.api_key},
        )
        if resp.status_code >= 400:
            logger.error(
                "check_quota %s failed: %d %s",
                usage_type, resp.status_code, resp.text,
            )
        resp.raise_for_status()
        return resp.json()

    async def record_usage(
        self,
        user_id: str,
        usage_type: str,
        quantity: float,
        unit: str,
        source: str,
        source_id: str,
        metadata: dict | None = None,
    ) -> dict:
        """Record trusted billable usage."""
        resp = await self.client.post(
            "/usage/record",
            json={
                "user_id": user_id,
                "usage_type": usage_type,
                "quantity": quantity,
                "unit": unit,
                "source": source,
                "source_id": source_id,
                "metadata": metadata or {},
            },
            headers={"X-Internal-Key": self.api_key},
        )
        if resp.status_code >= 400:
            logger.error(
                "record_usage %s failed: %d %s",
                usage_type, resp.status_code, resp.text,
            )
        resp.raise_for_status()
        return resp.json()

    async def start_usage_session(
        self,
        user_id: str,
        usage_type: str,
        unit: str,
        source: str,
        source_id: str,
        started_at: str,
        metadata: dict | None = None,
    ) -> dict:
        """Start a trusted usage session."""
        resp = await self.client.post(
            "/usage/session/start",
            json={
                "user_id": user_id,
                "usage_type": usage_type,
                "unit": unit,
                "source": source,
                "source_id": source_id,
                "started_at": started_at,
                "metadata": metadata or {},
            },
            headers={"X-Internal-Key": self.api_key},
        )
        if resp.status_code >= 400:
            logger.error(
                "start_usage_session %s failed: %d %s",
                usage_type, resp.status_code, resp.text,
            )
        resp.raise_for_status()
        return resp.json()

    async def commit_usage_session(
        self,
        user_id: str,
        usage_type: str,
        unit: str,
        source: str,
        source_id: str,
        started_at: str,
        ended_at: str,
        metadata: dict | None = None,
    ) -> dict:
        """Commit a trusted usage session."""
        resp = await self.client.post(
            "/usage/session/commit",
            json={
                "user_id": user_id,
                "usage_type": usage_type,
                "unit": unit,
                "source": source,
                "source_id": source_id,
                "started_at": started_at,
                "ended_at": ended_at,
                "metadata": metadata or {},
            },
            headers={"X-Internal-Key": self.api_key},
        )
        if resp.status_code >= 400:
            logger.error(
                "commit_usage_session %s failed: %d %s",
                usage_type, resp.status_code, resp.text,
            )
        resp.raise_for_status()
        return resp.json()

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
        metadata: dict | None = None,
        input_tokens: int = 0,
        output_tokens: int = 0,
        audio_input_seconds: float = 0,
        audio_output_seconds: float = 0,
        image_count: int = 0,
        video_seconds: float = 0,
    ) -> dict:
        """Record internal AI cost telemetry for admin analytics."""
        resp = await self.client.post(
            "/ai-cost/record",
            json={
                "user_id": user_id,
                "provider": provider,
                "model": model,
                "modality": modality,
                "source": source,
                "source_id": source_id,
                "input_tokens": input_tokens,
                "output_tokens": output_tokens,
                "audio_input_seconds": audio_input_seconds,
                "audio_output_seconds": audio_output_seconds,
                "image_count": image_count,
                "video_seconds": video_seconds,
                "cost_usd": cost_usd,
                "cost_vnd": cost_vnd,
                "currency": "USD",
                "estimated": estimated,
                "pricing_version": pricing_version,
                "occurred_at": occurred_at,
                "metadata": metadata or {},
            },
            headers={"X-Internal-Key": self.api_key},
        )
        if resp.status_code >= 400:
            logger.error(
                "record_ai_cost %s failed: %d %s",
                model, resp.status_code, resp.text,
            )
        resp.raise_for_status()
        return resp.json()

    async def close(self):
        await self.client.aclose()
