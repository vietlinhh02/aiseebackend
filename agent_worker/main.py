import asyncio
import logging

from backend import BackendClient
from config import (
    AI_TRANSPORT,
    ASSISTANT_WS_HOST,
    ASSISTANT_WS_PORT,
    GO_BACKEND_URL,
    INTERNAL_API_KEY,
)
from livekit.agents import WorkerOptions, cli
from prompts import select_prompt

from gemini_bridge import GeminiBridgeWebSocketService, GoogleSDKBridge
from gemini_bridge.core import GeminiLiveSettings
from gemini_bridge.exa_search import ExaSearchClient
from gemini_bridge.tool_router import ToolRouter
from gemini_bridge.transport import TransportDeviceCommandChannel

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("aisee-agent")


async def google_sdk_entrypoint(ctx):
    bridge = GoogleSDKBridge(ctx=ctx, logger=logger)
    await bridge.run()


entrypoint = google_sdk_entrypoint

def _build_user_context(session_context: dict) -> dict:
    profile = session_context.get("profile")
    if not isinstance(profile, dict):
        profile = {}
    return {
        "user_id": session_context.get("user_id", ""),
        "user_identity": session_context.get("user_id", ""),
        "user_name": profile.get("user_name") or profile.get("display_name") or "",
        "locale": session_context.get("locale", "vi"),
        "accessibility": profile.get("accessibility", "low_vision"),
        "has_family_link": str(profile.get("has_family_link", "false")).lower(),
    }


def _build_tool_router(transport, session_context: dict) -> ToolRouter:
    user_context = _build_user_context(session_context)
    router = ToolRouter(
        backend=BackendClient(GO_BACKEND_URL, INTERNAL_API_KEY),
        device_channel=TransportDeviceCommandChannel(transport=transport),
        search_client=ExaSearchClient.from_env(),
        user_context=user_context,
    )
    # Wire camera watchdog: when timeout fires, route deactivation through tool router
    router.runtime_state.set_camera_deactivate_callback(
        lambda: router._deactivate_camera({}),
    )
    return router


def _build_instructions(session_context: dict) -> str:
    user_context = _build_user_context(session_context)
    return select_prompt(
        user_context.get("locale", "vi"),
        bool(session_context.get("wake_greeting")),
        user_context,
    )


async def websocket_entrypoint() -> None:
    service = GeminiBridgeWebSocketService(
        settings=GeminiLiveSettings.from_env(),
        tool_router_factory=_build_tool_router,
        instructions_factory=_build_instructions,
        log=logger,
    )
    server = await service.serve(ASSISTANT_WS_HOST, ASSISTANT_WS_PORT)
    logger.info(
        "assistant websocket service listening on %s:%s",
        ASSISTANT_WS_HOST,
        ASSISTANT_WS_PORT,
    )
    await server.serve_forever()


if __name__ == "__main__":
    if AI_TRANSPORT == "livekit":
        cli.run_app(
            WorkerOptions(
                entrypoint_fnc=entrypoint,
                agent_name="aisee-agent",
            ),
        )
    else:
        asyncio.run(websocket_entrypoint())
