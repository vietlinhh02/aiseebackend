"""Exa-backed web search for Gemini function calling."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import httpx


@dataclass(frozen=True, slots=True)
class ExaSearchClient:
    api_key: str
    search_url: str = "https://api.exa.ai/search"
    default_max_results: int = 3
    timeout: float = 12.0
    transport: httpx.AsyncBaseTransport | None = None

    @classmethod
    def from_env(cls) -> "ExaSearchClient":
        from config import EXA_API_KEY, EXA_SEARCH_MAX_RESULTS, EXA_SEARCH_URL

        return cls(
            api_key=EXA_API_KEY,
            search_url=EXA_SEARCH_URL,
            default_max_results=EXA_SEARCH_MAX_RESULTS,
        )

    async def search(self, query: str, *, max_results: int | None = None) -> dict[str, Any]:
        query = query.strip()
        if not query:
            return {"status": "error", "error": "web_search requires a non-empty query"}
        if not self.api_key:
            return {"status": "error", "error": "EXA_API_KEY is not configured"}

        limit = self._normalize_limit(max_results)
        payload = {
            "query": query,
            "type": "auto",
            "numResults": limit,
        }
        async with httpx.AsyncClient(timeout=self.timeout, transport=self.transport) as client:
            response = await client.post(
                self.search_url,
                headers={"x-api-key": self.api_key, "Content-Type": "application/json"},
                json=payload,
            )
            response.raise_for_status()
        data = response.json()
        results = data.get("results") if isinstance(data, dict) else []
        return {
            "status": "ok",
            "query": query,
            "results": [self._coerce_result(item) for item in (results or [])[:limit]],
        }

    def _normalize_limit(self, max_results: int | None) -> int:
        try:
            value = int(max_results or self.default_max_results)
        except (TypeError, ValueError):
            value = self.default_max_results
        return min(max(value, 1), 5)

    def _coerce_result(self, item: object) -> dict[str, str]:
        if not isinstance(item, dict):
            return {"title": "", "url": "", "text": ""}
        return {
            "title": str(item.get("title") or ""),
            "url": str(item.get("url") or ""),
            "published_date": str(item.get("publishedDate") or ""),
            "author": str(item.get("author") or ""),
            "text": self._trim(str(item.get("text") or item.get("summary") or "")),
        }

    @staticmethod
    def _trim(value: str, limit: int = 1200) -> str:
        value = " ".join(value.split())
        if len(value) <= limit:
            return value
        return value[: limit - 1].rstrip() + "..."
