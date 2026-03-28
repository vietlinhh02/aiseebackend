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
        resp.raise_for_status()
        return resp.json()

    async def close(self):
        await self.client.aclose()
