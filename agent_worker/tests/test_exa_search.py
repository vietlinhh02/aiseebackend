import json

import httpx
import pytest

from gemini_bridge.exa_search import ExaSearchClient


@pytest.mark.anyio
async def test_exa_search_uses_metadata_only_payload_by_default():
    requests = []

    def handler(request):
        requests.append(request)
        return httpx.Response(
            200,
            json={
                "results": [
                    {
                        "title": "One",
                        "url": "https://example.test/one",
                        "publishedDate": "2026-04-27",
                    }
                ]
            },
        )

    client = ExaSearchClient(
        api_key="exa-key",
        default_max_results=3,
        transport=httpx.MockTransport(handler),
    )

    result = await client.search("tin mới")

    assert result == {
        "status": "ok",
        "query": "tin mới",
        "results": [
            {
                "title": "One",
                "url": "https://example.test/one",
                "published_date": "2026-04-27",
                "author": "",
                "text": "",
            }
        ],
    }
    payload = json.loads(requests[0].content)
    assert payload == {
        "query": "tin mới",
        "type": "auto",
        "numResults": 3,
    }


@pytest.mark.anyio
async def test_exa_search_caps_results_to_reduce_cost():
    requests = []

    def handler(request):
        requests.append(request)
        return httpx.Response(200, json={"results": []})

    client = ExaSearchClient(
        api_key="exa-key",
        default_max_results=3,
        transport=httpx.MockTransport(handler),
    )

    await client.search("tin mới", max_results=20)

    payload = json.loads(requests[0].content)
    assert payload["numResults"] == 5
