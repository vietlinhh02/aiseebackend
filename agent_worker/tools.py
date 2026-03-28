"""Tool stubs for Phase 0. Real implementations added in Phase 1-2."""

from livekit.agents import function_tool


@function_tool()
async def activate_camera(reason: str = "User requested") -> dict:
    """Enable camera to see through user's phone."""
    return {"status": "not_implemented_yet"}


@function_tool()
async def deactivate_camera() -> dict:
    """Disable camera, return to audio-only."""
    return {"status": "not_implemented_yet"}


ALL_TOOLS = [activate_camera, deactivate_camera]
